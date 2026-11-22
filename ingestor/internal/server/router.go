package server

import "net/http"

func (s *Server) BuildRoutes() {
	registerRoutes(http.DefaultServeMux, NewIngestHandler(s))
}

func registerRoutes(mux *http.ServeMux, handler *IngestHandler) {
	mux.HandleFunc("/lambda/{project}/{name}", handler.Sync)
	// Subtree patterns keep GET path parameters and trailing slashes working;
	// without them Go's mux returns 404 before the proxy can forward the path.
	mux.HandleFunc("/lambda/{project}/{name}/", handler.Sync)
	mux.HandleFunc("/lambda/sse/{project}/{name}", handler.SSE)
	mux.HandleFunc("/lambda/sse/{project}/{name}/", handler.SSE)
	mux.HandleFunc("/lambda/ws/{project}/{name}", handler.WS)
	mux.HandleFunc("/lambda/ws/{project}/{name}/", handler.WS)
	mux.HandleFunc("/hook/{language}/{project}", handler.RuntimeHook)
}
