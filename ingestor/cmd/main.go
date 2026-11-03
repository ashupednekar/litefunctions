package main

import (
	"log/slog"

	settings "github.com/ashupednekar/litefunctions/ingestor/internal"
	"github.com/ashupednekar/litefunctions/ingestor/internal/server"
	"github.com/nats-io/nats.go"
)

func main() {
	settings.LoadSettings()

	nc, err := nats.Connect(settings.Settings.NatsUrl)
	if err != nil {
		slog.Error("error connecting to broker", "error", err)
		return
	}

	s, err := server.NewServer(nc)
	if err != nil {
		slog.Error("error creating server", "error", err)
		return
	}
	slog.Info("ingestor starting", "settings", settings.Settings)

	if err := s.Start(); err != nil {
		slog.Error("error starting server", "error", err)
		return
	}
}
