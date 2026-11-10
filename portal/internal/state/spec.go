package state

import (
	"fmt"

	"github.com/ashupednekar/litefunctions/portal/internal/auth"
	"github.com/ashupednekar/litefunctions/portal/internal/config"
	"github.com/ashupednekar/litefunctions/portal/internal/state/connections"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
)

type AppState struct {
	Authn  *webauthn.WebAuthn
	DBPool *pgxpool.Pool
	Broker *nats.Conn
}

func NewState() (*AppState, error) {
	authn, err := auth.NewWebauthn()
	if err != nil {
		return nil, fmt.Errorf("couldn't initialize state - webauthn: %s", err)
	}
	connections.ConnectDB()
	nc, err := nats.Connect(config.Cfg.NatsUrl)
	if err != nil {
		return nil, err
	}
	return &AppState{Authn: authn, DBPool: connections.DBPool, Broker: nc}, nil
}
