package handlers

import (
	"context"
	"encoding/hex"
	"fmt"
	"log/slog"
	"strings"
	"time"

	agentadaptor "github.com/ashupednekar/litefunctions/agent/adaptor"
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

func relativeTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "now"
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}

func truncateText(value string, limit int) string {
	value = strings.TrimSpace(value)
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit]) + "…"
}

// projectRepoURL builds the public web URL for a project's VCS repository.
func projectRepoURL(projectName string) string {
	if strings.TrimSpace(projectName) == "" {
		return ""
	}
	vc := config.Cfg.VCSConfig()
	base := strings.TrimSpace(vc.PublicBaseUrl)
	if base == "" {
		base = "https://" + config.Cfg.Fqdn
	}
	return strings.TrimRight(base, "/") + "/" + vc.User + "/" + projectName
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
	userID, ok := ctx.Get("userID")
	if !ok || userID == nil {
		page := templates.AuthLayout(
			templates.HomeContent(config.Cfg.AdminLoginEnabled),
		)
		if err := page.Render(ctx, ctx.Writer); err != nil {
			slog.Error("page render failed", "error", err)
		}
		return
	}

	uid := userID.([]byte)
	username := ctx.GetString("userName")
	activeProjectID, _ := ctx.Cookie("lws_project")

	var projects []templates.Project
	projectsQ := accessAdaptors.New(h.state.DBPool)
	if rows, err := projectsQ.ListProjectsForUserOrAdmin(
		ctx.Request.Context(),
		uid,
		auth.IsSystemUserID(uid),
	); err == nil {
		for _, row := range rows {
			projects = append(projects, templates.Project{
				ID:   hex.EncodeToString(row.ID.Bytes[:]),
				Name: row.Name,
				Role: string(row.Role),
			})
		}
	}

	data := templates.HomeAgentData{
		Username:        username,
		Projects:        projects,
		ActiveProjectID: activeProjectID,
	}
	for _, p := range projects {
		if p.ID == activeProjectID {
			data.ActiveProject = p.Name
			data.Role = p.Role
			break
		}
	}
	if data.ActiveProject == "" && len(projects) > 0 {
		// The agent screen is only useful with an active project.
		data.ActiveProjectID = projects[0].ID
		data.ActiveProject = projects[0].Name
		data.Role = projects[0].Role
		ctx.SetCookie("lws_project", projects[0].ID, 60*60*24*30, "/", "", false, true)
	}

	if data.ActiveProjectID != "" && h.state.DBPool != nil {
		if raw, err := hex.DecodeString(data.ActiveProjectID); err == nil && len(raw) == 16 {
			var projectUUID pgtype.UUID
			copy(projectUUID.Bytes[:], raw)
			projectUUID.Valid = true
			q := agentadaptor.New(h.state.DBPool)
			sessions, err := q.ListAgentSessionsForProject(ctx.Request.Context(), agentadaptor.ListAgentSessionsForProjectParams{
				ProjectID: projectUUID,
				UserID:    uid,
				RowLimit:  20,
			})
			if err == nil {
				for _, session := range sessions {
					summary := templates.AgentSessionSummary{
						ID:      session.ID,
						Title:   session.Title,
						Updated: relativeTime(session.UpdatedAt.Time),
					}
					chats, err := q.ListAgentChatsForSession(ctx.Request.Context(), agentadaptor.ListAgentChatsForSessionParams{
						SessionID: session.ID,
						Limit:     1,
					})
					if err == nil && len(chats) > 0 {
						chat := chats[0]
						if strings.TrimSpace(chat.Prompt) != "" {
							summary.Preview = truncateText(chat.Prompt, 80)
						}
					}
					if summary.Title == "" {
						summary.Title = "Session"
					}
					if summary.Preview == "" {
						summary.Preview = "No messages yet"
					}
					data.Sessions = append(data.Sessions, summary)
				}
			}
		}
	}

	shell := templates.ShellData{
		Title:       "Agent",
		Section:     "Workspace",
		Active:      "home",
		Username:    username,
		Role:        data.Role,
		ProjectID:   data.ActiveProjectID,
		ProjectName: data.ActiveProject,
	}
	page := templates.BaseLayout(shell, templates.HomeAgentContent(data))
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
	projectName, _ := ctx.Get("projectName")
	projectNameStr, _ := projectName.(string)
	repoURL := projectRepoURL(projectNameStr)
	endpointByFn := map[string]string{}
	endpointMethodByFn := map[string]string{}
	endpointNameByFn := map[string]string{}
	if eps, err := epQ.ListEndpointsForProject(ctx.Request.Context(), projUUID); err == nil {
		for _, ep := range eps {
			fnID := hex.EncodeToString(ep.FunctionID.Bytes[:])
			if _, exists := endpointByFn[fnID]; exists {
				continue
			}
			endpointByFn[fnID] = hex.EncodeToString(ep.ID.Bytes[:])
			endpointMethodByFn[fnID] = ep.Method
			endpointNameByFn[fnID] = ep.Name
		}
	}

	if dbFns, err := q.ListFunctionsForProject(ctx.Request.Context(), projUUID); err == nil {
		for _, f := range dbFns {
			lang := f.Language
			icon := fmt.Sprintf("/static/imgs/%s-svgrepo-com.svg", lang)
			fnID := hex.EncodeToString(f.ID.Bytes[:])

			method := endpointMethodByFn[fnID]
			if f.IsAsync {
				method = "GET"
			} else if method == "" {
				method = "POST"
			}
			endpointName := endpointNameByFn[fnID]
			if endpointName == "" {
				endpointName = fmt.Sprintf("/%s/%s", projectNameStr, f.Name)
			}
			functions = append(functions, templates.Function{
				ID:           fnID,
				Name:         f.Name,
				ProjectName:  projectNameStr,
				Language:     lang,
				Icon:         icon,
				EndpointID:   endpointByFn[fnID],
				EndpointName: endpointName,
				IsAsync:      f.IsAsync,
				IsUI:         f.IsUi || strings.HasSuffix(f.Name, "-ui"),
				Method:       method,
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
		templates.FunctionContent(functions, langs, activeProjectID, usernameStr, samples, h.loadEndpoints(ctx), repoURL),
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
				FunctionID:   hex.EncodeToString(e.FunctionID.Bytes[:]),
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
