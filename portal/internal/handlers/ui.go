package handlers

import (
	"context"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/ashupednekar/litefunctions/common/sample"
	accessAdaptors "github.com/ashupednekar/litefunctions/portal/internal/access/adaptors"
	"github.com/ashupednekar/litefunctions/portal/internal/auth"
	authAdaptors "github.com/ashupednekar/litefunctions/portal/internal/auth/adaptors"
	"github.com/ashupednekar/litefunctions/portal/internal/config"
	endpointAdaptors "github.com/ashupednekar/litefunctions/portal/internal/endpoint/adaptors"
	functionAdaptors "github.com/ashupednekar/litefunctions/portal/internal/function/adaptors"
	"github.com/ashupednekar/litefunctions/portal/internal/state"
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

// shellData builds the persistent application chrome for a page.
func shellData(ctx *gin.Context, title string, section string, active string) templates.ShellData {
	username := "User"
	if v, ok := ctx.Get("userName"); ok {
		if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
			username = s
		}
	}
	projectName := ""
	if v, ok := ctx.Get("projectName"); ok {
		if s, ok := v.(string); ok {
			projectName = s
		}
	}
	projectID, _ := ctx.Cookie("lws_project")
	role := ctx.GetString("projectRole")

	return templates.ShellData{
		Title:       title,
		Section:     section,
		Active:      active,
		Username:    username,
		Role:        role,
		ProjectID:   projectID,
		ProjectName: projectName,
		Dummy:       templates.PageUsesDummyData(active),
	}
}

