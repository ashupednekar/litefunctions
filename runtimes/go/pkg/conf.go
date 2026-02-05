package pkg

import (
	"fmt"
	"sync"

	"go-simpler.org/env"
)

type Settings struct {
	Project string `env:"PROJECT"`
	Name    string `env:"NAME"`

	DatabaseUrl   string `env:"DATABASE_URL"`
	RedisUrl      string `env:"REDIS_URL"`
	RedisPassword string `env:"REDIS_PASSWORD"`
	NatsUrl       string `env:"NATS_URL"`

	OtlpHost     string `env:"OTLP_HOST"`
	OtlpPort     string `env:"OTLP_PORT"`
	UseTelemetry bool   `env:"USE_TELEMETRY"`
}

var (
	settings *Settings
	once     sync.Once
)

func LoadSettings() *Settings {
	once.Do(func() {
		settings = &Settings{}
		err := env.Load(settings, nil)
		if err != nil {
			fmt.Printf("%s", err)
		}
	})
	return settings
}
