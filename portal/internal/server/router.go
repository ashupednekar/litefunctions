package server

import (
	"io/fs"
	"net/http"

	"github.com/ashupednekar/litefunctions/portal/internal/config"
	"github.com/ashupednekar/litefunctions/portal/internal/handlers"
	"github.com/ashupednekar/litefunctions/portal/internal/server/middleware"
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
	s.router.POST("/admin/login/", auth.AdminLogin)

	s.router.GET("/logout/", auth.Logout)
	s.router.POST("/logout/", auth.Logout)

	runtimeAssets := handlers.NewRuntimeAssetsHandler()
	s.router.GET("/api/runtime-assets/runtimes.tar.gz", runtimeAssets.RuntimesTarGz)
	s.router.GET("/api/runtime-assets/runtimes/*filepath", runtimeAssets.RuntimesFile)

	ui := handlers.NewUIHandlers(s.state)

	s.router.GET("/", middleware.OptionalAuthMiddleware(auth.GetStore()), ui.Home)

	// Cluster-internal API for the MCP server to list and create projects.
	internal := s.router.Group("/internal")
	internal.Use(middleware.InternalToken(config.Cfg.MCPToken))
	{
		internalHandlers := handlers.NewInternalHandlers(s.state)
		internal.GET("/projects/", internalHandlers.ListProjects)
		internal.POST("/projects/", internalHandlers.CreateProject)
		internal.GET("/projects/:name/", internalHandlers.GetProject)
	}

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
		protected.GET("/data/", ui.Data)
		protected.GET("/dependencies/", ui.Dependencies)
		protected.GET("/data-model/", ui.DataModel)
		protected.GET("/queries/", ui.Queries)
		protected.GET("/agent/", ui.Agent)
		protected.GET("/pubsub/", ui.PubSub)
		protected.GET("/access/", ui.Access)
		protected.GET("/audit/", ui.Audit)
	}

	projectHandlers := handlers.NewProjectHandlers(s.state)
	agentHandlers := handlers.NewAgentHandlers(s.state)

	apiJoined := s.router.Group("/api/")
	apiJoined.Use(middleware.AuthMiddleware(auth.GetStore()))
	{
		apiJoined.POST("/projects/", projectHandlers.CreateProject)
		apiJoined.POST("/projects/join/", projectHandlers.JoinProjectByInvite)
		// Agent history/activity reads do not need a repo clone. Moving them
		// here keeps a broken or missing project repo from breaking the chat UI.
		apiJoined.GET("/agent/chats/:id/", agentHandlers.Chat)
		apiJoined.GET("/agent/sessions/:id/messages/", agentHandlers.Messages)
		apiJoined.GET("/agent/sessions/:id/history/", agentHandlers.History)
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

		manager := middleware.RequireProjectRole("owner", "manager")

		api.GET("/projects/", projectHandlers.ListProjects)
		api.GET("/projects/:id/", projectHandlers.GetProject)
		api.DELETE("/projects/:id/", manager, projectHandlers.DeleteProject)
		api.POST("/projects/sync/", manager, projectHandlers.SyncProject)

		api.POST("/projects/invites/", projectHandlers.CreateProjectInvite)
		api.GET("/projects/access/", projectHandlers.ListProjectAccess)
		api.PUT("/projects/access/:id/", projectHandlers.UpdateProjectAccess)
		api.DELETE("/projects/access/:id/", projectHandlers.RevokeProjectAccess)

		api.POST("/functions/", manager, functionHandlers.CreateFunction)
		api.GET("/functions/", functionHandlers.ListFunctions)
		api.GET("/functions/:fnID/", functionHandlers.GetFunction)
		api.GET("/functions/:fnID/files/", functionHandlers.ListFunctionFiles)
		api.GET("/functions/:fnID/logs/", functionHandlers.StreamLogs)
		api.PUT("/functions/:fnID/", manager, functionHandlers.UpdateFunction)
		api.DELETE("/functions/:fnID/", manager, functionHandlers.DeleteFunction)

		api.GET("/endpoints/", endpointHandlers.ListEndpoints)
		api.GET("/endpoints/:epID/", endpointHandlers.GetEndpoint)
		api.PUT("/endpoints/:epID/", manager, endpointHandlers.UpdateEndpoint)

		api.GET("/actions/status/", actionHandlers.Status)

		api.POST("/agent/prompt/", agentHandlers.Prompt)
		api.GET("/agent/sessions/", agentHandlers.Sessions)
		api.POST("/agent/sessions/", agentHandlers.CreateSession)
		api.DELETE("/agent/sessions/:id/", agentHandlers.DeleteSession)
	}
}
