package server

import (
	"github.com/ashupednekar/litewebservices-portal/pkg/handlers"
)

func (s *Server) BuildRoutes() {
	probes := handlers.ProbeHandler{}
	s.router.GET("/livez/", probes.Livez)
	s.router.GET("/healthz/", probes.Healthz)
	ui := handlers.UIHandlers{}
	s.router.GET("/", ui.Home)
}
