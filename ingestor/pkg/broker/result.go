package broker 

import (
	"fmt"
	"os"
	"time"

	"github.com/nats-io/nats.go"
)

func Reply(nc *nats.Conn, req *Req) ([]byte, error) {
	subscriber, err := nc.SubscribeSync(
		fmt.Sprintf("%s.%s.res.%s.%s", req.Project, req.Name, req.Lang, req.ReqId),
	)
	if err != nil{
		return nil, fmt.Errorf("error starting subscriber: %s", err)
	}
	timeout, err := time.ParseDuration(os.Getenv("REPLY_TIMEOUT"))
	if err != nil{
		return nil, fmt.Errorf("reply timeout improperly configured: %s", err)
	}
	msg, err := subscriber.NextMsg(timeout)
	if err != nil{
		return nil, fmt.Errorf("error returning response: %s", err)
	}
	return msg.Data, nil
}

func Subscribe(nc *nats.Conn, req *Req) (chan []byte, error){
	ch := make(chan *nats.Msg)
	res := make(chan []byte)
	subscriber, err := nc.ChanSubscribe(
		fmt.Sprintf("%s.%s.res.%s.%s", req.Project, req.Name, req.Lang, req.ReqId),
		ch,
	)
	if err != nil{
		return nil, fmt.Errorf("error starting subscriber: %s", err)
	}
	defer subscriber.Unsubscribe()
	for msg := range ch{
		res <- msg.Data
	}
	return res, nil
}