func (h *UIHandlers) Home(ctx *gin.Context) {
	userId, ok := ctx.Get("userID")
	if ok && userId != nil {
		ctx.Redirect(http.StatusFound, "/dashboard")
		return
	}
	page := templates.AuthLayout(
		templates.HomeContent(config.Cfg.AdminLoginEnabled),
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
		dbProjects, err := q.ListProjectsForUserOrAdmin(
			ctx.Request.Context(),
			userID.([]byte),
			auth.IsSystemUserID(userID.([]byte)),
		)
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

	data := templates.DashboardData{
		Username:        usernameStr,
		Projects:        projects,
		ActiveProjectID: activeProjectID,
	}

	// Resolve the active project's display name, role and counts.
	for _, p := range projects {
		if p.ID != activeProjectID {
			continue
		}
		data.ActiveProject = p.Name
		data.ActiveRole = p.Role
		break
	}

	if data.ActiveProject != "" {
		if idBytes, err := hex.DecodeString(activeProjectID); err == nil && len(idBytes) == 16 {
			var projectUUID pgtype.UUID
			copy(projectUUID.Bytes[:], idBytes)
			projectUUID.Valid = true

			fnQ := functionAdaptors.New(h.state.DBPool)
			if fns, err := fnQ.ListFunctionsForProject(ctx.Request.Context(), projectUUID); err == nil {
				data.FunctionCount = len(fns)
			}
			epQ := endpointAdaptors.New(h.state.DBPool)
			if eps, err := epQ.ListEndpointsForProject(ctx.Request.Context(), projectUUID); err == nil {
				data.EndpointCount = len(eps)
			}
		}
	}

	shell := templates.ShellData{
		Title:       "Overview",
		Section:     "Workspace",
		Active:      "overview",
		Username:    usernameStr,
		Role:        data.ActiveRole,
		ProjectID:   activeProjectID,
		ProjectName: data.ActiveProject,
		Dummy:       true,
	}
	if shell.Role == "" {
		shell.Role = "owner"
	}

	page := templates.BaseLayout(
		shell,
		templates.DashboardContent(data),
	)

	_ = page.Render(ctx, ctx.Writer)
}

func (h *UIHandlers) Functions(ctx *gin.Context) {
	userID, _ := ctx.MustGet("userID").([]byte)
	activeProjectID, _ := ctx.Cookie("lws_project")

	var functions []templates.Function
	usernameStr := ctx.MustGet("userName").(string)

	q := functionAdaptors.New(h.state.DBPool)
	epQ := endpointAdaptors.New(h.state.DBPool)

	projUUID := ctx.MustGet("projectUUID").(pgtype.UUID)
	endpointByFn := map[string]string{}
	if eps, err := epQ.ListEndpointsForProject(ctx.Request.Context(), projUUID); err == nil {
		for _, ep := range eps {
			fnID := hex.EncodeToString(ep.FunctionID.Bytes[:])
			if _, exists := endpointByFn[fnID]; exists {
				continue
			}
			endpointByFn[fnID] = hex.EncodeToString(ep.ID.Bytes[:])
		}
	}

	if dbFns, err := q.ListFunctionsForProject(ctx.Request.Context(), projUUID); err == nil {
		for _, f := range dbFns {
			lang := f.Language
			icon := fmt.Sprintf("/static/imgs/%s-svgrepo-com.svg", lang)
			fnID := hex.EncodeToString(f.ID.Bytes[:])

			functions = append(functions, templates.Function{
				ID:         fnID,
				Name:       f.Name,
				Language:   lang,
				Icon:       icon,
				EndpointID: endpointByFn[fnID],
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
		{ID: "ts", Icon: fmt.Sprintf("/static/imgs/%s-svgrepo-com.svg", "ts"), Label: "TypeScript", AceMode: "typescript"},
		{ID: "lua", Icon: fmt.Sprintf("/static/imgs/%s-svgrepo-com.svg", "lua"), Label: "Lua", AceMode: "lua"},
	}

	samples := map[string]templates.CodeSample{}
	for _, lang := range langs {
		syncCode, syncErr := sample.LoadSync(lang.ID)
		asyncCode, asyncErr := sample.LoadAsync(lang.ID)
		if syncErr != nil || asyncErr != nil {
			slog.Error("failed to load function samples", "language", lang.ID, "sync_error", syncErr, "async_error", asyncErr)
			continue
		}
		samples[lang.ID] = templates.CodeSample{
			Sync:  string(syncCode),
			Async: string(asyncCode),
		}
	}

	page := templates.BaseLayout(
		shellData(ctx, "Functions", "Workspace", "functions"),
		templates.FunctionContent(functions, langs, activeProjectID, usernameStr, samples),
	)

	if err := page.Render(ctx, ctx.Writer); err != nil {
		slog.Error("page render failed", "error", err)
	}
}

func (h *UIHandlers) Data(ctx *gin.Context) {
	page := templates.BaseLayout(
		shellData(ctx, "Objects", "Data", "data"),
		templates.DataContent(),
	)

	if err := page.Render(ctx, ctx.Writer); err != nil {
		slog.Error("page render failed", "error", err)
	}
}

func (h *UIHandlers) loadEndpoints(ctx *gin.Context) []templates.Endpoint {
	projUUID := ctx.MustGet("projectUUID").(pgtype.UUID)
	q := endpointAdaptors.New(h.state.DBPool)

	var endpoints []templates.Endpoint
	dbEps, err := q.ListEndpointsForProject(ctx.Request.Context(), projUUID)
	if err == nil {
		baseURL := strings.TrimRight(config.Cfg.IngestorUrl, "/")
		for _, e := range dbEps {
			endpoints = append(endpoints, templates.Endpoint{
				ID:           hex.EncodeToString(e.ID.Bytes[:]),
				Name:         e.Name,
				Method:       e.Method,
				Scope:        e.Scope,
				FunctionName: e.FunctionName,
				Language:     e.FunctionLanguage,
				URL:          baseURL + e.Name,
				IsAsync:      e.IsAsync,
			})
		}
	} else {
		slog.Error("failed to list endpoints", "project", projUUID, "error", err)
	}
	return endpoints
}

func (h *UIHandlers) Endpoints(ctx *gin.Context) {
	// Endpoints now live inside Traffic. Keep the old path as an alias.
	target := "/traffic/"
	if q := ctx.Request.URL.RawQuery; q != "" {
		target += "?" + q
	}
	ctx.Redirect(http.StatusFound, target)
}

func (h *UIHandlers) Configuration(ctx *gin.Context) {
	page := templates.BaseLayout(
		shellData(ctx, "Configs & Secrets", "Project settings", "configuration"),
		templates.ConfigurationContent(),
	)

	if err := page.Render(ctx, ctx.Writer); err != nil {
		slog.Error("page render failed", "error", err)
	}
}

func (h *UIHandlers) Dependencies(ctx *gin.Context) {
	page := templates.BaseLayout(
		shellData(ctx, "Dependencies", "Build", "dependencies"),
		templates.DependenciesContent(),
	)
	if err := page.Render(ctx, ctx.Writer); err != nil {
		slog.Error("page render failed", "error", err)
	}
}

func (h *UIHandlers) Queries(ctx *gin.Context) {
	page := templates.BaseLayout(
		shellData(ctx, "Queries", "Data", "queries"),
		templates.QueriesContent(),
	)
	if err := page.Render(ctx, ctx.Writer); err != nil {
		slog.Error("page render failed", "error", err)
	}
}

func (h *UIHandlers) DataModel(ctx *gin.Context) {
	page := templates.BaseLayout(
		shellData(ctx, "Tables", "Data", "data-model"),
		templates.DataModelContent(),
	)
	if err := page.Render(ctx, ctx.Writer); err != nil {
		slog.Error("page render failed", "error", err)
	}
}

func (h *UIHandlers) Agent(ctx *gin.Context) {
	page := templates.BaseLayout(
		shellData(ctx, "Plan", "Build", "agent"),
		templates.AgentContent(),
	)
	if err := page.Render(ctx, ctx.Writer); err != nil {
		slog.Error("page render failed", "error", err)
	}
}

func (h *UIHandlers) PubSub(ctx *gin.Context) {
	page := templates.BaseLayout(
		shellData(ctx, "Pub/Sub", "Data", "pubsub"),
		templates.PubSubContent(),
	)
	if err := page.Render(ctx, ctx.Writer); err != nil {
		slog.Error("page render failed", "error", err)
	}
}

func (h *UIHandlers) Traffic(ctx *gin.Context) {
	page := templates.BaseLayout(
		shellData(ctx, "Traffic", "Workspace", "traffic"),
		templates.TrafficContent(h.loadEndpoints(ctx)),
	)
	if err := page.Render(ctx, ctx.Writer); err != nil {
		slog.Error("page render failed", "error", err)
	}
}

func (h *UIHandlers) Access(ctx *gin.Context) {
	page := templates.BaseLayout(
		shellData(ctx, "Team & Access", "Project settings", "access"),
		templates.AccessContent(),
	)
	if err := page.Render(ctx, ctx.Writer); err != nil {
		slog.Error("page render failed", "error", err)
	}
}

func (h *UIHandlers) Audit(ctx *gin.Context) {
	page := templates.BaseLayout(
		shellData(ctx, "Audit Log", "Project settings", "audit"),
		templates.AuditContent(),
	)
	if err := page.Render(ctx, ctx.Writer); err != nil {
		slog.Error("page render failed", "error", err)
	}
}
