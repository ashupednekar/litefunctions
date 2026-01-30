package controller

import (
	"sync"

	"github.com/go-logr/logr"
	"go-simpler.org/env"
)

type Settings struct {
	Registry            string `env:"REGISTRY" default:"ghcr.io"`
	VcsUser             string `env:"VCS_USER" default:"lwsrepos"`
	PullSecret          string `env:"PULL_SECRET" default:"ghcr-secret"`
	DbSecretName        string `env:"DB_SECRET_NAME" default:"litefunctions-pguser-litefunctions"`
	DbSecretKey         string `env:"DB_SECRET_KEY" default:"pgbouncer-uri"`
	RedisUrl            string `env:"REDIS_URL" default:"redis://litefunctions-valkey-cluster:6379"`
	NatsUrl             string `env:"NATS_URL" default:"nats://litefunctions-nats:4222"`
	DeprovisionDuration string `env:"DEPROVISION_DURATION" default:"2m"`
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
	})
	if LoadErr != nil {
		logger.Error(LoadErr, "error loading settings")
	}
}
