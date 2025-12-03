package handlers

import (
	"context"
	"encoding/hex"
	"fmt"

	authAdaptors "github.com/ashupednekar/litewebservices-portal/internal/auth/adaptors"
	functionAdaptors "github.com/ashupednekar/litewebservices-portal/internal/function/adaptors"
	projectAdaptors "github.com/ashupednekar/litewebservices-portal/internal/project/adaptors"
	"github.com/ashupednekar/litewebservices-portal/pkg/state"
	"github.com/ashupednekar/litewebservices-portal/templates"
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
	page := templates.BaseLayout(
		templates.HomeContent(),
	)

	if err := page.Render(ctx, ctx.Writer); err != nil {
		ctx.JSON(500, gin.H{"err": err.Error()})
	}
}

func (h *UIHandlers) Dashboard(ctx *gin.Context) {
	userID, _ := ctx.Get("userID")
	activeProjectID, _ := ctx.Cookie("lws_project")

	var projects []templates.Project
	usernameStr := "User"

	if userID != nil {
		q := projectAdaptors.New(h.state.DBPool)
		dbProjects, err := q.ListProjectsForUser(ctx.Request.Context(), userID.([]byte))
		if err == nil {
			for _, p := range dbProjects {
				projects = append(projects, templates.Project{
					ID:   hex.EncodeToString(p.ID.Bytes[:]),
					Name: p.Name,
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
	userID, _ := ctx.Get("userID")
	activeProjectID, _ := ctx.Cookie("lws_project")

	var functions []templates.Function
	usernameStr := "User"

	q := functionAdaptors.New(h.state.DBPool)

	if activeProjectID != "" {
		var projUUID pgtype.UUID

		if decoded, err := hex.DecodeString(activeProjectID); err == nil && len(decoded) == 16 {
			copy(projUUID.Bytes[:], decoded)
			projUUID.Valid = true

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
		}
	}

	if userID != nil {
		authQ := authAdaptors.New(h.state.DBPool)
		if user, err := authQ.GetUserByID(context.Background(), userID.([]byte)); err == nil {
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
		ctx.JSON(500, gin.H{"err": err.Error()})
	}
}

func (h *UIHandlers) Data(ctx *gin.Context) {
	page := templates.BaseLayout(
		templates.DataContent(),
	)

	if err := page.Render(ctx, ctx.Writer); err != nil {
		ctx.JSON(500, gin.H{"err": err.Error()})
	}
}

func (h *UIHandlers) Endpoints(ctx *gin.Context) {
	page := templates.BaseLayout(
		templates.EndpointsContent(),
	)

	if err := page.Render(ctx, ctx.Writer); err != nil {
		ctx.JSON(500, gin.H{"err": err.Error()})
	}
}

func (h *UIHandlers) Configuration(ctx *gin.Context) {
	page := templates.BaseLayout(
		templates.ConfigurationContent(),
	)

	if err := page.Render(ctx, ctx.Writer); err != nil {
		ctx.JSON(500, gin.H{"err": err.Error()})
	}
}
