package handlers

import (
	"context"
	"encoding/hex"

	authAdaptors "github.com/ashupednekar/litewebservices-portal/internal/auth/adaptors"
	"github.com/ashupednekar/litewebservices-portal/internal/project/adaptors"
	"github.com/ashupednekar/litewebservices-portal/pkg/state"
	"github.com/ashupednekar/litewebservices-portal/templates"
	"github.com/gin-gonic/gin"
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
		q := adaptors.New(h.state.DBPool)
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
	page := templates.BaseLayout(
		templates.FunctionContent(),
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
