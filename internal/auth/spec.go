package auth

import (
	"fmt"
	"log"

	"github.com/ashupednekar/litewebservices-portal/pkg"
	"github.com/go-webauthn/webauthn/webauthn"
)

type PasskeyUser interface {
	webauthn.User
	AddCredential(*webauthn.Credential) error
	UpdateCredential(*webauthn.Credential) error
}

type PasskeyStore interface {
    GetOrCreateUser(userName string) (PasskeyUser, error)
    SaveUser(PasskeyUser) error

    SaveCredential(user PasskeyUser, cred *webauthn.Credential) error
    UpdateCredential(user PasskeyUser, cred *webauthn.Credential) error
    GetCredentialsForUser(user PasskeyUser) ([]webauthn.Credential, error)

    GetSession(token string) (webauthn.SessionData, bool)
    SaveSession(username string, token string, data webauthn.SessionData) error
    DeleteSession(token string) error
}

func NewWebauthn() (*webauthn.WebAuthn, error) {
	cfg := &webauthn.Config{
		RPDisplayName: "Lite web services",
		RPID:          pkg.Cfg.Fqdn,
		RPOrigins: []string{
			fmt.Sprintf("http://%s:%d", pkg.Cfg.Fqdn, pkg.Cfg.Port),
			fmt.Sprintf("https://%s", pkg.Cfg.Fqdn),
		},
	}
	log.Printf("webauthn fqdn: %v", pkg.Cfg.Fqdn)
	log.Printf("webauthn origins: %v", cfg.RPOrigins)
	return webauthn.New(cfg)
}

type User struct {
	ID          []byte
	DisplayName string
	Name        string

	Creds []webauthn.Credential
}

func (u *User) WebAuthnID() []byte {
	return u.ID
}

func (u *User) WebAuthnName() string {
	return u.Name
}

func (u *User) WebAuthnDisplayName() string {
	return u.DisplayName
}

func (u *User) WebAuthnIcon() string {
	//TODO: later, actual icon, maybe
	return "https://pics.com/avatar.png"
}

func (u *User) WebAuthnCredentials() []webauthn.Credential {
	return u.Creds
}

func (u *User) AddCredential(credential *webauthn.Credential) error {
	u.Creds = append(u.Creds, *credential)
	return nil
}

func (u *User) UpdateCredential(credential *webauthn.Credential) error {
	for i, c := range u.Creds {
		if string(c.ID) == string(credential.ID) {
			u.Creds[i] = *credential
		}
	}
	return nil
}
