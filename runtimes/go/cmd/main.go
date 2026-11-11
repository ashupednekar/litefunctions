package main

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	runtime "runtimes/go/internal"
)

func main() {
	settings := runtime.LoadSettings()
	logger := slog.Default().With(
		"project", settings.Project,
		"function", settings.Name,
	)
	ctx := context.Background()
	state, err := runtime.NewAppState(ctx)
	if err != nil {
		logger.Error("failed to initialize runtime state", "error", err)
		return
	}

	go startHTTPServer(state, settings, logger)

	if err := runtime.StartFunction(ctx, state); err != nil {
		logger.Error("function consumer exited", "error", err)
	}
}

func startHTTPServer(state *runtime.AppState, settings *runtime.Settings, logger *slog.Logger) {
	port := settings.HttpPort
	if port == "" {
		port = "8080"
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", runtime.Handle)

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
