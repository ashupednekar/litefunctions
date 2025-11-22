package adaptors

import (
	"context"
	"encoding/json"
	"log"

	"github.com/ashupednekar/litewebservices-portal/internal/auth"
	"github.com/go-webauthn/webauthn/protocol"
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
		log.Printf("session not found: %v", err)
		return webauthn.SessionData{}, false
	}

	var credParams []protocol.CredentialParameter
	_ = json.Unmarshal(row.CredParams, &credParams)

	var extensions protocol.AuthenticationExtensions
	_ = json.Unmarshal(row.Extensions, &extensions)

	sess := webauthn.SessionData{
		Challenge:            string(row.Challenge),
		RelyingPartyID:       row.RpID.String,
		UserID:               row.UserID,
		AllowedCredentialIDs: row.AllowedCredentials,
		Expires:              row.ExpiresAt.Time,
		CredParams:           credParams,
		Extensions:           extensions,
		UserVerification:     protocol.UserVerificationRequirement(row.UserVerification.String),
		Mediation:            protocol.CredentialMediationRequirement(row.Mediation.String),
	}

	return sess, true
}
