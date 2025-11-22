package handlers

import (
	"fmt"
	"log"
	"net/http"

	"github.com/ashupednekar/litewebservices-portal/internal/auth"
	"github.com/ashupednekar/litewebservices-portal/internal/auth/adaptors"
	"github.com/ashupednekar/litewebservices-portal/pkg/state"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type AuthHandlers struct {
	state     *state.AppState
	datastore auth.PasskeyStore
}

func NewAuthHandlers(state *state.AppState) *AuthHandlers{
	return &AuthHandlers{state: state, datastore: adaptors.NewInMemoryStore()}
}

func (h *AuthHandlers) BeginRegistration(ctx *gin.Context) {
	log.Printf("[INFO] begin registration ---------------------- with %v\\")

	username, err := auth.GetUsername(ctx)
	if err != nil {
		log.Printf("[ERRO] can't get user name: %s", err.Error())
		panic(err)
	}

	user := h.datastore.GetOrCreateUser(username) 
	options, session, err := h.state.Authn.BeginRegistration(user)
	if err != nil {
		msg := fmt.Sprintf("can't begin registration: %s", err.Error())
		log.Printf("[ERRO] %s", msg)
		ctx.JSON(http.StatusBadRequest, gin.H{"msg": msg})
		return
	}

	t := uuid.New().String()
	h.datastore.SaveSession(t, *session)
	ctx.Header("Session-Key", t)
	ctx.JSON(http.StatusOK, options)
}

func (h *AuthHandlers) FinishRegistration(ctx *gin.Context) {
	t := ctx.Request.Header.Get("Session-Key")
		session, _ := h.datastore.GetSession(t) 
		user := h.datastore.GetOrCreateUser(string(session.UserID)) 
	credential, err := h.state.Authn.FinishRegistration(user, session, ctx.Request)
	if err != nil {
		msg := fmt.Sprintf("can't finish registration: %s", err.Error())
		log.Printf("[ERRO] %s", msg)
		ctx.JSON(http.StatusBadRequest, gin.H{"msg": msg})
		return
	}

	user.AddCredential(credential)
	h.datastore.SaveUser(user)
		h.datastore.DeleteSession(t)

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

	user := h.datastore.GetOrCreateUser(username) 
	options, session, err := h.state.Authn.BeginLogin(user)
	if err != nil {
		msg := fmt.Sprintf("can't begin login: %s", err.Error())
		log.Printf("[ERRO] %s", msg)
		ctx.JSON(http.StatusBadRequest, gin.H{"msg": msg})
		return
	}

	t := uuid.New().String()
	h.datastore.SaveSession(t, *session)
	
	ctx.Header("Session-Key", t)
	ctx.JSON(http.StatusOK, options)
}

func (h *AuthHandlers) FinishLogin(ctx *gin.Context) {
		t := ctx.Request.Header.Get("Session-Key")
		session, _ := h.datastore.GetSession(t) 
		user := h.datastore.GetOrCreateUser(string(session.UserID)) 
	credential, err := h.state.Authn.FinishLogin(user, session, ctx.Request)
	if err != nil {
		log.Printf("[ERRO] can't finish login %s", err.Error())
		panic(err)
	}

				
		user.UpdateCredential(credential)
	h.datastore.SaveUser(user)
		h.datastore.DeleteSession(t)

	log.Printf("[INFO] finish login ----------------------/")
	ctx.JSON(http.StatusOK, gin.H{"msg": "Login Success"})
}
