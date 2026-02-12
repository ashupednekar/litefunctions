package pkg

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/nats-io/nats.go"
)

func Consume(ctx context.Context, state *AppState, subject string) error {
	logger := slog.Default().With(
		"project", settings.Project,
		"function", settings.Name,
	)
	logger.Info("waiting for messages")
	sub, err := state.Nc.Subscribe(subject, func(msg *nats.Msg) {
		logger.Info("received event", "subject", msg.Subject)
		parts := strings.Split(msg.Subject, ".")
		reqID := parts[len(parts)-1]
		logger.Info("request id extracted", "request_id", reqID)
		payload := append([]byte(nil), msg.Data...)
		go handleMessage(state, logger, reqID, payload)
	})
	if err != nil {
		return err
	}
	defer sub.Unsubscribe()

	select {
	case <-ctx.Done():
		return ctx.Err()
	}
}

func handleMessage(state *AppState, logger *slog.Logger, reqID string, payload []byte) {
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
}

func StartFunction(ctx context.Context, state *AppState) error {
	settings := LoadSettings()
	logger := slog.Default().With(
		"project", settings.Project,
		"function", settings.Name,
	)
	name := fmt.Sprintf("%s-%s", settings.Project, settings.Name)
	subject := fmt.Sprintf("%s.%s.exec.go.*", settings.Project, settings.Name)
	logger.Info("starting consumer", "subject", subject, "name", name)
	err := Consume(ctx, state, subject)
	if err != nil {
		return fmt.Errorf("ERR-CONSUMER: %v", err)
	}
	return nil
}
