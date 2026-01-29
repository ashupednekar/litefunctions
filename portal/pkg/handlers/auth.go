package handlers

import (
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"time"

	"github.com/ashupednekar/litefunctions/portal/internal/auth"
	"github.com/ashupednekar/litefunctions/portal/internal/auth/adaptors"
	"github.com/ashupednekar/litefunctions/portal/pkg"
	"github.com/ashupednekar/litefunctions/portal/pkg/state"
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
	user, err := h.store.GetOrCreateUser(username)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"msg": fmt.Sprintf("error creating/retrieving user: %s", err)})
		return
	}
	options, session, err := h.State.Authn.BeginRegistration(user)
	expDur, parseErr := time.ParseDuration(pkg.Cfg.SessionExpiry)
	if parseErr != nil {
		slog.Error("invalid session expiry configured", "error", parseErr)
		ctx.JSON(http.StatusInternalServerError, gin.H{"msg": "internal configuration error"})
		return
	}
	session.Expires = time.Now().Add(expDur)
	if err != nil {
		slog.Error("can't begin registration", "error", err)
		ctx.JSON(http.StatusBadRequest, gin.H{"msg": err.Error()})
		return
	}

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

	// Create a persistent user session
	sessionID, err := auth.GenerateSessionID()
	if err != nil {
		slog.Error("failed to generate session ID", "error", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"msg": "failed to create session"})
		return
	}

	expiresAt, err := auth.GetSessionExpiry()
	if err != nil {
		slog.Error("failed to get session expiry", "error", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"msg": "failed to create session"})
		return
	}

	userAgent := ctx.Request.UserAgent()
	ipAddress := ctx.ClientIP()

	if err := h.store.CreateUserSession(user.WebAuthnID(), sessionID, expiresAt, userAgent, ipAddress); err != nil {
		slog.Error("failed to create user session", "error", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"msg": "failed to create session"})
		return
	}

	// Set secure cookie
	maxAge := int(time.Until(expiresAt).Seconds())
	ctx.SetCookie(
		auth.SessionCookieName, // name
		sessionID,              // value
		maxAge,                 // maxAge in seconds
		"/",                    // path
		"",                     // domain (empty = current domain)
		false,                  // secure (set to true in production with HTTPS)
		true,                   // httpOnly
	)

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

	// Create a persistent user session
	sessionID, err := auth.GenerateSessionID()
	if err != nil {
		slog.Error("failed to generate session ID", "error", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"msg": "failed to create session"})
		return
	}

	expiresAt, err := auth.GetSessionExpiry()
	if err != nil {
		slog.Error("failed to get session expiry", "error", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"msg": "failed to create session"})
		return
	}

	userAgent := ctx.Request.UserAgent()
	ipAddress := ctx.ClientIP()

	if err := h.store.CreateUserSession(user.WebAuthnID(), sessionID, expiresAt, userAgent, ipAddress); err != nil {
		slog.Error("failed to create user session", "error", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"msg": "failed to create session"})
		return
	}

	// Set secure cookie
	maxAge := int(time.Until(expiresAt).Seconds())
	ctx.SetCookie(
		auth.SessionCookieName, // name
		sessionID,              // value
		maxAge,                 // maxAge in seconds
		"/",                    // path
		"",                     // domain (empty = current domain)
		false,                  // secure (set to true in production with HTTPS)
		true,                   // httpOnly
	)

	slog.Info("Finish login successful", "user", user.WebAuthnName())
	ctx.JSON(http.StatusOK, gin.H{"msg": "Login Success"})
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
	ctx.SetCookie(
		auth.SessionCookieName, // name
		"",                     // value
		-1,                     // maxAge (negative = delete)
		"/",                    // path
		"",                     // domain
		false,                  // secure
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
		"SET LOCAL search_path TO "+pgx.Identifier{pkg.Cfg.DatabaseSchema}.Sanitize(),
	)
	defer tx.Commit(ctx)
	return &tx
}
