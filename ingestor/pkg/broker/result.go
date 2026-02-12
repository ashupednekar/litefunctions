package broker

import (
	"fmt"
	"sync"
	"time"

	"github.com/ashupednekar/litefunctions/ingestor/pkg"
	"github.com/nats-io/nats.go"
)

func Reply(nc *nats.Conn, req *Req) ([]byte, error) {
	subscriber, err := nc.SubscribeSync(
		fmt.Sprintf("%s.%s.res.%s.%s", req.Project, req.Name, req.Lang, req.ReqId),
	)
	if err != nil {
		return nil, fmt.Errorf("error starting subscriber: %s", err)
	}
	timeout, err := time.ParseDuration(pkg.Settings.ReplyTimeout)
	if err != nil {
		return nil, fmt.Errorf("reply timeout improperly configured: %s", err)
	}
	msg, err := subscriber.NextMsg(timeout)
	if err != nil {
		return nil, fmt.Errorf("error returning response: %s", err)
	}
	return msg.Data, nil
}

func Subscribe(nc *nats.Conn, req *Req) (<-chan []byte, func(), error) {
	res := make(chan []byte, 32)
	subject := fmt.Sprintf("%s.%s.res.%s.%s", req.Project, req.Name, req.Lang, req.ReqId)

	subscriber, err := nc.Subscribe(subject, func(msg *nats.Msg) {
		payload := append([]byte(nil), msg.Data...)
		select {
		case res <- payload:
		default:
			// Drop when downstream is slow to avoid blocking NATS callback goroutines.
		}
	})
	if err != nil {
		return nil, nil, fmt.Errorf("error starting subscriber: %s", err)
	}

	var once sync.Once
	cleanup := func() {
		once.Do(func() {
			_ = subscriber.Unsubscribe()
			close(res)
		})
	}

	return res, cleanup, nil
}
