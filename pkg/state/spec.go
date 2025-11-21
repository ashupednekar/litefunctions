package state

import (
	"fmt"

	"github.com/ashupednekar/litewebservices-portal/internal/auth"
	"github.com/go-webauthn/webauthn/webauthn"
)

type AppState struct {
	Authn *webauthn.WebAuthn
}

func NewState() (*AppState, error) {
	authn, err := auth.NewWebauthn()
	if err != nil {
		return nil, fmt.Errorf("couldn't initialize state - webauthn: %s", err)
	}
	return &AppState{Authn: authn}, nil
}
