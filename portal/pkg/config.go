package pkg

import (
	"log"
	"sync"

	"github.com/ashupednekar/litefunctions/common/vcs"
	"go-simpler.org/env"
)

type Settings struct {
	Port                    int    `env:"LISTEN_PORT" default:"3000"`
	Fqdn                    string `env:"FQDN" default:"localhost"`
	DatabaseUrl             string `env:"DATABASE_URL,required"`
	DatabaseSchema          string `env:"DATABASE_SCHEMA" default:"litefunctions"`
	DatabaseConnTimeout     string `env:"DATABASE_CONN_TIMEOUT" default:"10s"`
	DatabaseMaxConns        int32  `env:"DATABASE_MAX_CONNS" default:"20"`
	DatabaseMinConns        int32  `env:"DATABASE_MIN_CONNS" default:"5"`
	DatabaseMaxConnLifetime string `env:"DATABASE_MAX_CONN_LIFETIME" default:"1h"`
	DatabaseMaxConnIdleTime string `env:"DATABASE_MAX_CONN_IDLETIME" default:"10m"`
	SessionExpiry           string `env:"SESSION_EXPIRY" default:"1h"`
	OperatorUrl             string `env:"OPERATOR_URL" default:"litefunctions-operator:50051"`
	PodNamespace            string `env:"POD_NAMESPACE" default:"default"`
	IngestorUrl             string `env:"INGESTOR_URL" default:"http://litefunctions-ingestor:3000"`
	vcs                     vcs.Config
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
	Cfg.vcs = vcs.C
}
