package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/ashupednekar/litefunctions/operator/internal/client"
	"github.com/go-logr/logr"
)

type StatusHandler struct {
	Client *client.Client
	Log    logr.Logger
}

func NewStatusHandler(client *client.Client, log logr.Logger) *StatusHandler {
	return &StatusHandler{
		Client: client,
		Log:    log,
	}
}

func (h *StatusHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	namespace := r.URL.Query().Get("namespace")
	name := r.URL.Query().Get("name")

	if namespace == "" || name == "" {
		h.writeError(w, http.StatusBadRequest, "namespace and name query parameters are required")
		return
	}

	isActive, err := h.Client.IsFunctionActive(r.Context(), namespace, name)
	if err != nil {
		h.Log.Error(err, "Failed to check function active status", "namespace", namespace, "name", name)
		h.writeError(w, http.StatusInternalServerError, "Failed to check function status: "+err.Error())
		return
	}

	resp := map[string]any{
		"namespace": namespace,
		"name":      name,
		"isActive":  isActive,
	}

	h.writeJSON(w, http.StatusOK, resp)
}

func (h *StatusHandler) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (h *StatusHandler) writeError(w http.ResponseWriter, status int, message string) {
	h.writeJSON(w, status, map[string]string{"error": message})
}
