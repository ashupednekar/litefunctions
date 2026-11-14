package function

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/ashupednekar/litefunctions/common"
	"github.com/ashupednekar/litefunctions/common/events"
	accessadaptors "github.com/ashupednekar/litefunctions/portal/internal/access/adaptors"
	"github.com/ashupednekar/litefunctions/portal/internal/auth"
	authadaptors "github.com/ashupednekar/litefunctions/portal/internal/auth/adaptors"
	endpointadaptors "github.com/ashupednekar/litefunctions/portal/internal/endpoint/adaptors"
	functionadaptors "github.com/ashupednekar/litefunctions/portal/internal/function/adaptors"
	projectadaptors "github.com/ashupednekar/litefunctions/portal/internal/project/adaptors"
	"github.com/ashupednekar/litefunctions/portal/internal/state"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nats-io/nats.go"
)

const bootstrapUserName = auth.SystemUserName

var bootstrapUserID = auth.SystemUserID

func Consume(s *state.AppState) error {
	slog.Info("waiting for messages")
	ctx := context.Background() // TODO: consider timeouts
	_, err := s.Broker.Subscribe("lf.bootstrap.*", func(msg *nats.Msg) {
		slog.Info("received bootstrap event", "subject", msg.Subject)
		var event events.FunctionBootstrap
		if err := json.Unmarshal(msg.Data, &event); err != nil {
			slog.Error("failed to unmarshal bootstrap event", "error", err)
			return
		}
		if err := applyBootstrapEvent(ctx, s, event); err != nil {
			slog.Error("failed to apply bootstrap event", "project", event.Project, "function", event.Name, "error", err)
			return
		}
		slog.Info("bootstrapped project function", "project", event.Project, "function", event.Name)
	})
	if err != nil {
		return err
	}
	// The subscription is owned by the long-lived NATS connection in AppState.
	// Unsubscribing here would tear it down as soon as this function returns.
	return nil
}

func applyBootstrapEvent(ctx context.Context, s *state.AppState, event events.FunctionBootstrap) error {
	if common.LangExt[event.Language] == "" {
		return fmt.Errorf("invalid language %q", event.Language)
	}

	// CR bootstrap carries no trusted human identity. Keep operator-created
	// projects system-owned until ownership claiming is designed (issue #94).
	users := authadaptors.New(s.DBPool)
	if err := users.CreateUser(ctx, authadaptors.CreateUserParams{
		ID:          bootstrapUserID,
		Name:        bootstrapUserName,
		DisplayName: "LiteFunctions",
	}); err != nil {
		return fmt.Errorf("ensure bootstrap user: %w", err)
	}

	projects := projectadaptors.New(s.DBPool)
	project, err := projects.GetProjectByName(ctx, event.Project)
	if errors.Is(err, pgx.ErrNoRows) {
		project, err = projects.CreateProject(ctx, projectadaptors.CreateProjectParams{
			Name:        event.Project,
			Description: pgtype.Text{String: "Created by LiteFunctions operator", Valid: true},
			CreatedBy:   bootstrapUserID,
		})
		if err != nil {
			return fmt.Errorf("create project %s: %w", event.Project, err)
		}
		if err := accessadaptors.New(s.DBPool).AddProjectOwner(ctx, accessadaptors.AddProjectOwnerParams{
			UserID:    bootstrapUserID,
			ProjectID: project.ID,
		}); err != nil {
			return fmt.Errorf("assign project %s owner: %w", event.Project, err)
		}
	} else if err != nil {
		return fmt.Errorf("retrieve project %s: %w", event.Project, err)
	}

	functions := functionadaptors.New(s.DBPool)
	existingFunctions, err := functions.ListFunctionsForProject(ctx, project.ID)
	if err != nil {
		return fmt.Errorf("list functions for project %s: %w", event.Project, err)
	}

	var fn functionadaptors.Function
	found := false
	for _, existing := range existingFunctions {
		if existing.Name == event.Name {
			fn = existing
			found = true
			break
		}
	}
	if !found {
		fn, err = functions.CreateFunction(ctx, functionadaptors.CreateFunctionParams{
			ProjectID: project.ID,
			Name:      event.Name,
			Language:  event.Language,
			Path:      event.Path,
			CreatedBy: bootstrapUserID,
		})
		if err != nil {
			return fmt.Errorf("create function %s: %w", event.Name, err)
		}
	}

	endpoints := endpointadaptors.New(s.DBPool)
	existingEndpoints, err := endpoints.ListEndpointsForProject(ctx, project.ID)
	if err != nil {
		return fmt.Errorf("list endpoints for project %s: %w", event.Project, err)
	}
	endpointName := fmt.Sprintf("/%s/%s", event.Project, event.Name)
	for _, endpoint := range existingEndpoints {
		if endpoint.FunctionID == fn.ID && endpoint.Method == "GET" && endpoint.Name == endpointName {
			return nil
		}
	}

	if _, err := endpoints.CreateEndpoint(ctx, endpointadaptors.CreateEndpointParams{
		ProjectID:  project.ID,
		Name:       endpointName,
		Method:     "GET",
		Scope:      "public",
		FunctionID: fn.ID,
	}); err != nil {
		return fmt.Errorf("create endpoint for function %s: %w", event.Name, err)
	}
	return nil
}
