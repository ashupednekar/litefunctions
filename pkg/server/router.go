package server

func (s *Server) BuildRoutes() {
	s.router.GET("/livez/", Livez)
	s.router.GET("/healthz/", s.Healthz)
}
