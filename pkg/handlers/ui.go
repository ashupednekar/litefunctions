package handlers

import (
	"github.com/ashupednekar/litewebservices-portal/templates"
	"github.com/gin-gonic/gin"
)

type UIHandlers struct{}

func (h *UIHandlers) Home(ctx *gin.Context) {
	c := templates.Hello("ashu")
	if err := c.Render(ctx, ctx.Writer); err != nil {
		ctx.JSON(500, gin.H{"err": err.Error()})
	}
}
