package function

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/ashupednekar/litefunctions/common"
	"github.com/ashupednekar/litefunctions/common/events"
	endpointadaptors "github.com/ashupednekar/litefunctions/portal/internal/endpoint/adaptors"
	functionadaptors "github.com/ashupednekar/litefunctions/portal/internal/function/adaptors"
	projectadaptors "github.com/ashupednekar/litefunctions/portal/internal/project/adaptors"
	"github.com/ashupednekar/litefunctions/portal/internal/state"
	"github.com/nats-io/nats.go"
)

func Consume(s *state.AppState) error {
	slog.Info("waiting for messages")
	ctx := context.Background() // TODO: consider timeouts
	p := projectadaptors.New(s.DBPool)
	q := functionadaptors.New(s.DBPool)
	eq := endpointadaptors.New(s.DBPool)
	sub, err := s.Broker.Subscribe("lf.bootstrap.*.*", func(msg *nats.Msg) {
		slog.Info("recieved bootstrap event", "subject", msg.Subject)
		var event events.FunctionBootstrap
		err := json.Unmarshal(msg.Data, &event)
		if err != nil {
			slog.Error("failed to unmarshal event, skipping")
			return
		}
		project, err := p.GetProjectByName(ctx, event.Project) //TODO: cache
		if err != nil {
			slog.Error("failed to retrieve project")
			return
		}
		ext := common.LangExt[event.Language]
		if ext == "" {
			slog.Error("invalid language")
			return
		}
		fn, err := q.CreateFunction(ctx, functionadaptors.CreateFunctionParams{
			ProjectID: project.ID,
			Name:      event.Name,
			Language:  event.Language,
			Path:      event.Path,
		})
		if err != nil {
			slog.Error("failed to create function")
			return
		}
		_, err = eq.CreateEndpoint(ctx, endpointadaptors.CreateEndpointParams{
			ProjectID:  fn.ProjectID,
			Name:       fn.Name,
			Method:     "GET",
			Scope:      "public",
			FunctionID: fn.ID,
		})
		if err != nil {
			slog.Error("failed to create endpoint")
			return
		}
	})
	if err != nil {
		return err
	}
	defer sub.Unsubscribe()
	return nil
}
