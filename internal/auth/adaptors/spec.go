package adaptors

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log"

	"github.com/ashupednekar/litewebservices-portal/internal/auth"
	"github.com/go-webauthn/webauthn/webauthn"
)

type WebauthnStore struct {
	queries *Queries
}

func NewWebauthnStore(q *Queries) *WebauthnStore {
	return &WebauthnStore{queries: q}
}

func (db WebauthnStore) GenSessionID() (string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

func (db WebauthnStore) GetSession(token string) (webauthn.SessionData, bool) {
	row, err := db.queries.GetSession(context.Background(), token)
	if err != nil {
		log.Println("session not found, using new session")
		return webauthn.SessionData{}, false
	}
	session := webauthn.SessionData{
		Challenge: string(row.Challenge),
		UserID:    []byte(row.UserName),
		Expires:   row.ExpiresAt.Time,
	}
	for _, b := range row.AllowedCredentials {
		session.AllowedCredentialIDs = append(session.AllowedCredentialIDs, b)
	}
	return session, true
}

func (db WebauthnStore) SaveSession(token string, data webauthn.SessionData) error {
	log.Printf("[DEBUG] SaveSession: %s - %v", token, data)
	var allowed [][]byte
	for _, c := range data.AllowedCredentialIDs {
		allowed = append(allowed, c)
	}
	err := db.queries.SaveSession(
		context.Background(),
		SaveSessionParams{
			SessionID:          token,
			UserName:           string(data.UserID),
			Challenge:          []byte(data.Challenge),
			UserID:             data.UserID,
			AllowedCredentials: allowed,
		},
	)
	if err != nil {
		return fmt.Errorf("error saving session to db - %s", err)
	}
	return nil
}

func (db WebauthnStore) DeleteSession(token string) error {
	log.Printf("[DEBUG] DeleteSession: %v", token)
	return db.queries.DeleteSession(context.Background(), token)
}

func (db *WebauthnStore) GetOrCreateUser(userName string) (auth.PasskeyUser, error) {
	log.Printf("[DEBUG] GetOrCreateUser: %v", userName)

	u, err := db.queries.GetUserByName(context.Background(), userName)
	if err == nil {
		return &auth.User{
			ID:          u.ID,
			Name:        u.Name,
			DisplayName: u.DisplayName,
		}, err
	}

	newUser := &auth.User{
		ID:          []byte(userName),
		Name:        userName,
		DisplayName: userName,
	}

	err = db.queries.CreateUser(context.Background(), CreateUserParams{
		ID:          newUser.ID,
		Name:        newUser.Name,
		DisplayName: newUser.DisplayName,
		//Icon:        "https://pics.com/avatar.png",
	})

	if err != nil {
		return nil, fmt.Errorf("error creating new user: %s", err)
	}
	return newUser, nil
}

func (db *WebauthnStore) SaveUser(user auth.PasskeyUser) error {
	err := db.queries.UpdateUser(
		context.Background(),
		UpdateUserParams{
			ID:          user.WebAuthnID(),
			DisplayName: user.WebAuthnDisplayName(),
			//Icon:        user.WebAuthnIcon(),
		},
	)
	if err != nil {
		return err
	}
	return nil
}
