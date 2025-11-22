package server

import (
	"github.com/ashupednekar/litewebservices-portal/pkg/handlers"
)

func (s *Server) BuildRoutes() {
	s.router.Static("/static", "./static")
	probes := handlers.ProbeHandler{}
	s.router.GET("/livez/", probes.Livez)
	s.router.GET("/healthz/", probes.Healthz)
	auth := handlers.NewAuthHandlers(s.state)
	s.router.GET("/passkey/register/start/", auth.BeginRegistration)
	s.router.POST("/passkey/register/finish/", auth.FinishRegistration)
	s.router.GET("/passkey/login/start/", auth.BeginLogin)
	s.router.POST("/passkey/login/finish/", auth.FinishLogin)
	ui := handlers.UIHandlers{}
	s.router.GET("/", ui.Home)
	s.router.GET("/dashboard", ui.Dashboard)
	s.router.GET("/functions", ui.Functions)
}
