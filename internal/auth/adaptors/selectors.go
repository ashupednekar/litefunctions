package adaptors

import (
	"context"
	"log"

	"github.com/ashupednekar/litewebservices-portal/internal/auth"
	"github.com/go-webauthn/webauthn/webauthn"
)

func (db *WebauthnStore) GetUser(userName string) (*auth.User, error) {
	u, err := db.queries.GetUserByName(context.Background(), userName)
	return &auth.User{
		ID:          u.ID,
		Name:        u.Name,
		DisplayName: u.DisplayName,
	}, err
}

func (db WebauthnStore) GetSession(token string) (webauthn.SessionData, bool) {
	log.Printf("looking for session token: %s", token)
	row, err := db.queries.GetSession(context.Background(), token)
	if err != nil {
		log.Printf("%s - session not found, using new session", err)
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
