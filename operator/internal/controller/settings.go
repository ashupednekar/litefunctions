package controller

import (
	"os"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"go-simpler.org/env"
)

type Settings struct {
	Registry            string        `env:"REGISTRY" default:"ghcr.io"`
	RegistryUser        string        `env:"REGISTRY_USER" default:"lwsrepos"`
	PullSecret          string        `env:"PULL_SECRET" default:"ghcr-secret"`
	DbSecretName        string        `env:"DB_SECRET_NAME" default:"litefunctions-pguser-litefunctions"`
	DbSecretKey         string        `env:"DB_SECRET_KEY" default:"pgbouncer-uri"`
	RedisUrl            string        `env:"REDIS_URL" default:"redis://litefunctions-valkey-cluster:6379"`
	NatsUrl             string        `env:"NATS_URL" default:"nats://litefunctions-nats:4222"`
	DeprovisionDuration string        `env:"DEPROVISION_DURATION" default:"2m"`
	KeepWarmDuration    time.Duration `env:"KEEP_WARM_DURATION" default:"5m"`
}

var (
	once    sync.Once
	Cfg     Settings
	LoadErr error
)

func LoadCfg(logger logr.Logger) {
	once.Do(func() {
		if err := env.Load(&Cfg, nil); err != nil {
			LoadErr = err
		}
		if _, ok := os.LookupEnv("REGISTRY_USER"); !ok {
			if legacy, ok := os.LookupEnv("VCS_USER"); ok && legacy != "" {
				Cfg.RegistryUser = legacy
			}
		}
	})
	if LoadErr != nil {
		logger.Error(LoadErr, "error loading settings")
	}
}
