package adaptors

import (
	"context"
	"fmt"
	"log"

	"github.com/ashupednekar/litewebservices-portal/internal/auth"
	"github.com/go-webauthn/webauthn/webauthn"
)



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

func (db *WebauthnStore) CreateUser(userName string) (auth.PasskeyUser, error) {
	newUser := &auth.User{
		ID:          []byte(userName),
		Name:        userName,
		DisplayName: userName,
	}
	err := db.queries.CreateUser(context.Background(), CreateUserParams{
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
