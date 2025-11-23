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
	if err != nil {
		return nil, err
	}
	creds, err := db.queries.GetCredentialsForUser(context.Background(), u.ID)
	if err != nil {
		return nil, err
	}

	webCreds := make([]webauthn.Credential, 0, len(creds))
	for _, c := range creds {
		transports := make([]protocol.AuthenticatorTransport, 0, len(c.Transports))
		for _, t := range c.Transports {
			transports = append(transports, protocol.AuthenticatorTransport(t))
		}

		raw := protocol.AuthenticatorFlags(c.Flags)
		credFlags := webauthn.NewCredentialFlags(raw)

		webCreds = append(webCreds, webauthn.Credential{
			ID:              c.ID,
			PublicKey:       c.PublicKey,
			AttestationType: c.AttestationType.String,
			Transport:       transports,
			Flags:           credFlags,
			Authenticator: webauthn.Authenticator{
				SignCount: uint32(c.SignCount),
			},
		})
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

func (db WebauthnStore) GetCredentialsForUser(user auth.PasskeyUser) ([]webauthn.Credential, error) {
	rows, err := db.queries.GetCredentialsForUser(context.Background(), user.WebAuthnID())
	if err != nil {
		return nil, err
	}

	creds := make([]webauthn.Credential, 0, len(rows))

	for _, row := range rows {
		transports := make([]protocol.AuthenticatorTransport, 0, len(row.Transports))
		for _, t := range row.Transports {
			transports = append(transports, protocol.AuthenticatorTransport(t))
		}
		raw := protocol.AuthenticatorFlags(row.Flags)
		credFlags := webauthn.NewCredentialFlags(raw)
		creds = append(creds, webauthn.Credential{
			ID:              row.ID,
			PublicKey:       row.PublicKey,
			AttestationType: row.AttestationType.String,
			Transport:       transports,

			Flags: credFlags,

			Authenticator: webauthn.Authenticator{
				SignCount: uint32(row.SignCount),
			},
		})
	}

	return creds, nil
}
