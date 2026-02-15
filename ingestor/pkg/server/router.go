package server

import "net/http"

func (s *Server) BuildRoutes() {
	handler := NewIngestHandler(s)
	http.HandleFunc("/lambda/{project}/{name}", handler.Sync)
	http.HandleFunc("/lambda/sse/{project}/{name}", handler.SSE)
	http.HandleFunc("/lambda/ws/{project}/{name}", handler.WS)
	http.HandleFunc("/hook/python/{project}", handler.PythonHook)
}
