package handlers

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/ashupednekar/litefunctions/common/agent"
	"github.com/ashupednekar/litefunctions/portal/internal/state"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
)

type AgentHandlers struct {
	state *state.AppState
}

func NewAgentHandlers(s *state.AppState) *AgentHandlers {
	return &AgentHandlers{state: s}
}

func (h *AgentHandlers) available() bool {
	return h.state.Agent != nil && h.state.Agent.Enabled()
}

type promptRequest struct {
	Prompt    string `json:"prompt"`
	SessionID string `json:"session_id"`
	Directory string `json:"directory"`
}

// Prompt sends a project-aware prompt to OpenCode, then syncs the repository so
// anything the agent wrote through MCP becomes visible in the portal.
func (h *AgentHandlers) Prompt(c *gin.Context) {
	if !h.available() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "agent is not configured; deploy the opencode service"})
		return
	}

	var req promptRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if strings.TrimSpace(req.Prompt) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "prompt is required"})
		return
	}

	projectUUID := c.MustGet("projectUUID").(pgtype.UUID)
	projectName := c.MustGet("projectName").(string)
	userName := c.GetString("userName")

	result, err := h.state.Agent.Prompt(c.Request.Context(), agent.PromptRequest{
		SessionID: req.SessionID,
		Project:   projectName,
		ProjectID: projectUUID.String(),
		UserName:  userName,
		Directory: req.Directory,
		Text:      req.Prompt,
	})
	if err != nil {
		slog.Error("agent prompt failed", "project", projectName, "error", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	syncCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	userID := c.MustGet("userID").([]byte)
	syncErr := SyncRepoFunctionsToDb(syncCtx, h.state.DBPool, projectUUID, projectName, userID)

	response := gin.H{
		"session_id": result.SessionID,
		"text":       result.Text,
		"synced":     syncErr == nil,
	}
	if syncErr != nil {
		slog.Warn("agent repo sync failed", "project", projectName, "error", syncErr)
		response["sync_error"] = syncErr.Error()
	}
	c.JSON(http.StatusOK, response)
}

// Sessions lists conversations known to the agent server.
func (h *AgentHandlers) Sessions(c *gin.Context) {
	if !h.available() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "agent is not configured; deploy the opencode service"})
		return
	}
	sessions, err := h.state.Agent.Sessions(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, sessions)
}

// CreateSession starts a new conversation for the current project.
func (h *AgentHandlers) CreateSession(c *gin.Context) {
	if !h.available() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "agent is not configured; deploy the opencode service"})
		return
	}
	var req struct {
		Title     string `json:"title"`
		Directory string `json:"directory"`
	}
	_ = c.ShouldBindJSON(&req)
	session, err := h.state.Agent.CreateSession(c.Request.Context(), agent.SessionRequest{
		Title:     req.Title,
		Project:   c.MustGet("projectName").(string),
		ProjectID: c.MustGet("projectUUID").(pgtype.UUID).String(),
		UserName:  c.GetString("userName"),
		Directory: req.Directory,
	})
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, session)
}

// Messages returns the transcript for a session.
func (h *AgentHandlers) Messages(c *gin.Context) {
	if !h.available() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "agent is not configured; deploy the opencode service"})
		return
	}
	messages, err := h.state.Agent.Messages(c.Request.Context(), c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, messages)
}

// DeleteSession removes a conversation.
func (h *AgentHandlers) DeleteSession(c *gin.Context) {
	if !h.available() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "agent is not configured; deploy the opencode service"})
		return
	}
	if err := h.state.Agent.DeleteSession(c.Request.Context(), c.Param("id")); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}
