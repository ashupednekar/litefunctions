package pkg

import (
	"fmt"
	"sync"

	"go-simpler.org/env"
)

type IngestorConf struct{
	ListenPort int `env:"LISTEN_PORT"`
	NatsUrl string `env:"NATS_URL"`
	ReplyTimeout string `env:"REPLY_TIMEOUT"`
}

var (
	Settings *IngestorConf
	once sync.Once
)

func LoadSettings() (*IngestorConf, error) {
	settings := IngestorConf{ListenPort: 3000, NatsUrl: "nats://localhost:4222", ReplyTimeout: "500ms"}
	once.Do(func(){
		err := env.Load(&settings, nil)
		if err != nil{
			fmt.Printf("improperly configured: %v", err)
		}
	})
	Settings = &settings
	return Settings, nil
}
