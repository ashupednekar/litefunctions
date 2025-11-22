package connections

import (
	"context"
	"crypto/tls"
	"log"
	"sync"
	"time"

	"github.com/ashupednekar/litewebservices-portal/pkg"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	DBPool  *pgxpool.Pool
	once    sync.Once
	connErr error
)

func ConnectDB() {
	log.Println("Go connecting to:", pkg.Cfg.DatabaseUrl)
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

		config.ConnConfig.TLSConfig = &tls.Config{
			InsecureSkipVerify: true,
		}

		config.MaxConns = pkg.Cfg.DatabaseMaxConns
		config.MinConns = pkg.Cfg.DatabaseMinConns

		if d, err := time.ParseDuration(pkg.Cfg.DatabaseMaxConnLifetime); err == nil {
			config.MaxConnLifetime = d
		}
		if d, err := time.ParseDuration(pkg.Cfg.DatabaseMaxConnIdleTime); err == nil {
			config.MaxConnIdleTime = d
		}

		if pkg.Cfg.DatabaseSchema != "" {
			config.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
				_, err := conn.Exec(ctx, "create schema if not exist "+pkg.Cfg.DatabaseSchema)
				_, err = conn.Exec(ctx, "set search_path to "+pkg.Cfg.DatabaseSchema)
				return err
			}
		}

		DBPool, err = pgxpool.NewWithConfig(ctx, config)
		if err != nil {
			connErr = err
			return
		}
	})
}
