package pkg

import (
	"log"
	"sync"

	"go-simpler.org/env"
)

type Settings struct {
	Port                    int    `env:"LISTEN_PORT" default:"3000"`
	Fqdn                    string `env:"FQDN" default:"localhost"`
	DatabaseUrl             string `env:"DATABASE_URL,required"`
	DatabaseSchema          string `env:"DATABASE_SCHEMA" default:"lwsportal"`
	DatabaseConnTimeout     string `env:"DATABASE_CONN_TIMEOUT" default:"10s"`
	DatabaseMaxConns        int32  `env:"DATABASE_MAX_CONNS" default:"20"`
	DatabaseMinConns        int32  `env:"DATABASE_MIN_CONNS" default:"5"`
	DatabaseMaxConnLifetime string `env:"DATABASE_MAX_CONN_LIFETIME" default:"1h"`
	DatabaseMaxConnIdleTime string `env:"DATABASE_MAX_CONN_IDLETIME" default:"10m"`
	SessionExpiry           string `env:"SESSION_EXPIRY" default:"1h"`
	//Vcs conf
	VcsAuthMode             string `env:"GIT_AUTH_MODE" default:"ssh"`
	VcsPrivKeyPath       string `env:"GIT_PRIVATE_KEY_PATH" default:"/app/.ssh/privkey.pem"` 
	VcsPrivKeyPassword string `env:"GIT_PRIVATE_KEY_PASSWORD"`
	VcsToken                string `env:"GIT_TOKEN"`
	VcsUser string `env:"VCS_USER"`
	VcsVendor string `env:"VCS_VENDOR"`
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

