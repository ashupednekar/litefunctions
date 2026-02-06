package main

import (
	"context"
	"net/http"
	"log/slog"
	"time"

	"github.com/ashupednekar/litefunctions/runtimes/go/pkg"
)

func main() {
	settings := pkg.LoadSettings()
	logger := slog.Default().With(
		"project", settings.Project,
		"function", settings.Name,
	)
	ctx := context.Background()
	state, err := pkg.NewAppState(ctx)
	if err != nil {
		logger.Error("failed to initialize runtime state", "error", err)
		return
	}

	go startHTTPServer(state, settings, logger)

	if err := pkg.StartFunction(ctx, state); err != nil {
		logger.Error("function consumer exited", "error", err)
	}
}

func startHTTPServer(state *pkg.AppState, settings *pkg.Settings, logger *slog.Logger) {
	port := settings.HttpPort
	if port == "" {
		port = "8080"
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", pkg.Handle)

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	logger.Info("starting http server", "addr", server.Addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("http server error", "error", err)
	}
}
