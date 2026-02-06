package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/ashupednekar/litefunctions/runtimes/go/pkg"
)

func main() {
	settings := pkg.LoadSettings()
	ctx := context.Background()
	state, err := pkg.NewAppState(ctx)
	if err != nil {
		log.Printf("error starting function: %v", err)
		return
	}

	go startHTTPServer(state, settings)

	if err := pkg.StartFunction(ctx, state); err != nil {
		log.Printf("error starting function: %v", err)
	}
}

func startHTTPServer(state *pkg.AppState, settings *pkg.Settings) {
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
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Printf("http server error: %v", err)
	}
}
