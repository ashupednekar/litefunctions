package pkg

import (
	"context"
	"fmt"

	"github.com/go-redis/redis/v8"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

type AppState struct{
	DBPool *pgxpool.Pool
	RedisClient *redis.Client
	Nc *nats.Conn
	Js jetstream.Stream
}

func NewAppState(ctx context.Context) (*AppState, error){
  settings := LoadSettings()

	dbPool, err := pgxpool.New(ctx, settings.DatabaseUrl)
	if err != nil{
		return nil, fmt.Errorf("ERR-DB-CONN: %v", err)
	}

	redisClient := redis.NewClient(&redis.Options{
		Addr: settings.RedisUrl,
	})
	if err := redisClient.Ping(ctx).Err(); err != nil{
		return nil, fmt.Errorf("ERR-REDIS-CONN: %v", err)
	}

	nc, err := nats.Connect(settings.NatsBrokerUrl)
	if err != nil{
		return nil, fmt.Errorf("ERR-NATS-CONN: %v", err)
	}

	js, err := jetstream.New(nc)
	if err != nil{
		return nil, fmt.Errorf("ERR-NATS-JS: %v", err)
	}
	streamConfig := jetstream.StreamConfig{
		Name: settings.Project,
		Subjects: []string{fmt.Sprintf("%s.>", settings.Project)},
	}
	stream, err := js.CreateOrUpdateStream(ctx, streamConfig)
	if err != nil{
		return nil, fmt.Errorf("ERR-NATS-STREAM: %v", err)
	}
  return &AppState{
		DBPool: dbPool,
		RedisClient: redisClient,
		Nc: nc,
		Js: stream,
	}, nil
}
