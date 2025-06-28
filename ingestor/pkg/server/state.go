package server

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/nats-io/nats.go"
)

type Server struct{
	port int
	nc *nats.Conn
}

func NewServer() (*Server, error){
	nc, err := nats.Connect(os.Getenv("NATS_BROKER_URL"))
  if err != nil{
		return nil, fmt.Errorf("error connecting to broker: %v", err)
	}
	port, err := strconv.Atoi(os.Getenv("LISTEN_PORT"))
	if err != nil{
		log.Printf("invalid port env, defaulting to 3000")
		port = 3000
	}
	return &Server{port, nc}, nil
}

func (s *Server) Start() error {
	http.HandleFunc("/{project}/{name}", s.SyncHandler)
	http.HandleFunc("/sse/{project}/{name}", s.SSEHandler)
	http.HandleFunc("/ws/{project}/{name}", s.WSHandler)
	log.Printf("listening at %d", s.port)
	if err := http.ListenAndServe(fmt.Sprintf(":%d", s.port), nil); err != nil{
		fmt.Printf("error starting server: %v", err)
		return err 
	}
	return nil
}


