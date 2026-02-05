package pkg

import (
	"fmt"
	"sync"

	"go-simpler.org/env"
)

type IngestorConf struct {
	ListenPort   int    `env:"LISTEN_PORT" default:"3000"`
	NatsUrl      string `env:"NATS_URL" default:"nats://litefunctions-nats:4222"`
	ReplyTimeout string `env:"REPLY_TIMEOUT" default:"500ms"`
	OperatorUrl  string `env:"OPERATOR_URL" default:"litefunctions-operator:50051"`
}

var (
	Settings *IngestorConf
	once     sync.Once
)

func LoadSettings() (*IngestorConf, error) {
	settings := IngestorConf{}
	once.Do(func() {
		err := env.Load(&settings, nil)
		if err != nil {
			fmt.Printf("improperly configured: %v", err)
		}
	})
	Settings = &settings
	return Settings, nil
}
