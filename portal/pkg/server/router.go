package server

import (
	"io/fs"
	"net/http"

	"github.com/ashupednekar/litefunctions/portal/pkg/handlers"
	"github.com/ashupednekar/litefunctions/portal/pkg/server/middleware"
	"github.com/ashupednekar/litefunctions/portal/static"
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

	runtimeAssets := handlers.NewRuntimeAssetsHandler()
	s.router.GET("/api/runtime-assets/runtimes.tar.gz", runtimeAssets.RuntimesTarGz)
	s.router.GET("/api/runtime-assets/runtimes/*filepath", runtimeAssets.RuntimesFile)

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
		functionHandlers := handlers.NewFunctionHandlers(s.state)
		endpointHandlers := handlers.NewEndpointHandlers(s.state)
		actionHandlers := handlers.NewActionHandlers()

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

		api.GET("/endpoints/", endpointHandlers.ListEndpoints)
		api.GET("/endpoints/:epID/", endpointHandlers.GetEndpoint)
		api.PUT("/endpoints/:epID/", endpointHandlers.UpdateEndpoint)
		api.POST("/endpoints/:epID/test/", endpointHandlers.TestEndpoint)

		api.GET("/actions/status/", actionHandlers.Status)

	}
}
