package server

import (
	"github.com/ashupednekar/litewebservices-portal/pkg/handlers"
	"github.com/ashupednekar/litewebservices-portal/pkg/server/middleware"
)

func (s *Server) BuildRoutes() {

	s.router.Static("/static", "./static")
	probes := handlers.ProbeHandler{}
	s.router.GET("/livez/", probes.Livez)
	s.router.GET("/healthz/", probes.Healthz)

	auth := handlers.NewAuthHandlers(s.state)

	s.router.POST("/passkey/register/start/", auth.BeginRegistration)
	s.router.POST("/passkey/register/finish/", auth.FinishRegistration)
	s.router.POST("/passkey/login/start/", auth.BeginLogin)
	s.router.POST("/passkey/login/finish/", auth.FinishLogin)

	s.router.GET("/logout", auth.Logout)
	s.router.POST("/logout", auth.Logout)

	ui := handlers.UIHandlers{}

	s.router.GET("/", middleware.OptionalAuthMiddleware(auth.GetStore()), ui.Home)

	protected := s.router.Group("/")
	protected.Use(middleware.AuthMiddleware(auth.GetStore()))
	{
		protected.GET("/dashboard", ui.Dashboard)
		protected.GET("/functions", ui.Functions)
	}
}
