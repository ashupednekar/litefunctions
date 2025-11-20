package handlers

import (
	"github.com/ashupednekar/litewebservices-portal/templates"
	"github.com/gin-gonic/gin"
)

type UIHandlers struct{}

func (h *UIHandlers) Home(ctx *gin.Context) {
	page := templates.BaseLayout(
		templates.HomeContent(), 
	)

	if err := page.Render(ctx, ctx.Writer); err != nil {
		ctx.JSON(500, gin.H{"err": err.Error()})
	}
}


func (h *UIHandlers) Dashboard(ctx *gin.Context) {
	page := templates.BaseLayout(
		templates.DashboardContent(), 
	)

	if err := page.Render(ctx, ctx.Writer); err != nil {
		ctx.JSON(500, gin.H{"err": err.Error()})
	}
}
