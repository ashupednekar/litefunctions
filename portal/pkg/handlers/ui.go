package handlers

import (
	"context"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"

	accessAdaptors "github.com/ashupednekar/litefunctions/portal/internal/access/adaptors"
	authAdaptors "github.com/ashupednekar/litefunctions/portal/internal/auth/adaptors"
	endpointAdaptors "github.com/ashupednekar/litefunctions/portal/internal/endpoint/adaptors"
	functionAdaptors "github.com/ashupednekar/litefunctions/portal/internal/function/adaptors"
	"github.com/ashupednekar/litefunctions/portal/pkg/state"
	"github.com/ashupednekar/litefunctions/portal/templates"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
)

type UIHandlers struct {
	state *state.AppState
}

func NewUIHandlers(s *state.AppState) *UIHandlers {
	return &UIHandlers{state: s}
}

func (h *UIHandlers) Home(ctx *gin.Context) {
	userId, ok := ctx.Get("userID")
	if ok && userId != nil {
		ctx.Redirect(http.StatusFound, "/dashboard")
		return
	}
	page := templates.BaseLayout(
		templates.HomeContent(),
	)

	if err := page.Render(ctx, ctx.Writer); err != nil {
		slog.Error("page render failed", "error", err)
	}
}

func (h *UIHandlers) Dashboard(ctx *gin.Context) {
	userID, _ := ctx.Get("userID")
	activeProjectID, _ := ctx.Cookie("lws_project")

	var projects []templates.Project
	usernameStr := "User"

	if userID != nil {
		q := accessAdaptors.New(h.state.DBPool)
		dbProjects, err := q.ListProjectsForUser(ctx.Request.Context(), userID.([]byte))
		if err == nil {
			for _, p := range dbProjects {
				projects = append(projects, templates.Project{
					ID:   hex.EncodeToString(p.ID.Bytes[:]),
					Name: p.Name,
					Role: string(p.Role),
				})
			}
		}

		authQ := authAdaptors.New(h.state.DBPool)
		user, err := authQ.GetUserByID(context.Background(), userID.([]byte))
		if err == nil {
			usernameStr = user.Name
		}
	}

	page := templates.BaseLayout(
		templates.DashboardContent(usernameStr, projects, activeProjectID),
	)

	_ = page.Render(ctx, ctx.Writer)
}

func (h *UIHandlers) Functions(ctx *gin.Context) {
	userID, _ := ctx.MustGet("userID").([]byte)
	activeProjectID, _ := ctx.Cookie("lws_project")

	var functions []templates.Function
	usernameStr := ctx.MustGet("userName").(string)

	q := functionAdaptors.New(h.state.DBPool)

	projUUID := ctx.MustGet("projectUUID").(pgtype.UUID)
	if dbFns, err := q.ListFunctionsForProject(ctx.Request.Context(), projUUID); err == nil {
		for _, f := range dbFns {
			lang := f.Language
			icon := fmt.Sprintf("/static/imgs/%s-svgrepo-com.svg", lang)

			functions = append(functions, templates.Function{
				ID:       hex.EncodeToString(f.ID.Bytes[:]),
				Name:     f.Name,
				Language: lang,
				Icon:     icon,
			})
		}
	}

	if userID != nil {
		authQ := authAdaptors.New(h.state.DBPool)
		if user, err := authQ.GetUserByID(context.Background(), userID); err == nil {
			usernameStr = user.Name
		}
	}

	langs := []templates.Lang{
		{ID: "rust", Icon: fmt.Sprintf("/static/imgs/%s-svgrepo-com.svg", "rust"), Label: "Rust", AceMode: "rust"},
		{ID: "go", Icon: fmt.Sprintf("/static/imgs/%s-svgrepo-com.svg", "go"), Label: "Go", AceMode: "golang"},
		{ID: "python", Icon: fmt.Sprintf("/static/imgs/%s-svgrepo-com.svg", "python"), Label: "Python", AceMode: "python"},
		{ID: "javascript", Icon: fmt.Sprintf("/static/imgs/%s-svgrepo-com.svg", "javascript"), Label: "JavaScript", AceMode: "javascript"},
		{ID: "lua", Icon: fmt.Sprintf("/static/imgs/%s-svgrepo-com.svg", "lua"), Label: "Lua", AceMode: "lua"},
	}

	page := templates.BaseLayout(
		templates.FunctionContent(functions, langs, activeProjectID, usernameStr),
	)

	if err := page.Render(ctx, ctx.Writer); err != nil {
		slog.Error("page render failed", "error", err)
	}
}

func (h *UIHandlers) Data(ctx *gin.Context) {
	page := templates.BaseLayout(
		templates.DataContent(),
	)

	if err := page.Render(ctx, ctx.Writer); err != nil {
		slog.Error("page render failed", "error", err)
	}
}

func (h *UIHandlers) Endpoints(ctx *gin.Context) {
	projUUID := ctx.MustGet("projectUUID").(pgtype.UUID)
	q := endpointAdaptors.New(h.state.DBPool)

	var endpoints []templates.Endpoint
	dbEps, err := q.ListEndpointsForProject(ctx.Request.Context(), projUUID)
	if err == nil {
		for _, e := range dbEps {
			// e.ID is available, e.Name is available, e.Method is available, e.Scope is available, e.FunctionName is available from join
			endpoints = append(endpoints, templates.Endpoint{
				ID:           hex.EncodeToString(e.ID.Bytes[:]),
				Name:         e.Name,
				Method:       e.Method,
				Scope:        e.Scope,
				FunctionName: e.FunctionName,
			})
		}
	} else {
		slog.Error("failed to list endpoints", "project", projUUID, "error", err)
	}

	page := templates.BaseLayout(
		templates.EndpointsContent(endpoints),
	)

	if err := page.Render(ctx, ctx.Writer); err != nil {
		slog.Error("page render failed", "error", err)
	}
}

func (h *UIHandlers) Configuration(ctx *gin.Context) {
	page := templates.BaseLayout(
		templates.ConfigurationContent(),
	)

	if err := page.Render(ctx, ctx.Writer); err != nil {
		slog.Error("page render failed", "error", err)
	}
}
