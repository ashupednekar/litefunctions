package main

import (
	"github.com/ashupednekar/litefunctions/ingestor/pkg/server"
	"log"
)

func main() {
	s, err := server.NewServer()
	if err != nil {
		log.Fatalf("error creating server: %v", err)
	}
	s.Start()
}
