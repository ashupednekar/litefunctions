package function

import (
	"log/slog"
	"strings"

	"github.com/nats-io/nats.go"
)

func Consume(nc *nats.Conn) error {
	slog.Info("waiting for messages")
	sub, err := nc.Subscribe("lf.bootstrap.*.*", func(msg *nats.Msg) {
		parts := strings.Split(msg.Subject, ".")
		project, function := parts[2], parts[3]
		slog.Info("recieved bootstrap event", "project", project, "function", function)

	})
	if err != nil {
		return err
	}
	defer sub.Unsubscribe()
	return nil
}
