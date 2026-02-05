package pkg

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/nats-io/nats.go/jetstream"
)

func Consume(ctx context.Context, state *AppState, c jetstream.Consumer) error {
	log.Printf("waiting for messages...")
	cc, err := c.Consume(func(msg jetstream.Msg){
		log.Printf("received event")
		msg.Ack()
		parts := strings.Split(msg.Subject(), ".")
		req_id := parts[len(parts)-1]
		log.Printf("request id: %s", req_id)
		res, err := Handler(state, &req_id)
		if err != nil{
			log.Printf("ERR-FUNCTION: %v", err)
		}
		state.Nc.Publish(fmt.Sprintf("%s.%s.res.go.%s", settings.Project, settings.Name, req_id), res)
	})
	if err != nil{
		return err
	}
	defer cc.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-cc.Closed():
		return fmt.Errorf("ERR-CONSUMER-CLOSED")
	}
}

func StartFunction() error {
	ctx := context.Background()
	settings := LoadSettings()
	state, err := NewAppState(ctx)
	if err != nil{
		return fmt.Errorf("ERR-STATE-INIT: %v", err)
	}
	name := fmt.Sprintf("%s-%s", settings.Project, settings.Name)
	subject := fmt.Sprintf("%s.%s.exec.go.*", settings.Project, settings.Name)
	log.Printf("starting consumer listening to subject: %s", subject)
	consumer, err := state.Js.CreateOrUpdateConsumer(ctx, jetstream.ConsumerConfig{
		Durable: name,
		FilterSubject: subject,
	})
	if err != nil{
		return fmt.Errorf("ERR-CONSUMER-INIT: %v", err)
	}
	err = Consume(ctx, state, consumer)
	if err != nil{
		return fmt.Errorf("ERR-CONSUMER: %v", err)
	}
	return nil
}
