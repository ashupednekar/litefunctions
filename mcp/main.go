package main

import (
	"log"
	"net/http"
	"time"

	vcs "github.com/ashupednekar/litefunctions/common/vcs/pkg"
	"go-simpler.org/env"
)

// Settings controls the MCP HTTP server and the operator hop used after a
// function is written.
type Settings struct {
	ListenAddr   string `env:"MCP_LISTEN_ADDR" default:":8090"`
	Path         string `env:"MCP_PATH" default:"/mcp"`
	OperatorAddr string `env:"OPERATOR_URL" default:"litefunctions-operator:50051"`
	Namespace    string `env:"POD_NAMESPACE" default:"litefunctions"`
	Token        string `env:"MCP_TOKEN"`
}

func main() {
	var settings Settings
	if err := env.Load(&settings, nil); err != nil {
		log.Fatalf("load MCP settings: %v", err)
	}

	vcsCfg, err := vcs.LoadCfg()
	if err != nil {
		log.Fatalf("load VCS settings: %v", err)
	}

	server := NewServer(settings, *vcsCfg)
	httpServer := &http.Server{
		Addr:              settings.ListenAddr,
		Handler:           server,
		ReadHeaderTimeout: 10 * time.Second,
	}

	log.Printf("litefunctions MCP server listening on %s%s", settings.ListenAddr, settings.Path)
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("MCP server stopped: %v", err)
	}
}
