package server

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/ashupednekar/litefunctions/ingestor/pkg/broker"
	"github.com/gorilla/websocket"
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

	info, err := h.server.activateFunction(project, name)
	if err != nil {
		h.logger.Error("failed to activate function", "error", err)
		http.Error(w, fmt.Sprintf("%s", err), http.StatusBadRequest)
		return
	}
	if !h.validateMethod(w, r, info.Method, project, name) {
		return
	}

	if info.IsAsync {
		_, err := broker.Submit(h.server.nc, r, info.Language)
		if err != nil {
			h.logger.Error("failed to submit request to broker", "error", err)
			http.Error(w, fmt.Sprintf("%s", err), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusAccepted)
		return
	}

	if info.ServiceName != "" && info.ServicePort > 0 {
		if err := proxyToRuntime(w, r, project, name, "default", info.ServiceName, int(info.ServicePort)); err != nil {
			h.logger.Error("failed to proxy request to runtime", "error", err)
			http.Error(w, fmt.Sprintf("%s", err), http.StatusBadGateway)
		}
		return
	}

	req, err := broker.Submit(h.server.nc, r, info.Language)
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
	h.logger.Info("sync request completed", "project", project, "name", name, "language", info.Language)
	w.Write(res)
}

func proxyToRuntime(w http.ResponseWriter, r *http.Request, project, name, namespace, service string, port int) error {
	runtimePath := strings.TrimPrefix(r.URL.Path, "/lambda/"+project+"/"+name)
	if runtimePath == "" {
		runtimePath = "/"
	}
	if !strings.HasPrefix(runtimePath, "/") {
		runtimePath = "/" + runtimePath
	}

	base := fmt.Sprintf("http://%s.%s.svc.cluster.local:%d", service, namespace, port)
	u, err := url.Parse(base)
	if err != nil {
		return err
	}
	u.Path = runtimePath
	u.RawQuery = r.URL.RawQuery

	req, err := http.NewRequestWithContext(r.Context(), r.Method, u.String(), r.Body)
	if err != nil {
		return err
	}
	req.Header = r.Header.Clone()

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	for k, vals := range resp.Header {
		for _, v := range vals {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	_, err = io.Copy(w, resp.Body)
	if err != nil {
		slog.Error("error writing response body", "error", err)
	}
	slog.Info("response", "resp", resp)
	return err
}

func (h *IngestHandler) SSE(w http.ResponseWriter, r *http.Request) {
	project, name := parsePath(r.URL.Path)

	h.logger.Info("handling SSE request", "project", project, "name", name)

	info, err := h.server.activateFunction(project, name)
	if err != nil {
		h.logger.Error("failed to activate function", "error", err)
		http.Error(w, fmt.Sprintf("%s", err), http.StatusBadRequest)
		return
	}
	if !h.validateMethod(w, r, info.Method, project, name) {
		return
	}

	req, err := broker.Submit(h.server.nc, r, info.Language)
	if err != nil {
		h.logger.Error("failed to submit request to broker", "error", err)
		http.Error(w, fmt.Sprintf("%s", err), http.StatusBadRequest)
		return
	}
	ch, cleanup, err := broker.Subscribe(h.server.nc, req)
	if err != nil {
		h.logger.Error("failed to subscribe to broker", "error", err)
		http.Error(w, fmt.Sprintf("%s", err), http.StatusBadRequest)
		return
	}
	defer cleanup()
	for {
		select {
		case <-r.Context().Done():
			return
		case res, ok := <-ch:
			if !ok {
				return
			}
			w.Write(res)
		}
	}
}

func (h *IngestHandler) WS(w http.ResponseWriter, r *http.Request) {
	project, name := parsePath(r.URL.Path)

	h.logger.Info("handling WS request", "project", project, "name", name)

	info, err := h.server.activateFunction(project, name)
	if err != nil {
		h.logger.Error("failed to activate function", "error", err)
		http.Error(w, fmt.Sprintf("%s", err), http.StatusBadRequest)
		return
	}
	if !h.validateMethod(w, r, info.Method, project, name) {
		return
	}

	conn, req, err := broker.Produce(h.server.nc, w, r, info.Language)
	if err != nil {
		h.logger.Error("failed to produce message to broker", "error", err)
		return
	}
	defer conn.Close()
	ch, cleanup, err := broker.Subscribe(h.server.nc, req)
	if err != nil {
		h.logger.Error("failed to subscribe to broker", "error", err)
		http.Error(w, fmt.Sprintf("%s", err), http.StatusBadRequest)
		return
	}
	defer cleanup()
	for {
		select {
		case <-r.Context().Done():
			return
		case res, ok := <-ch:
			if !ok {
				return
			}
			if err := conn.WriteMessage(websocket.BinaryMessage, res); err != nil {
				h.logger.Error("failed to write websocket response", "error", err)
				return
			}
		}
	}
}

func (h *IngestHandler) validateMethod(w http.ResponseWriter, r *http.Request, expected, project, name string) bool {
	if expected == "" {
		return true
	}
	if strings.EqualFold(r.Method, expected) {
		return true
	}
	allowed := strings.ToUpper(expected)
	w.Header().Set("Allow", allowed)
	h.logger.Warn("method mismatch", "project", project, "name", name, "expected", allowed, "actual", r.Method)
	http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
	return false
}

func parsePath(path string) (string, string) {
	trimmed := strings.Trim(path, "/")
	if trimmed == "" {
		return "", ""
	}

	parts := strings.Split(trimmed, "/")
	if len(parts) < 3 || parts[0] != "lambda" {
		return "", ""
	}

	idx := 1
	if parts[1] == "sse" || parts[1] == "ws" {
		idx = 2
	}

	if len(parts) <= idx+1 {
		return "", ""
	}

	return parts[idx], parts[idx+1]
}
