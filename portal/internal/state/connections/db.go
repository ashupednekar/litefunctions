package connections

import (
	"context"
	"crypto/tls"
	"log/slog"
	"sync"
	"time"

	"github.com/ashupednekar/litefunctions/portal/internal/config"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	DBPool  *pgxpool.Pool
	once    sync.Once
	connErr error
)

func ConnectDB() {
	slog.Info("DB connecting", "url", config.Cfg.DatabaseUrl)
	once.Do(func() {
		timeout, err := time.ParseDuration(config.Cfg.DatabaseConnTimeout)
		if err != nil {
			connErr = err
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		poolConfig, err := pgxpool.ParseConfig(config.Cfg.DatabaseUrl)
		if err != nil {
			connErr = err
			return
		}

		poolConfig.ConnConfig.TLSConfig = &tls.Config{
			InsecureSkipVerify: true,
		}

		poolConfig.MaxConns = config.Cfg.DatabaseMaxConns
		poolConfig.MinConns = config.Cfg.DatabaseMinConns

		if d, err := time.ParseDuration(config.Cfg.DatabaseMaxConnLifetime); err == nil {
			poolConfig.MaxConnLifetime = d
		}
		if d, err := time.ParseDuration(config.Cfg.DatabaseMaxConnIdleTime); err == nil {
			poolConfig.MaxConnIdleTime = d
		}

		if config.Cfg.DatabaseSchema != "" {
			poolConfig.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
				_, err := conn.Exec(ctx, "create schema if not exist "+config.Cfg.DatabaseSchema)
				_, err = conn.Exec(ctx, "set search_path to "+config.Cfg.DatabaseSchema)
				return err
			}
		}

		DBPool, err = pgxpool.NewWithConfig(ctx, poolConfig)
		if err != nil {
			connErr = err
			return
		}
	})
}
