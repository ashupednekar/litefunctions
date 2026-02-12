package broker

import (
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strings"

	"github.com/gorilla/websocket"
	"github.com/nats-io/nats.go"
)

type Req struct {
	Project string
	Name    string
	Lang    string
	ReqId   string
}

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func Submit(nc *nats.Conn, r *http.Request, lang string) (*Req, error) {
	project, name := parsePath(r.URL.Path)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading request body: %s", err)
	}
	reqID := randString(8)
	if err := nc.Publish(
		fmt.Sprintf("%s.%s.exec.%s.%s", project, name, lang, reqID),
		body,
	); err != nil {
		return nil, fmt.Errorf("error submitting request: %v", err)
	}
	return &Req{Project: project, Name: name, Lang: lang, ReqId: reqID}, nil
}

func Produce(nc *nats.Conn, w http.ResponseWriter, r *http.Request, lang string) (*websocket.Conn, *Req, error) {
	project, name := parsePath(r.URL.Path)
	reqID := randString(8)
	req := Req{Project: project, Name: name, Lang: lang, ReqId: reqID}
	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}
	// Do not echo request headers into the response. Gorilla rejects
	// application-specific Sec-WebSocket-Extensions headers.
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("error upgrading connection: %s", err)
	}
	go func() {
		defer conn.Close()
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			if err := nc.Publish(
				fmt.Sprintf("%s.%s.exec.%s.%s", project, name, lang, reqID),
				msg,
			); err != nil {
				return
			}
		}
	}()
	return conn, &req, nil
}

func parsePath(path string) (string, string) {
	trimmed := strings.Trim(path, "/")
	if trimmed == "" {
		return "", ""
	}

	parts := strings.Split(trimmed, "/")
	if len(parts) < 3 || parts[0] != "lambda" {
		return "", ""
	}

	idx := 1
	if parts[1] == "sse" || parts[1] == "ws" {
		idx = 2
	}

	if len(parts) <= idx+1 {
		return "", ""
	}

	return parts[idx], parts[idx+1]
}

func randString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}
