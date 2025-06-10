package pkg

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/nats-io/nats.go/jetstream"
)

func consume(state *AppState, c jetstream.Consumer) error {
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
	defer cc.Stop()
	if err != nil{
		return err
	}
	return nil
}

func start_function() error {
	ctx := context.Background()
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
	err = consume(state, consumer)
	if err != nil{
		return fmt.Errorf("ERR-CONSUMER: %v", err)
	}
	return nil
}
