package controller

import (
	"sync"
	"time"

	vcs "github.com/ashupednekar/litefunctions/common/vcs/pkg"
	"github.com/go-logr/logr"
	"go-simpler.org/env"
)

type Settings struct {
	Registry            string        `env:"REGISTRY" default:"ghcr.io"`
	VcsUser             string        `env:"VCS_USER" default:"lwsrepos"`
	VcsBaseUrl          string        `env:"VCS_BASE_URL" default:"https://github.com"`
	VcsAuthMode         string        `env:"VCS_AUTH_MODE" default:"token"`
	VcsPrivKeyPath      string        `env:"VCS_PRIVATE_KEY_PATH" default:"/app/.ssh/privkey.pem"`
	VcsPrivKeyPassword  string        `env:"VCS_PRIVATE_KEY_PASSWORD"`
	VcsToken            string        `env:"VCS_TOKEN"`
	VcsVendor           string        `env:"VCS_VENDOR" default:"gitea"`
	VcsPublicBaseUrl    string        `env:"VCS_PUBLIC_BASE_URL"`
	Fqdn                string        `env:"FQDN" default:"localhost"`
	GitTokenSecretName  string        `env:"GIT_TOKEN_SECRET_NAME" default:"litefunctions-admin-token"`
	GitTokenSecretKey   string        `env:"GIT_TOKEN_SECRET_KEY" default:"token"`
	PullSecret          string        `env:"PULL_SECRET" default:"ghcr-secret"`
	DbSecretName        string        `env:"DB_SECRET_NAME" default:"litefunctions-pguser-litefunctions"`
	DbSecretKey         string        `env:"DB_SECRET_KEY" default:"pgbouncer-uri"`
	RedisUrl            string        `env:"REDIS_URL" default:"redis://litefunctions-valkey:6379"`
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

func (s Settings) VCSConfig() vcs.Config {
	return vcs.Config{
		AuthMode:        s.VcsAuthMode,
		PrivKeyPath:     s.VcsPrivKeyPath,
		PrivKeyPassword: s.VcsPrivKeyPassword,
		Token:           s.VcsToken,
		User:            s.VcsUser,
		Vendor:          s.VcsVendor,
		BaseURL:         s.VcsBaseUrl,
		PublicBaseURL:   s.VcsPublicBaseUrl,
		FQDN:            s.Fqdn,
	}
}
