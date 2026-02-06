package pkg

import (
	"context"
	"fmt"
	"strings"
	"log/slog"

	"github.com/nats-io/nats.go/jetstream"
)

func Consume(ctx context.Context, state *AppState, c jetstream.Consumer) error {
	logger := slog.Default().With(
		"project", settings.Project,
		"function", settings.Name,
	)
	logger.Info("waiting for messages")
	cc, err := c.Consume(func(msg jetstream.Msg){
		logger.Info("received event", "subject", msg.Subject())
		msg.Ack()
		parts := strings.Split(msg.Subject(), ".")
		req_id := parts[len(parts)-1]
		logger.Info("request id extracted", "request_id", req_id)
		res, err := Handler(state, &req_id, msg.Data())
		if err != nil{
			logger.Error("function handler failed", "error", err, "request_id", req_id)
		}
		if err := state.Nc.Publish(fmt.Sprintf("%s.%s.res.go.%s", settings.Project, settings.Name, req_id), res); err != nil {
			logger.Error("failed to publish response", "error", err, "request_id", req_id)
		}
	})
	if err != nil{
		return err
	}
	defer cc.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-cc.Closed():
		return fmt.Errorf("ERR-CONSUMER-CLOSED")
	}
}

func StartFunction(ctx context.Context, state *AppState) error {
	settings := LoadSettings()
	logger := slog.Default().With(
		"project", settings.Project,
		"function", settings.Name,
	)
	name := fmt.Sprintf("%s-%s", settings.Project, settings.Name)
	subject := fmt.Sprintf("%s.%s.exec.go.*", settings.Project, settings.Name)
	logger.Info("starting consumer", "subject", subject, "durable", name)
	consumer, err := state.Js.CreateOrUpdateConsumer(ctx, jetstream.ConsumerConfig{
		Durable: name,
		FilterSubject: subject,
	})
	if err != nil{
		return fmt.Errorf("ERR-CONSUMER-INIT: %v", err)
	}
	err = Consume(ctx, state, consumer)
	if err != nil{
		return fmt.Errorf("ERR-CONSUMER: %v", err)
	}
	return nil
}
