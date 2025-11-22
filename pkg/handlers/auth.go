package handlers

import (
	"fmt"
	"log"
	"net/http"

	"github.com/ashupednekar/litewebservices-portal/internal/auth"
	"github.com/ashupednekar/litewebservices-portal/pkg/state"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type AuthHandlers struct {
	state     *state.AppState
	datastore auth.PasskeyStore
}

func NewAuthHandlers(state *state.AppState) *AuthHandlers{
	return &AuthHandlers{state: state}
}

func (h *AuthHandlers) BeginRegistration(ctx *gin.Context) {
	log.Printf("[INFO] begin registration ----------------------\\")

	username, err := auth.GetUsername(ctx)
	if err != nil {
		log.Printf("[ERRO] can't get user name: %s", err.Error())
		panic(err)
	}

	user := h.datastore.GetUser(username) // Find or create the new user

	options, session, err := h.state.Authn.BeginRegistration(user)
	if err != nil {
		msg := fmt.Sprintf("can't begin registration: %s", err.Error())
		log.Printf("[ERRO] %s", msg)
		ctx.JSON(http.StatusBadRequest, gin.H{"msg": msg})
		return
	}

	t := uuid.New().String()
	h.datastore.SaveSession(t, *session)

	ctx.JSON(http.StatusOK, options)
	// return the options generated with the session key
	// options.publicKey contain our registration options
}

func (h *AuthHandlers) FinishRegistration(ctx *gin.Context) {
	t := ctx.Request.Header.Get("Session-Key")
	// Get the session data stored from the function above
	session := h.datastore.GetSession(t) // FIXME: cover invalid session

	// In out example username == userID, but in real world it should be different    user := h.datastore.GetUser(string(session.UserID)) // Get the user

	credential, err := h.state.Authn.FinishRegistration(user, session, ctx.Request)
	if err != nil {
		msg := fmt.Sprintf("can't finish registration: %s", err.Error())
		log.Printf("[ERRO] %s", msg)
		ctx.JSON(http.StatusBadRequest, gin.H{"msg": msg})
		return
	}

	user.AddCredential(credential)
	h.datastore.SaveUser(user)
	// Delete the session data
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

	user := h.datastore.GetUser(username) // Find the user

	options, session, err := h.state.Authn.BeginLogin(user)
	if err != nil {
		msg := fmt.Sprintf("can't begin login: %s", err.Error())
		log.Printf("[ERRO] %s", msg)
		ctx.JSON(http.StatusBadRequest, gin.H{"msg": msg})
		return
	}

	// Make a session key and store the sessionData values
	t := uuid.New().String()
	h.datastore.SaveSession(t, *session)
	ctx.JSON(http.StatusOK, options)
}

func (h *AuthHandlers) FinishLogin(ctx *gin.Context) {
	// Get the session key from the header
	t := ctx.Request.Header.Get("Session-Key")
	// Get the session data stored from the function above
	session := h.datastore.GetSession(t) // FIXME: cover invalid session

	// In out example username == userID, but in real world it should be different
	user := h.datastore.GetUser(string(session.UserID)) // Get the user

	credential, err := h.state.Authn.FinishLogin(user, session, ctx.Request)
	if err != nil {
		log.Printf("[ERRO] can't finish login %s", err.Error())
		panic(err)
	}

	// Handle credentialog.Authenticator.CloneWarning
	// if credentialog.Authenticator.CloneWarning {
	// 	log.Printf("[WARN] can't finish login: %s", "CloneWarning")
	// }

	// If login was successful, update the credential object
	user.UpdateCredential(credential)
	h.datastore.SaveUser(user)
	// Delete the session data
	h.datastore.DeleteSession(t)

	log.Printf("[INFO] finish login ----------------------/")
	ctx.JSON(http.StatusOK, gin.H{"msg": "Login Success"})
}
