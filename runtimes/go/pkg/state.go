package pkg

import (
	"context"
	"fmt"
	"log"
	"net/url"

	"github.com/go-redis/redis/v8"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
)

type AppState struct {
	DBPool      *pgxpool.Pool
	RedisClient *redis.Client
	Nc          *nats.Conn
}

func NewAppState(ctx context.Context) (*AppState, error) {
	settings := LoadSettings()
	if err := validateSettings(settings); err != nil {
		return nil, err
	}
	log.Printf(
		"settings: project=%s name=%s nats_url=%s database_url_set=%t redis_url_set=%t\n",
		settings.Project,
		settings.Name,
		safeURLSummary(settings.NatsUrl),
		settings.DatabaseUrl != "",
		settings.RedisUrl != "",
	)

	dbPool, err := pgxpool.New(ctx, settings.DatabaseUrl)
	if err != nil {
		return nil, fmt.Errorf("ERR-DB-CONN: %v", err)
	}

	redisOptions, err := redis.ParseURL(settings.RedisUrl)
	if err != nil {
		return nil, fmt.Errorf("ERR-REDIS-PARSE: %v", err)
	}
	if redisOptions.Password == "" && settings.RedisPassword != "" {
		redisOptions.Password = settings.RedisPassword
	}
	redisClient := redis.NewClient(redisOptions)
	if err := redisClient.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("ERR-REDIS-CONN: %v", err)
	}
	if err := redisClient.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("ERR-REDIS-CONN: %v", err)
	}

	nc, err := nats.Connect(settings.NatsUrl)
	if err != nil {
		return nil, fmt.Errorf("ERR-NATS-CONN: %v", err)
	}

	return &AppState{
		DBPool:      dbPool,
		RedisClient: redisClient,
		Nc:          nc,
	}, nil
}

func validateSettings(settings *Settings) error {
	if settings.Project == "" {
		return fmt.Errorf("ERR-SETTINGS: PROJECT is required (set env PROJECT)")
	}
	if settings.Name == "" {
		return fmt.Errorf("ERR-SETTINGS: NAME is required (set env NAME)")
	}
	if settings.DatabaseUrl == "" {
		return fmt.Errorf("ERR-SETTINGS: DATABASE_URL is required (set env DATABASE_URL)")
	}
	if settings.RedisUrl == "" {
		return fmt.Errorf("ERR-SETTINGS: REDIS_URL is required (set env REDIS_URL)")
	}
	if settings.NatsUrl == "" {
		return fmt.Errorf("ERR-SETTINGS: NATS_URL is required (set env NATS_URL)")
	}
	return nil
}

func safeURLSummary(raw string) string {
	if raw == "" {
		return "(empty)"
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "(invalid)"
	}
	if u.Scheme == "" && u.Host == "" {
		return "(invalid)"
	}
	return fmt.Sprintf("%s://%s", u.Scheme, u.Host)
}
