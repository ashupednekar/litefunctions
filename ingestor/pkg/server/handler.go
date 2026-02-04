package server

import (
	"fmt"
	"log/slog"
	"net/http"
	"github.com/ashupednekar/litefunctions/ingestor/pkg/broker"
)

type IngestHandler struct {
	server *Server
	logger *slog.Logger
}

func NewIngestHandler(server *Server) *IngestHandler {
	return &IngestHandler{
		server: server,
		logger: slog.Default(),
	}
}

func (h *IngestHandler) Sync(w http.ResponseWriter, r *http.Request) {
	project, name := parsePath(r.URL.Path)

	h.logger.Info("handling sync request", "project", project, "name", name)

	lang, err := h.server.activateFunction(project, name)
	if err != nil {
		h.logger.Error("failed to activate function", "error", err)
		http.Error(w, fmt.Sprintf("%s", err), http.StatusBadRequest)
		return
	}

	req, err := broker.Submit(h.server.nc, r, lang)
	if err != nil {
		h.logger.Error("failed to submit request to broker", "error", err)
		http.Error(w, fmt.Sprintf("%s", err), http.StatusBadRequest)
		return
	}
	res, err := broker.Reply(h.server.nc, req)
	if err != nil {
		h.logger.Error("failed to get reply from broker", "error", err)
		http.Error(w, fmt.Sprintf("%s", err), http.StatusInternalServerError)
		return
	}
	h.logger.Info("sync request completed", "project", project, "name", name, "language", lang)
	w.Write(res)
}

func (h *IngestHandler) SSE(w http.ResponseWriter, r *http.Request) {
	project, name := parsePath(r.URL.Path)

	h.logger.Info("handling SSE request", "project", project, "name", name)

	lang, err := h.server.activateFunction(project, name)
	if err != nil {
		h.logger.Error("failed to activate function", "error", err)
		http.Error(w, fmt.Sprintf("%s", err), http.StatusBadRequest)
		return
	}

	req, err := broker.Submit(h.server.nc, r, lang)
	if err != nil {
		h.logger.Error("failed to submit request to broker", "error", err)
		http.Error(w, fmt.Sprintf("%s", err), http.StatusBadRequest)
		return
	}
	ch, err := broker.Subscribe(h.server.nc, req)
	if err != nil {
		h.logger.Error("failed to subscribe to broker", "error", err)
		http.Error(w, fmt.Sprintf("%s", err), http.StatusBadRequest)
		return
	}
	for res := range ch {
		w.Write(res)
	}
}

func (h *IngestHandler) WS(w http.ResponseWriter, r *http.Request) {
	project, name := parsePath(r.URL.Path)

	h.logger.Info("handling WS request", "project", project, "name", name)

	lang, err := h.server.activateFunction(project, name)
	if err != nil {
		h.logger.Error("failed to activate function", "error", err)
		http.Error(w, fmt.Sprintf("%s", err), http.StatusBadRequest)
		return
	}

	ch := make(chan []byte)
	req, err := broker.Produce(h.server.nc, ch, w, r, lang)
	if err != nil {
		h.logger.Error("failed to produce message to broker", "error", err)
		http.Error(w, fmt.Sprintf("%s", err), http.StatusInternalServerError)
		return
	}
	ch, err = broker.Subscribe(h.server.nc, req)
	if err != nil {
		h.logger.Error("failed to subscribe to broker", "error", err)
		http.Error(w, fmt.Sprintf("%s", err), http.StatusBadRequest)
		return
	}
	for res := range ch {
		w.Write(res)
	}
}

func parsePath(path string) (string, string) {
	if len(path) < 2 || path[0] != '/' {
		return "", ""
	}
	parts := path[1:]
	project := parts
	name := ""
	for i, p := range parts {
		if p == '/' {
			project = parts[:i]
			name = parts[i+1:]
			break
		}
	}
	return project, name
}
