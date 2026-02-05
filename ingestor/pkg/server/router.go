package server

import "net/http"

func (s *Server) BuildRoutes() {
	handler := NewIngestHandler(s)
	http.HandleFunc("/{project}/{name}", handler.Sync)
	http.HandleFunc("/sse/{project}/{name}", handler.SSE)
	http.HandleFunc("/ws/{project}/{name}", handler.WS)
}
