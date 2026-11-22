package state

import (
	"fmt"

	"github.com/ashupednekar/litefunctions/common/agent"
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
	Agent  *agent.Service
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
	var agentService *agent.Service
	if config.Cfg.AgentEnabled {
		agentService = agent.NewService(agent.Config{
			BaseURL:        config.Cfg.AgentUrl,
			Username:       config.Cfg.AgentUsername,
			Password:       config.Cfg.AgentPassword,
			Model:          config.Cfg.AgentModel,
			Agent:          config.Cfg.AgentName,
			TimeoutSeconds: config.Cfg.AgentTimeout,
		})
	}

	return &AppState{
		Authn:  authn,
		DBPool: connections.DBPool,
		Broker: nc,
		Agent:  agentService,
	}, nil
}
