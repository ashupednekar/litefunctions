package main

import (
	"fmt"
	"io"
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

func (server *Server) Start() error {
	http.HandleFunc("/{lang}/{project}/{name}", server.Submit)
	log.Printf("listening at %d", server.port)
	if err := http.ListenAndServe(fmt.Sprintf(":%d", server.port), nil); err != nil{
		fmt.Printf("error starting server: %v", err)
		return err 
	}
	return nil
}

func (server *Server) Submit(w http.ResponseWriter, r *http.Request){
	project := r.PathValue("project")
	name := r.PathValue("name")
	lang := "rust"
	reqId := "jnkldmlv"
	body, err := io.ReadAll(r.Body)
	if err != nil{
		http.Error(w, fmt.Sprintf("error reading request body: %s", err), http.StatusBadRequest)
	}
	server.nc.Publish(
		fmt.Sprintf("%s.%s.exec.%s.%s", project, name, lang, reqId),
		body,
	)
	subscriber, err := server.nc.SubscribeSync(
		fmt.Sprintf("%s.%s.res.%s.%s", project, name, lang, reqId),
	)
	if err != nil{
		http.Error(w, "error reading response", http.StatusInternalServerError)
	}
}

func main(){
	server, err := NewServer()
	if err != nil{
		log.Fatalf("error creating server: %v", err)
	}
	server.Start()
}
