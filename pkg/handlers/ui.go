package handlers

import (
	"github.com/ashupednekar/litewebservices-portal/templates"
	"github.com/gin-gonic/gin"
)

type UIHandlers struct{}

func (h *UIHandlers) Home(ctx *gin.Context) {
	page := templates.BaseLayout(
		templates.HomeContent(), // passed into @content
	)

	if err := page.Render(ctx, ctx.Writer); err != nil {
		ctx.JSON(500, gin.H{"err": err.Error()})
	}
}
