package main

import (
	"log"

	"github.com/ashupednekar/litefunctions/ingestor/pkg"
	"github.com/ashupednekar/litefunctions/ingestor/pkg/server"
)

func main() {
	pkg.LoadSettings()
	s, err := server.NewServer()
	if err != nil {
		log.Fatalf("error creating server: %v", err)
	}
	s.Start()
}
