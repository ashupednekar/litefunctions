package pkg

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/nats-io/nats.go/jetstream"
)

func Consume(ctx context.Context, state *AppState, c jetstream.Consumer) error {
	logger := slog.Default().With(
		"project", settings.Project,
		"function", settings.Name,
	)
	logger.Info("waiting for messages")
	cc, err := c.Consume(func(msg jetstream.Msg) {
		logger.Info("received event", "subject", msg.Subject())
		msg.Ack()
		parts := strings.Split(msg.Subject(), ".")
		reqID := parts[len(parts)-1]
		logger.Info("request id extracted", "request_id", reqID)
		go func(reqID string, payload []byte) {
			in := make(chan []byte, 1)
			in <- payload
			close(in)
			out := StreamHandler(in)
			if out == nil {
				logger.Error("stream handler returned nil channel", "request_id", reqID)
				return
			}
			for res := range out {
				if err := state.Nc.Publish(fmt.Sprintf("%s.%s.res.go.%s", settings.Project, settings.Name, reqID), res); err != nil {
					logger.Error("failed to publish response", "error", err, "request_id", reqID)
				}
			}
		}(reqID, msg.Data())
	})
	if err != nil {
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
		Durable:       name,
		FilterSubject: subject,
	})
	if err != nil {
		return fmt.Errorf("ERR-CONSUMER-INIT: %v", err)
	}
	err = Consume(ctx, state, consumer)
	if err != nil {
		return fmt.Errorf("ERR-CONSUMER: %v", err)
	}
	return nil
}
