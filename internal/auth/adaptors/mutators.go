package adaptors

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/ashupednekar/litewebservices-portal/internal/auth"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/jackc/pgx/v5/pgtype"
)

func (db WebauthnStore) SaveSession(username string, token string, data webauthn.SessionData) error {
	log.Printf("SaveSession: %s - %v", token, data)

	allowed := make([][]byte, len(data.AllowedCredentialIDs))
	copy(allowed, data.AllowedCredentialIDs)

	credParamsJSON, _ := json.Marshal(data.CredParams)
	extensionsJSON, _ := json.Marshal(data.Extensions)

	err := db.queries.SaveSession(
		context.Background(),
		SaveSessionParams{
			SessionID:          token,
			UserName:           username,
			Challenge:          []byte(data.Challenge),
			UserID:             data.UserID,
			AllowedCredentials: allowed,
			ExpiresAt:          pgtype.Timestamptz{Time: data.Expires, Valid: true},
			RpID:               pgtype.Text{String: data.RelyingPartyID},
			CredParams:         credParamsJSON,
			Extensions:         extensionsJSON,
			UserVerification:   pgtype.Text{String: string(data.UserVerification)},
			Mediation:          pgtype.Text{String: string(data.Mediation)},
		},
	)
	if err != nil {
		return fmt.Errorf("save session failed: %w", err)
	}
	return nil
}

func (db WebauthnStore) DeleteSession(token string) error {
	log.Printf("DeleteSession: %v", token)
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
	fmt.Printf("created new user: %v\n", newUser)
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
