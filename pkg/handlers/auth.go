package handlers

import (
	"fmt"
	"log"
	"net/http"

	"github.com/ashupednekar/litewebservices-portal/internal/auth"
	"github.com/ashupednekar/litewebservices-portal/internal/auth/adaptors"
	"github.com/ashupednekar/litewebservices-portal/pkg"
	"github.com/ashupednekar/litewebservices-portal/pkg/state"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type AuthHandlers struct {
	State *state.AppState
	store auth.PasskeyStore
}

func NewAuthHandlers(state *state.AppState) *AuthHandlers{
	store := adaptors.NewWebauthnStore(state.DBPool)
	return &AuthHandlers{State: state, store: store}
}

func (h *AuthHandlers) BeginRegistration(ctx *gin.Context) {
	log.Printf("[INFO] begin registration ----------------------\\")

	username, err := auth.GetUsername(ctx)
	if err != nil {
		msg := fmt.Sprintf("[ERRO] can't get user name: %s", err.Error())
		ctx.JSON(http.StatusInternalServerError, gin.H{"msg": msg})
		return
	}
	user, err := h.store.GetOrCreateUser(username)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"msg": fmt.Sprintf("error creating/retirieving user: %s", err)})
		return
	}
	options, session, err := h.State.Authn.BeginRegistration(user)
	if err != nil {
		msg := fmt.Sprintf("can't begin registration: %s", err.Error())
		log.Printf("[ERRO] %s", msg)
		ctx.JSON(http.StatusBadRequest, gin.H{"msg": msg})
		return
	}

	t := uuid.New().String()
	h.store.SaveSession(t, *session)
	ctx.Header("Session-Key", t)
	ctx.JSON(http.StatusOK, options)
}

func (h *AuthHandlers) FinishRegistration(ctx *gin.Context) {
	t := ctx.Request.Header.Get("Session-Key")
	session, _ := h.store.GetSession(t)
	user, err := h.store.GetOrCreateUser(string(session.UserID))
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"msg": fmt.Sprintf("error creating/retirieving user: %s", err)})
		return
	}
	credential, err := h.State.Authn.FinishRegistration(user, session, ctx.Request)
	if err != nil {
		msg := fmt.Sprintf("can't finish registration: %s", err.Error())
		log.Printf("[ERRO] %s", msg)
		ctx.JSON(http.StatusBadRequest, gin.H{"msg": msg})
		return
	}

	user.AddCredential(credential)
	err = h.store.SaveUser(user)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"msg": fmt.Sprintf("error saving user: %s", err)})
		return
	}
	err = h.store.DeleteSession(t)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"msg": fmt.Sprintf("error clearing session: %s", err)})
		return
	}
	log.Printf("[INFO] finish registration ----------------------/")
	ctx.JSON(http.StatusOK, gin.H{"msg": "Registration Success"})
}

func (h *AuthHandlers) BeginLogin(ctx *gin.Context) {
	log.Printf("[INFO] begin login ----------------------\\")

	username, err := auth.GetUsername(ctx)
	if err != nil {
		log.Printf("[ERRO]can't get user name: %s", err.Error())
		panic(err)
	}

	user, err := h.store.GetOrCreateUser(username)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"msg": fmt.Sprintf("error creating/retirieving user: %s", err)})
		return
	}
	options, session, err := h.State.Authn.BeginLogin(user)
	if err != nil {
		msg := fmt.Sprintf("can't begin login: %s", err.Error())
		log.Printf("[ERRO] %s", msg)
		ctx.JSON(http.StatusBadRequest, gin.H{"msg": msg})
		return
	}

	t := uuid.New().String()
	h.store.SaveSession(t, *session)

	ctx.Header("Session-Key", t)
	ctx.JSON(http.StatusOK, options)
}

func (h *AuthHandlers) FinishLogin(ctx *gin.Context) {
	t := ctx.Request.Header.Get("Session-Key")

	session, _ := h.store.GetSession(t)
	user, err := h.store.GetOrCreateUser(string(session.UserID))
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"msg": fmt.Sprintf("error creating/retirieving user: %s", err)})
		return
	}

	credential, err := h.State.Authn.FinishLogin(user, session, ctx.Request)
	if err != nil {
		log.Printf("[ERRO] can't finish login %s", err.Error())
		panic(err)
	}

	user.UpdateCredential(credential)
	err = h.store.SaveUser(user)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"msg": fmt.Sprintf("error saving user: %s", err)})
		return
	}
	err = h.store.DeleteSession(t)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"msg": fmt.Sprintf("error clearing session: %s", err)})
		return
	}

	log.Printf("[INFO] finish login ----------------------/")
	ctx.JSON(http.StatusOK, gin.H{"msg": "Login Success"})
}

func (h *AuthHandlers) SetSchema(ctx *gin.Context) *pgx.Tx {
	tx, err := h.State.DBPool.Begin(ctx)
	if err != nil {
		msg := fmt.Sprintf("[ERRO] couln't obtain transaction: %s", err.Error())
		ctx.JSON(http.StatusInternalServerError, gin.H{"msg": msg})
		return nil
	}
	_, err = tx.Exec(ctx,
		"SET LOCAL search_path TO "+pgx.Identifier{pkg.Cfg.DatabaseSchema}.Sanitize(),
	)
	defer tx.Commit(ctx)
	return &tx
}
