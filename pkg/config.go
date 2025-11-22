package pkg

import (
	"log"
	"sync"

	"go-simpler.org/env"
)

type Settings struct {
	Port int    `env:"LISTEN_PORT,required"`
	Fqdn string `env:"FQDN,required"`
}

var (
	once    sync.Once
	Cfg     Settings
	LoadErr error
)

func LoadCfg() {
	once.Do(func() {
		if err := env.Load(&Cfg, nil); err != nil {
			LoadErr = err
		}
	})
	if LoadErr != nil {
		log.Fatal(LoadErr)
	}
}
