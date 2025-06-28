package pkg

import (
	"fmt"
	"sync"

	"go-simpler.org/env"
)

type IngestorConf struct{
	ListenPort int `env:"LISTEN_PORT"`
	NatsBrokerUrl string `env:"NATS_BROKER_URL"`
	ReplyTimeout string `env:"REPLY_TIMEOUT"`
}

var (
	Settings *IngestorConf
	once sync.Once
)

func LoadSettings() (*IngestorConf, error) {
	settings := IngestorConf{ListenPort: 3000, NatsBrokerUrl: "nats://localhost:4222", ReplyTimeout: "500ms"}
	once.Do(func(){
		err := env.Load(&settings, nil)
		if err != nil{
			fmt.Printf("improperly configured: %v", err)
		}
	})
	return &settings, nil
}
