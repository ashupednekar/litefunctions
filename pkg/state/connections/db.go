package connections

import (
	"context"
	"sync"
	"time"

	"github.com/ashupednekar/litewebservices-portal/pkg"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	DBPool  *pgxpool.Pool
	once    sync.Once
	connErr error
)

func ConnectDB() {
	once.Do(func() {
		timeout, err := time.ParseDuration(pkg.Cfg.DatabaseConnTimeout)
		if err != nil {
			connErr = err
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		config, err := pgxpool.ParseConfig(pkg.Cfg.DatabaseUrl)
		if err != nil {
			connErr = err
			return
		}
		config.MaxConns = pkg.Cfg.DatabaseMaxConns
		config.MinConns = pkg.Cfg.DatabaseMinConns
		t, err := time.ParseDuration(pkg.Cfg.DatabaseMaxConnLifetime)
		config.MaxConnLifetime = t
		t, err = time.ParseDuration(pkg.Cfg.DatabaseMaxConnIdleTime)
		config.MaxConnIdleTime = t
		DBPool, err = pgxpool.NewWithConfig(ctx, config)
		if err != nil {
			connErr = err
			return
		}
		defer DBPool.Close()
	})
}
