package auth

import (
	"fmt"

	"github.com/ashupednekar/litewebservices-portal/pkg"
	"github.com/go-webauthn/webauthn/webauthn"
)

type PasskeyUser interface {
	webauthn.User
	AddCredential(*webauthn.Credential)
	UpdateCredential(*webauthn.Credential)
}

type PasskeyStore interface {
	GetUser(userName string) PasskeyUser
	SaveUser(PasskeyUser)
	GetSession(token string) webauthn.SessionData
	SaveSession(token string, data webauthn.SessionData)
	DeleteSession(token string)
}

func NewWebauthn() (*webauthn.WebAuthn, error){
	cfg := &webauthn.Config{
		RPDisplayName: "Lite web services",
		RPID: pkg.Cfg.Fqdn,
		RPOrigins: []string{
			fmt.Sprintf("http:%s", pkg.Cfg.Fqdn),
			fmt.Sprintf("https:%s", pkg.Cfg.Fqdn),
		},
	}
	return webauthn.New(cfg)
}
