package main

import (
	"log/slog"

	"github.com/ashupednekar/litefunctions/ingestor/pkg"
	"github.com/ashupednekar/litefunctions/ingestor/pkg/server"
	"github.com/nats-io/nats.go"
)

func main() {
	pkg.LoadSettings()

	nc, err := nats.Connect(pkg.Settings.NatsUrl)
	if err != nil {
		slog.Error("error connecting to broker", "error", err)
		return
	}

	s, err := server.NewServer(nc)
	if err != nil {
		slog.Error("error creating server", "error", err)
		return
	}
	slog.Info("ingestor starting", "settings", pkg.Settings)

	if err := s.Start(); err != nil {
		slog.Error("error starting server", "error", err)
		return
	}
}
