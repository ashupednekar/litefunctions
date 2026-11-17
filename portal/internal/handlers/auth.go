package handlers

import (
	"crypto/subtle"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/ashupednekar/litefunctions/portal/internal/auth"
	"github.com/ashupednekar/litefunctions/portal/internal/auth/adaptors"
	"github.com/ashupednekar/litefunctions/portal/internal/config"
	"github.com/ashupednekar/litefunctions/portal/internal/state"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type AuthHandlers struct {
	State *state.AppState
	store auth.PasskeyStore
}

func NewAuthHandlers(state *state.AppState) *AuthHandlers {
	store := adaptors.NewWebauthnStore(state.DBPool)
	return &AuthHandlers{State: state, store: store}
}

// GetStore returns the PasskeyStore for use in middleware
func (h *AuthHandlers) GetStore() auth.PasskeyStore {
	return h.store
}

func (h *AuthHandlers) BeginRegistration(ctx *gin.Context) {
	slog.Info("Begin registration")

	username, err := auth.GetUsername(ctx)
	if err != nil {
		slog.Error("can't get user name", "error", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"msg": "internal error"})
		return
	}
	if auth.IsSystemUser(username) {
		ctx.JSON(http.StatusForbidden, gin.H{"msg": "the system admin account does not use passkeys"})
		return
	}
	user, err := h.store.GetOrCreateUser(username)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"msg": fmt.Sprintf("error creating/retrieving user: %s", err)})
		return
	}
	options, session, err := h.State.Authn.BeginRegistration(user)
	if err != nil {
		slog.Error("can't begin registration", "error", err)
		ctx.JSON(http.StatusBadRequest, gin.H{"msg": err.Error()})
		return
	}
	expDur, parseErr := time.ParseDuration(config.Cfg.SessionExpiry)
	if parseErr != nil {
		slog.Error("invalid session expiry configured", "error", parseErr)
		ctx.JSON(http.StatusInternalServerError, gin.H{"msg": "internal configuration error"})
		return
	}
	session.Expires = time.Now().Add(expDur)

	t := uuid.New().String()
	slog.Debug("saving registration session", "username", username)
	err = h.store.SaveSession(username, t, *session)
	if err != nil {
		slog.Error("error saving registration session", "error", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"msg": "failed to save session"})
		return
	}
	ctx.Header("Session-Key", t)
	ctx.JSON(http.StatusOK, options)
}

func (h *AuthHandlers) FinishRegistration(ctx *gin.Context) {
	t := ctx.Request.Header.Get("Session-Key")
	session, ok := h.store.GetSession(t)
	if !ok {
		ctx.JSON(http.StatusUnauthorized, gin.H{"msg": "invalid or expired session"})
		return
	}
	slog.Debug("finishing registration for session")
	if auth.IsSystemUser(string(session.UserID)) {
		ctx.JSON(http.StatusForbidden, gin.H{"msg": "the system admin account does not use passkeys"})
		return
	}

	user, err := h.store.GetOrCreateUser(string(session.UserID))
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"msg": fmt.Sprintf("error creating/retrieving user: %s", err),
		})
		return
	}

	credential, err := h.State.Authn.FinishRegistration(user, session, ctx.Request)
	if err != nil {
		slog.Error("can't finish registration", "error", err)
		ctx.JSON(http.StatusBadRequest, gin.H{"msg": err.Error()})
		return
	}

	slog.Debug("registration successful", "user", user.WebAuthnName())

	if err := h.store.SaveCredential(user, credential); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"msg": fmt.Sprintf("error saving credential: %s", err),
		})
		return
	}
	err = h.store.DeleteSession(t)
	if err != nil {
		slog.Warn("error clearing webauthn session", "error", err)
	}

	if err := h.createUserSession(ctx, user.WebAuthnID()); err != nil {
		slog.Error("failed to create user session", "error", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"msg": "failed to create session"})
		return
	}

	slog.Info("Finish registration successful", "user", user.WebAuthnName())
	ctx.JSON(http.StatusOK, gin.H{"msg": "Registration Success"})
}

func (h *AuthHandlers) BeginLogin(ctx *gin.Context) {
	slog.Info("Begin login")

	username, err := auth.GetUsername(ctx)
	if err != nil {
		slog.Error("can't get user name for login", "error", err)
		ctx.JSON(http.StatusBadRequest, gin.H{"msg": "invalid request"})
		return
	}
	if auth.IsSystemUser(username) {
		ctx.JSON(http.StatusForbidden, gin.H{"msg": "the system admin account uses password login"})
		return
	}

	user, err := h.store.GetOrCreateUser(username)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"msg": fmt.Sprintf("error creating/retrieving user: %s", err)})
		return
	}
	options, session, err := h.State.Authn.BeginLogin(user)
	if err != nil {
		slog.Error("can't begin login", "error", err)
		ctx.JSON(http.StatusBadRequest, gin.H{"msg": err.Error()})
		return
	}

	t := uuid.New().String()
	if err := h.store.SaveSession(username, t, *session); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"msg": err})
		return
	}

	ctx.Header("Session-Key", t)
	ctx.JSON(http.StatusOK, options)
}

