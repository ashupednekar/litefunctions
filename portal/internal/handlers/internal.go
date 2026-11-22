package handlers

import (
	"encoding/hex"
	"net/http"
	"strings"

	accessadaptors "github.com/ashupednekar/litefunctions/portal/internal/access/adaptors"
	"github.com/ashupednekar/litefunctions/portal/internal/auth"
	"github.com/ashupednekar/litefunctions/portal/internal/config"
	projectservice "github.com/ashupednekar/litefunctions/portal/internal/project"
	projectadaptors "github.com/ashupednekar/litefunctions/portal/internal/project/adaptors"
	"github.com/ashupednekar/litefunctions/portal/internal/state"
	"github.com/gin-gonic/gin"
)

// InternalHandlers exposes the cluster-internal project API used by the MCP
// server so the agent can list, create, and select projects.
type InternalHandlers struct {
	state *state.AppState
}

func NewInternalHandlers(s *state.AppState) *InternalHandlers {
	return &InternalHandlers{state: s}
}

func (h *InternalHandlers) ListProjects(c *gin.Context) {
	rows, err := accessadaptors.New(h.state.DBPool).ListProjectsForUserOrAdmin(
		c.Request.Context(),
		auth.SystemUserID,
		true,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list projects"})
		return
	}
	out := make([]gin.H, 0, len(rows))
	for _, row := range rows {
		out = append(out, gin.H{
			"id":   hex.EncodeToString(row.ID.Bytes[:]),
			"name": row.Name,
			"role": string(row.Role),
		})
	}
	c.JSON(http.StatusOK, gin.H{"projects": out})
}

func (h *InternalHandlers) GetProject(c *gin.Context) {
	name := strings.ToLower(strings.TrimSpace(c.Param("name")))
	project, err := projectadaptors.New(h.state.DBPool).GetProjectByName(c.Request.Context(), name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "project not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"id":   hex.EncodeToString(project.ID.Bytes[:]),
		"name": project.Name,
	})
}

func (h *InternalHandlers) CreateProject(c *gin.Context) {
	var req struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	project, err := projectservice.Create(
		c.Request.Context(),
		h.state.DBPool,
		req.Name,
		auth.SystemUserID,
		config.Cfg.VCSConfig(),
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"id":   hex.EncodeToString(project.ID.Bytes[:]),
		"name": project.Name,
	})
}
