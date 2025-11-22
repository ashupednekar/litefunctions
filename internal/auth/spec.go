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
	SaveSession(token string, data webauthn.SessionData) error
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
