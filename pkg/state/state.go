package state

import (
	"fmt"

	"github.com/ashupednekar/litewebservices-portal/internal/auth"
	"github.com/ashupednekar/litewebservices-portal/internal/auth/adaptors"
	"github.com/ashupednekar/litewebservices-portal/pkg/state/connections"
	"github.com/go-webauthn/webauthn/webauthn"
)

type AppState struct {
	Authn   *webauthn.WebAuthn
	Queries *adaptors.Queries
}

func NewState() (*AppState, error) {
	authn, err := auth.NewWebauthn()
	if err != nil {
		return nil, fmt.Errorf("couldn't initialize state - webauthn: %s", err)
	}
	connections.ConnectDB()
	queries := adaptors.New(connections.DBPool)
	return &AppState{Authn: authn, Queries: queries}, nil
}
