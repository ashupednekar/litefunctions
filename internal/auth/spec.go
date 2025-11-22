package auth

import (
	"fmt"
	"log"

	"github.com/ashupednekar/litewebservices-portal/pkg"
	"github.com/go-webauthn/webauthn/webauthn"
)

type PasskeyUser interface {
	webauthn.User
	AddCredential(*webauthn.Credential)
	UpdateCredential(*webauthn.Credential)
}

type PasskeyStore interface {
	GetOrCreateUser(userName string) (PasskeyUser, error)
	SaveUser(PasskeyUser) error
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

	creds []webauthn.Credential
}

func (o *User) WebAuthnID() []byte {
	return o.ID
}

func (o *User) WebAuthnName() string {
	return o.Name
}

func (o *User) WebAuthnDisplayName() string {
	return o.DisplayName
}

func (o *User) WebAuthnIcon() string {
	return "https://pics.com/avatar.png"
}

func (o *User) WebAuthnCredentials() []webauthn.Credential {
	return o.creds
}

func (o *User) AddCredential(credential *webauthn.Credential) {
	o.creds = append(o.creds, *credential)
}

func (o *User) UpdateCredential(credential *webauthn.Credential) {
	for i, c := range o.creds {
		if string(c.ID) == string(credential.ID) {
			o.creds[i] = *credential
		}
	}
}
