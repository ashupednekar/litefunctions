package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (s *Server) BuildRoutes() {
	s.router.GET("/ping", func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{
			"message": "pong",
		})
	})
}
