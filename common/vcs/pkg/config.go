package pkg

import (
	"github.com/ashupednekar/litefunctions/common/vcs/internal"
	"go-simpler.org/env"
)

type Config = internal.Config

func LoadCfg() (*internal.Config, error) {
	var cfg internal.Config
	if err := env.Load(&cfg, nil); err != nil {
		return nil, err
	}
	return &cfg, nil
}
