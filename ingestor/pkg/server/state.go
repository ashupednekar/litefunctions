package server

import (
	"fmt"
	"log"
	"net/http"
	"github.com/ashupednekar/litefunctions/ingestor/pkg"
	"github.com/nats-io/nats.go"
)

type Server struct{
	port int
	nc *nats.Conn
}

func NewServer() (*Server, error){
	fmt.Printf("s: %v", pkg.Settings)
	nc, err := nats.Connect(pkg.Settings.NatsBrokerUrl)
  if err != nil{
		return nil, fmt.Errorf("error connecting to broker: %v", err)
	}
	return &Server{pkg.Settings.ListenPort, nc}, nil
}

func (s *Server) Start() error {
	http.HandleFunc("/{project}/{name}/", s.SyncHandler)
	http.HandleFunc("/sse/{project}/{name}/", s.SSEHandler)
	http.HandleFunc("/ws/{project}/{name}/", s.WSHandler)
	log.Printf("listening at %d\n", s.port)
	if err := http.ListenAndServe(fmt.Sprintf(":%d", s.port), nil); err != nil{
		fmt.Printf("error starting server: %v", err)
		return err 
	}
	return nil
}


