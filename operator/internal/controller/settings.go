package controller

import (
	"sync"
	"time"

	"github.com/go-logr/logr"
	"go-simpler.org/env"
)

type Settings struct {
	Registry            string        `env:"REGISTRY" default:"ghcr.io"`
	VcsUser             string        `env:"VCS_USER" default:"lwsrepos"`
	VcsBaseUrl          string        `env:"VCS_BASE_URL" default:"https://github.com"`
	GitTokenSecretName  string        `env:"GIT_TOKEN_SECRET_NAME" default:"litefunctions-admin-token"`
	GitTokenSecretKey   string        `env:"GIT_TOKEN_SECRET_KEY" default:"token"`
	PullSecret          string        `env:"PULL_SECRET" default:"ghcr-secret"`
	DbSecretName        string        `env:"DB_SECRET_NAME" default:"litefunctions-pguser-litefunctions"`
	DbSecretKey         string        `env:"DB_SECRET_KEY" default:"pgbouncer-uri"`
	RedisUrl            string        `env:"REDIS_URL" default:"redis://litefunctions-valkey-cluster:6379"`
	RedisPassword       string        `env:"REDIS_PASSWORD" default:""`
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
	})
	if LoadErr != nil {
		logger.Error(LoadErr, "error loading settings")
	}
}
