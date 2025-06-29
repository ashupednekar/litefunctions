package broker 

import (
	"fmt"
	"io"
	"net/http"

	"github.com/nats-io/nats.go"
	"github.com/gorilla/websocket"
)

type Req struct{
	Project string
	Name string
	Lang string
	ReqId string
}

func Submit(nc *nats.Conn, r *http.Request) (*Req, error) {
	req := Req{r.PathValue("project"), r.PathValue("project"), "rust", ""}
	body, err := io.ReadAll(r.Body)
	if err != nil{
		return nil, fmt.Errorf("error reading request body: %s", err)
	}
	if err := nc.Publish(
		fmt.Sprintf("%s.%s.exec.%s.%s", req.Project, req.Name, req.Lang, req.ReqId),
		body,
	); err != nil{
		return nil, fmt.Errorf("error submittinng request: %v", err)
	}
	return &req, nil
}

func Produce(nc *nats.Conn, ch chan []byte, w http.ResponseWriter, r *http.Request) (*Req, error){
	req := Req{r.PathValue("project"), r.PathValue("project"), "rust", ""}
	upgrader := websocket.Upgrader{
		ReadBufferSize: 1024,
		WriteBufferSize: 1024,
	}
	conn, err := upgrader.Upgrade(w, r, r.Header)
	if err != nil{
		return nil, fmt.Errorf("error upgrading connection: %s", err)
	}
	for{
		_, msg, err := conn.ReadMessage()
		if err != nil{
			break
		}
		ch <- msg
	}
	return &req, nil
}
