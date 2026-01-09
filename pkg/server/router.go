package server

import (
	"io/fs"
	"net/http"

	"github.com/ashupednekar/litewebservices-portal/pkg/handlers"
	"github.com/ashupednekar/litewebservices-portal/pkg/server/middleware"
	"github.com/ashupednekar/litewebservices-portal/static"
)

func (s *Server) BuildRoutes() {
	staticFS, err := fs.Sub(static.Files, ".")
	if err != nil {
		panic(err)
	}

	s.router.StaticFS("/static/", http.FS(staticFS))

	probes := handlers.ProbeHandler{}
	s.router.GET("/livez/", probes.Livez)
	s.router.GET("/healthz/", probes.Healthz)

	auth := handlers.NewAuthHandlers(s.state)

	s.router.POST("/passkey/register/start/", auth.BeginRegistration)
	s.router.POST("/passkey/register/finish/", auth.FinishRegistration)
	s.router.POST("/passkey/login/start/", auth.BeginLogin)
	s.router.POST("/passkey/login/finish/", auth.FinishLogin)

	s.router.GET("/logout/", auth.Logout)
	s.router.POST("/logout/", auth.Logout)

	ui := handlers.NewUIHandlers(s.state)

	s.router.GET("/", ui.Home)

	dashboard := s.router.Group("/")
	dashboard.Use(middleware.AuthMiddleware(auth.GetStore()))
	{
		dashboard.GET("/dashboard/", ui.Dashboard)
	}

	protected := s.router.Group("/")
	protected.Use(
		middleware.AuthMiddleware(auth.GetStore()),
		middleware.ProjectMiddleware(s.state),
	)
	{
		protected.GET("/configuration/", ui.Configuration)
		protected.GET("/functions/", ui.Functions)
		protected.GET("/endpoints/", ui.Endpoints)
		protected.GET("/data/", ui.Data)
	}

	projectHandlers := handlers.NewProjectHandlers(s.state)
	functionHandlers := handlers.NewFunctionHandlers(s.state)

	apiJoined := s.router.Group("/api/")
	apiJoined.Use(middleware.AuthMiddleware(auth.GetStore()))
	{
		apiJoined.POST("/projects/", projectHandlers.CreateProject)
		apiJoined.POST("/projects/join/", projectHandlers.JoinProjectByInvite)
	}

	api := s.router.Group("/api/")
	api.Use(
		middleware.AuthMiddleware(auth.GetStore()),
		middleware.ProjectMiddleware(s.state),
	)
	{
		api.GET("/projects/", projectHandlers.ListProjects)
		api.GET("/projects/:id/", projectHandlers.GetProject)
		api.DELETE("/projects/:id/", projectHandlers.DeleteProject)
		api.POST("/projects/sync/", projectHandlers.SyncProject)

		api.POST("/projects/invites/", projectHandlers.CreateProjectInvite)
		api.GET("/projects/access/", projectHandlers.ListProjectAccess)
		api.PUT("/projects/access/:id/", projectHandlers.UpdateProjectAccess)
		api.DELETE("/projects/access/:id/", projectHandlers.RevokeProjectAccess)

		api.POST("/functions/", functionHandlers.CreateFunction)
		api.GET("/functions/", functionHandlers.ListFunctions)
		api.GET("/functions/:fnID/", functionHandlers.GetFunction)
		api.PUT("/functions/:fnID/", functionHandlers.UpdateFunction)
		api.DELETE("/functions/:fnID/", functionHandlers.DeleteFunction)

		api.POST("/endpoints/", handlers.CreateEndpoint)
		api.GET("/endpoints/", handlers.ListEndpoints)
		api.GET("/endpoints/:epID/", handlers.GetEndpoint)
		api.PUT("/endpoints/:epID/", handlers.UpdateEndpoint)
		api.DELETE("/endpoints/:epID/", handlers.DeleteEndpoint)

		api.GET("/config/", handlers.GetProjectConfig)
		api.PUT("/config/", handlers.UpdateProjectConfig)
	}
}
