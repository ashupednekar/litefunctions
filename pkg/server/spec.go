package server

import (
	"fmt"

	"github.com/ashupednekar/litewebservices-portal/pkg"
	"github.com/gin-gonic/gin"
)

type Server struct {
	Port   int
	router *gin.Engine
}

func NewServer() *Server {
	r := gin.Default()
	s := &Server{
		Port:   pkg.Cfg.Port,
		router: r,
	}
	s.BuildRoutes()
	return s
}

func (s *Server) Start() {
	s.router.Run(fmt.Sprintf("0.0.0.0:%d", s.Port))
}
