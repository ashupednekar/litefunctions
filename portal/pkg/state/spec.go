package state

import (
	"fmt"

	"github.com/ashupednekar/litefunctions/portal/internal/auth"
	"github.com/ashupednekar/litefunctions/portal/pkg/state/connections"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AppState struct {
	Authn  *webauthn.WebAuthn
	DBPool *pgxpool.Pool
}

func NewState() (*AppState, error) {
	authn, err := auth.NewWebauthn()
	if err != nil {
		return nil, fmt.Errorf("couldn't initialize state - webauthn: %s", err)
	}
	connections.ConnectDB()
	return &AppState{Authn: authn, DBPool: connections.DBPool}, nil
}
