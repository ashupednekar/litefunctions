package adaptors

import (
	"context"
	"encoding/json"
	"log"

	"github.com/ashupednekar/litewebservices-portal/internal/auth"
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
)

// convertDBCredentialToWebauthn converts a database credential to a webauthn.Credential
func convertDBCredentialToWebauthn(c Credential) webauthn.Credential {
	transports := make([]protocol.AuthenticatorTransport, 0, len(c.Transports))
	for _, t := range c.Transports {
		transports = append(transports, protocol.AuthenticatorTransport(t))
	}

	raw := protocol.AuthenticatorFlags(c.Flags)
	credFlags := webauthn.NewCredentialFlags(raw)

	return webauthn.Credential{
		ID:              c.ID,
		PublicKey:       c.PublicKey,
		AttestationType: c.AttestationType.String,
		Transport:       transports,
		Flags:           credFlags,
		Authenticator: webauthn.Authenticator{
			SignCount: uint32(c.SignCount),
		},
	}
}

func (db *WebauthnStore) GetUser(userName string) (*auth.User, error) {
	u, err := db.queries.GetUserByName(context.Background(), userName)
	if err != nil {
		return nil, err
	}
	creds, err := db.queries.GetCredentialsForUser(context.Background(), u.ID)
	if err != nil {
		return nil, err
	}

	webCreds := make([]webauthn.Credential, 0, len(creds))
	for _, c := range creds {
		webCreds = append(webCreds, convertDBCredentialToWebauthn(c))
	}

	return &auth.User{
		ID:          u.ID,
		Name:        u.Name,
		DisplayName: u.DisplayName,
		Creds:       webCreds, // ⭐ critical
	}, nil
}

func (db WebauthnStore) GetSession(token string) (webauthn.SessionData, bool) {
	log.Printf("looking for session token: %s", token)

	row, err := db.queries.GetSession(context.Background(), token)
	if err != nil {
		log.Printf("session not found: %v", err)
		return webauthn.SessionData{}, false
	}

	var credParams []protocol.CredentialParameter
	if err := json.Unmarshal(row.CredParams, &credParams); err != nil {
		log.Printf("[WARN] failed to unmarshal credParams: %v", err)
	}

	var extensions protocol.AuthenticationExtensions
	if err := json.Unmarshal(row.Extensions, &extensions); err != nil {
		log.Printf("[WARN] failed to unmarshal extensions: %v", err)
	}

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

func (db WebauthnStore) GetCredentialsForUser(user auth.PasskeyUser) ([]webauthn.Credential, error) {
	rows, err := db.queries.GetCredentialsForUser(context.Background(), user.WebAuthnID())
	if err != nil {
		return nil, err
	}

	creds := make([]webauthn.Credential, 0, len(rows))

	for _, row := range rows {
		creds = append(creds, convertDBCredentialToWebauthn(row))
	}

	return creds, nil
}

// GetUserSession retrieves a user session by session ID
func (db *WebauthnStore) GetUserSession(sessionID string) (userID []byte, found bool, err error) {
	session, err := db.queries.GetUserSession(context.Background(), sessionID)
	if err != nil {
		log.Printf("[DEBUG] session not found: %v", err)
		return nil, false, nil
	}
	return session.UserID, true, nil
}
