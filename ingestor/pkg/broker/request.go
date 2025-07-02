package broker

import (
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/nats-io/nats.go"
)

type Req struct {
	Project string
	Name    string
	Lang    string
	ReqId   string
}

func NewReq(r *http.Request) (*Req, error) {
	log.Println("creating new request context...")

	lang, err := r.Cookie("lang")
	if err != nil {
		log.Println("'lang' cookie missing, defaulting to 'rust'")
		lang = &http.Cookie{Value: "rs"}
	}

	reqId, err := r.Cookie("reqId")
	if err != nil {
		log.Println("'reqId' cookie missing, generating new UUIDv6...")
		val, err := uuid.NewV6()
		if err != nil {
			return nil, fmt.Errorf("error generating uuid: %s", err)
		}
		reqId = &http.Cookie{Value: val.String()}
	}
	return &Req{
		Project: r.PathValue("project"),
		Name:    r.PathValue("name"),
		Lang:    lang.Value,
		ReqId:   reqId.Value,
	}, nil
}

func Submit(nc *nats.Conn, r *http.Request) (*Req, error) {
	req, err := NewReq(r)
	if err != nil {
		log.Printf("Failed to initialize request: %s\n", err)
		return nil, err
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading request body: %s", err)
	}
	sub := fmt.Sprintf("%s.%s.exec.%s.%s", req.Project, req.Name, req.Lang, req.ReqId)
	if err := nc.Publish(sub, body); err != nil {
		return nil, fmt.Errorf("error submittinng request: %v", err)
	}
	log.Printf("request submitted to broker at: %s\n", sub)
	return req, nil
}

func Produce(nc *nats.Conn, ch chan []byte, w http.ResponseWriter, r *http.Request) (*Req, error) {
	log.Println("upgrading connection to WebSocket...")
	req, err := NewReq(r)
	if err != nil {
		log.Printf("Failed to initialize request: %s\n", err)
		return nil, err
	}
	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}
	conn, err := upgrader.Upgrade(w, r, r.Header)
	if err != nil {
		log.Printf("webSocket upgrade failed: %s\n", err)
		return nil, fmt.Errorf("error upgrading connection: %s", err)
	}
	log.Printf("webSocket connection established for reqId=%s\n", req.ReqId)
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			log.Printf("webSocket read closed: %s\n", err)
			break
		}
		log.Printf("received message over WebSocket (len=%d)\n", len(msg))
		ch <- msg
	}
	log.Printf("webSocket connection closed for reqId=%s\n", req.ReqId)
	return req, nil
}