func (h *AuthHandlers) FinishLogin(ctx *gin.Context) {
	t := ctx.Request.Header.Get("Session-Key")

	session, ok := h.store.GetSession(t)
	if !ok {
		ctx.JSON(http.StatusUnauthorized, gin.H{"msg": "invalid or expired session"})
		return
	}
	if auth.IsSystemUser(string(session.UserID)) {
		ctx.JSON(http.StatusForbidden, gin.H{"msg": "the system admin account uses password login"})
		return
	}

	user, err := h.store.GetOrCreateUser(string(session.UserID))
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"msg": fmt.Sprintf("error creating/retrieving user: %s", err),
		})
		return
	}

	credential, err := h.State.Authn.FinishLogin(user, session, ctx.Request)
	if err != nil {
		slog.Error("can't finish login", "error", err)
		ctx.JSON(http.StatusUnauthorized, gin.H{"msg": "authentication failed"})
		return
	}

	if err := h.store.UpdateCredential(user, credential); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"msg": fmt.Sprintf("error updating credential: %s", err),
		})
		return
	}

	// Delete the webauthn challenge session
	err = h.store.DeleteSession(t)
	if err != nil {
		log.Printf("[WARN] error clearing webauthn session: %s", err)
	}

	if err := h.createUserSession(ctx, user.WebAuthnID()); err != nil {
		slog.Error("failed to create user session", "error", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"msg": "failed to create session"})
		return
	}

	slog.Info("Finish login successful", "user", user.WebAuthnName())
	ctx.JSON(http.StatusOK, gin.H{"msg": "Login Success"})
}

// AdminLogin authenticates only the internal system user. The password is
// configuration, never persisted in the database, and the regular passkey
// flows explicitly reject this username.
func (h *AuthHandlers) AdminLogin(ctx *gin.Context) {
	if !config.Cfg.AdminLoginEnabled || config.Cfg.AdminPassword == "" {
		ctx.JSON(http.StatusNotFound, gin.H{"msg": "admin login is disabled"})
		return
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"msg": "invalid request"})
		return
	}

	userOK := auth.IsSystemUser(req.Username)
	passOK := subtle.ConstantTimeCompare([]byte(req.Password), []byte(config.Cfg.AdminPassword)) == 1
	if !userOK || !passOK {
		slog.Warn("admin login failed", "user", req.Username, "ip", ctx.ClientIP())
		ctx.JSON(http.StatusUnauthorized, gin.H{"msg": "invalid credentials"})
		return
	}

	user, err := h.store.GetOrCreateUser(auth.SystemUserName)
	if err != nil {
		slog.Error("failed to ensure system user", "error", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"msg": "failed to create session"})
		return
	}
	if err := h.createUserSession(ctx, user.WebAuthnID()); err != nil {
		slog.Error("failed to create admin session", "error", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"msg": "failed to create session"})
		return
	}

	slog.Info("admin login successful", "ip", ctx.ClientIP())
	ctx.JSON(http.StatusOK, gin.H{"msg": "Login Success"})
}

func (h *AuthHandlers) createUserSession(ctx *gin.Context, userID []byte) error {
	sessionID, err := auth.GenerateSessionID()
	if err != nil {
		return fmt.Errorf("generate session ID: %w", err)
	}

	expiresAt, err := auth.GetSessionExpiry()
	if err != nil {
		return fmt.Errorf("get session expiry: %w", err)
	}

	if err := h.store.CreateUserSession(userID, sessionID, expiresAt, ctx.Request.UserAgent(), ctx.ClientIP()); err != nil {
		return fmt.Errorf("create user session: %w", err)
	}

	maxAge := int(time.Until(expiresAt).Seconds())
	ctx.SetSameSite(http.SameSiteLaxMode)
	ctx.SetCookie(
		auth.SessionCookieName,
		sessionID,
		maxAge,
		"/",
		"",
		requestIsHTTPS(ctx),
		true,
	)
	return nil
}

func requestIsHTTPS(ctx *gin.Context) bool {
	if config.Cfg.CookieSecure || ctx.Request.TLS != nil {
		return true
	}
	// X-Forwarded-Proto is only meaningful when an operator has explicitly
	// configured trusted proxy CIDRs.
	if config.Cfg.TrustedProxies == "" {
		return false
	}
	return strings.EqualFold(ctx.GetHeader("X-Forwarded-Proto"), "https")
}

func (h *AuthHandlers) Logout(ctx *gin.Context) {
	// Get session cookie
	sessionID, err := ctx.Cookie(auth.SessionCookieName)
	if err == nil {
		// Delete session from database
		if err := h.store.DeleteUserSession(sessionID); err != nil {
			slog.Warn("failed to delete session on logout", "error", err)
		}
	}

	// Clear cookie
	ctx.SetSameSite(http.SameSiteLaxMode)
	ctx.SetCookie(
		auth.SessionCookieName, // name
		"",                     // value
		-1,                     // maxAge (negative = delete)
		"/",                    // path
		"",                     // domain
		requestIsHTTPS(ctx),    // secure
		true,                   // httpOnly
	)

	// Redirect to home
	ctx.Redirect(http.StatusFound, "/")
}

func (h *AuthHandlers) SetSchema(ctx *gin.Context) *pgx.Tx {
	tx, err := h.State.DBPool.Begin(ctx)
	if err != nil {
		slog.Error("couldn't obtain transaction", "error", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"msg": "internal error"})
		return nil
	}
	_, err = tx.Exec(ctx,
		"SET LOCAL search_path TO "+pgx.Identifier{config.Cfg.DatabaseSchema}.Sanitize(),
	)
	defer tx.Commit(ctx)
	return &tx
}
