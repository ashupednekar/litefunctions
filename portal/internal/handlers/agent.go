package handlers

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	agentadaptor "github.com/ashupednekar/litefunctions/agent/adaptor"
	"github.com/ashupednekar/litefunctions/common/agent"
	accessadaptors "github.com/ashupednekar/litefunctions/portal/internal/access/adaptors"
	"github.com/ashupednekar/litefunctions/portal/internal/auth"
	"github.com/ashupednekar/litefunctions/portal/internal/config"
	functionadaptors "github.com/ashupednekar/litefunctions/portal/internal/function/adaptors"
	"github.com/ashupednekar/litefunctions/portal/internal/state"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
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

func (h *AgentHandlers) historyAvailable() bool {
	return h.state.DBPool != nil
}

func (h *AgentHandlers) authorizeProject(c *gin.Context, projectID pgtype.UUID) bool {
	userID, ok := c.Get("userID")
	if !ok || userID == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return false
	}
	uid := userID.([]byte)
	if auth.IsSystemUserID(uid) {
		return true
	}
	if _, err := accessadaptors.New(h.state.DBPool).GetUserProjectRole(c.Request.Context(), accessadaptors.GetUserProjectRoleParams{
		UserID:    uid,
		ProjectID: projectID,
	}); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return false
	}
	return true
}

func (h *AgentHandlers) agentQueries() *agentadaptor.Queries {
	return agentadaptor.New(h.state.DBPool)
}

// saveMessages records the transcript and any tool calls. The surrounding
// prompt already runs in the background, so these writes do not block a
// request.
func (h *AgentHandlers) saveMessages(ctx context.Context, chatID pgtype.UUID, sessionID string, messages []agent.Message) {
	q := h.agentQueries()
	for _, message := range messages {
		parts, err := json.Marshal(message.Parts)
		if err != nil {
			continue
		}
		apiErr := ""
		if message.Info.Error != nil {
			apiErr = message.Info.Error.Error()
		}
		_, err = q.UpsertAgentMessage(ctx, agentadaptor.UpsertAgentMessageParams{
			SessionID: sessionID,
			ID:        message.Info.ID,
			ChatID:    chatID,
			Role:      message.Info.Role,
			Text:      message.Text(),
			Parts:     parts,
			Model:     agentNullableText(message.Info.Model),
			Provider:  agentNullableText(message.Info.Provider),
			Error:     agentNullableText(apiErr),
		})
		if err != nil {
			slog.Warn("agent message write failed", "session", sessionID, "error", err)
			continue
		}
		for _, call := range message.ToolCalls() {
			_, err := q.UpsertAgentToolCall(ctx, agentadaptor.UpsertAgentToolCallParams{
				SessionID:   sessionID,
				ID:          call.ID,
				MessageID:   call.MessageID,
				ChatID:      chatID,
				Tool:        call.Tool,
				Status:      call.Status,
				Title:       call.Title,
				Input:       call.Input,
				Output:      call.Output,
				Error:       call.Error,
				StartedAt:   agentTimestamp(call.StartedAt),
				CompletedAt: agentTimestamp(call.CompletedAt),
			})
			if err != nil {
				slog.Warn("agent tool call write failed", "tool", call.Tool, "error", err)
			}
		}
	}
}

func agentTimestamp(t time.Time) pgtype.Timestamptz {
	if t.IsZero() {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: t, Valid: true}
}

func agentNullableText(value string) pgtype.Text {
	if value == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: value, Valid: true}
}

type promptRequest struct {
	Prompt    string `json:"prompt"`
	SessionID string `json:"session_id"`
	Title     string `json:"title"`
	Directory string `json:"directory"`
}

// Prompt starts an agent chat and returns immediately. The chat continues in the
// background so a browser refresh or navigation does not cancel the agent.
func (h *AgentHandlers) Prompt(c *gin.Context) {
	if !h.available() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "agent is not configured; deploy the opencode service"})
		return
	}
	if !h.historyAvailable() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "agent history store is not configured"})
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
	userID := c.MustGet("userID").([]byte)
	directory := strings.TrimSpace(req.Directory)
	if directory == "" {
		directory = "/workspace"
	}

	sessionID := strings.TrimSpace(req.SessionID)
	title := ""
	if sessionID == "" {
		session, err := h.state.Agent.CreateSession(c.Request.Context(), agent.SessionRequest{
			Title:     req.Title,
			Project:   projectName,
			ProjectID: projectUUID.String(),
			UserName:  userName,
			Directory: req.Directory,
		})
		if err != nil {
			slog.Error("agent session failed", "project", projectName, "error", err)
			c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
			return
		}
		sessionID = session.ID
		title = session.Title
	} else if stored, err := h.agentQueries().GetAgentSession(c.Request.Context(), sessionID); err == nil {
		title = stored.Title
	}
	if title == "" {
		title = fmt.Sprintf("%s · %s", projectName, userName)
	}

	if _, err := h.agentQueries().UpsertAgentSession(c.Request.Context(), agentadaptor.UpsertAgentSessionParams{
		ID:        sessionID,
		ProjectID: projectUUID,
		UserID:    userID,
		Title:     title,
		Directory: directory,
		Model:     config.Cfg.AgentModel,
	}); err != nil {
		slog.Error("agent session store failed", "project", projectName, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to store agent session"})
		return
	}

	chat, err := h.agentQueries().CreateAgentChat(c.Request.Context(), agentadaptor.CreateAgentChatParams{
		SessionID: sessionID,
		ProjectID: projectUUID,
		Prompt:    req.Prompt,
	})
	if err != nil {
		slog.Error("agent chat store failed", "project", projectName, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to store agent chat"})
		return
	}

	go h.runChat(chat.ID, sessionID, projectUUID, projectName, userName, userID, directory, req.Prompt)

	c.JSON(http.StatusAccepted, gin.H{
		"chat_id":    uuidHex(chat.ID),
		"session_id": sessionID,
		"status":     chat.Status,
	})
}

// runChat executes one prompt and keeps the project and activity snapshots
// fresh until OpenCode finishes.
func (h *AgentHandlers) runChat(chatID pgtype.UUID, sessionID string, projectUUID pgtype.UUID, projectName, userName string, userID []byte, directory, prompt string) {
	timeout := time.Duration(config.Cfg.AgentTimeout) * time.Second
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	tracker := &chatTracker{ids: map[string]struct{}{}}
	done := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		h.syncLoop(ctx, done, chatID, projectUUID, projectName, userID)
	}()
	go func() {
		defer wg.Done()
		h.toolLoop(ctx, done, chatID, sessionID, tracker)
	}()

	result, err := h.state.Agent.Prompt(ctx, agent.PromptRequest{
		SessionID: sessionID,
		Project:   projectName,
		ProjectID: projectUUID.String(),
		UserName:  userName,
		Directory: directory,
		Text:      prompt,
	})

	close(done)
	wg.Wait()

	finalCtx, finalCancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer finalCancel()

	if err != nil {
		slog.Error("agent chat failed", "project", projectName, "session", sessionID, "error", err)
		h.finishChat(finalCtx, chatID, "failed", "", err.Error(), tracker.count())
		return
	}

	// Capture the final message and a last project snapshot. The regular loops
	// stop as soon as the prompt returns, so this closes the gap.
	if calls := h.pollTools(finalCtx, chatID, sessionID, tracker); len(calls) > 0 {
		slog.Debug("agent chat tools", "session", sessionID, "count", tracker.count())
	}
	if _, syncErr := h.syncRun(finalCtx, chatID, projectUUID, projectName, userID); syncErr != nil {
		slog.Warn("agent final sync failed", "project", projectName, "error", syncErr)
	}
	h.saveMessages(finalCtx, chatID, sessionID, []agent.Message{result.Message})
	h.finishChat(finalCtx, chatID, "completed", result.Text, "", tracker.count())
}

func (h *AgentHandlers) finishChat(ctx context.Context, chatID pgtype.UUID, status, reply, errText string, toolCalls int) {
	if _, err := h.agentQueries().FinishAgentChat(ctx, agentadaptor.FinishAgentChatParams{
		ID:        chatID,
		Status:    status,
		Reply:     reply,
		Error:     errText,
		ToolCalls: int32(toolCalls),
	}); err != nil {
		slog.Warn("agent chat finish failed", "chat", uuidHex(chatID), "error", err)
	}
}

func (h *AgentHandlers) syncLoop(ctx context.Context, done <-chan struct{}, chatID, projectUUID pgtype.UUID, projectName string, userID []byte) {
	interval := time.Duration(config.Cfg.AgentSyncSeconds) * time.Second
	if interval <= 0 {
		interval = time.Minute
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-done:
			return
		case <-ticker.C:
			syncCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
			if _, err := h.syncRun(syncCtx, chatID, projectUUID, projectName, userID); err != nil {
				slog.Warn("agent project sync failed", "project", projectName, "error", err)
			}
			cancel()
		}
	}
}

// syncRun mirrors the project repository into the portal and records the
// current function list on the chat.
func (h *AgentHandlers) syncRun(ctx context.Context, chatID, projectUUID pgtype.UUID, projectName string, userID []byte) (*functionadaptors.Function, error) {
	if err := SyncRepoFunctionsToDb(ctx, h.state.DBPool, projectUUID, projectName, userID); err != nil {
		return nil, err
	}

	q := functionadaptors.New(h.state.DBPool)
	fns, err := q.ListFunctionsForProject(ctx, projectUUID)
	if err != nil {
		return nil, err
	}

	sortFunctionsNewestFirst(fns)
	summaries := make([]functionSummary, 0, len(fns))
	for _, fn := range fns {
		summaries = append(summaries, functionSummary{
			ID:        uuidHex(fn.ID),
			Name:      fn.Name,
			Language:  fn.Language,
			Path:      fn.Path,
			IsAsync:   fn.IsAsync,
			CreatedAt: timeValue(fn.CreatedAt),
		})
	}
	functionsJSON, err := json.Marshal(summaries)
	if err != nil {
		return nil, err
	}

	var latestID pgtype.UUID
	latestName := ""
	latest, err := q.LatestFunctionForProject(ctx, projectUUID)
	switch {
	case err == nil:
		latestID = latest.ID
		latestName = latest.Name
	case errors.Is(err, pgx.ErrNoRows):
		err = nil
	default:
		return nil, err
	}

	if _, err := h.agentQueries().UpdateAgentChatSync(ctx, agentadaptor.UpdateAgentChatSyncParams{
		ID:                 chatID,
		Functions:          functionsJSON,
		LatestFunctionID:   latestID,
		LatestFunctionName: latestName,
	}); err != nil {
		return nil, err
	}

	if latestID.Valid {
		return &latest, nil
	}
	return nil, nil
}

func (h *AgentHandlers) toolLoop(ctx context.Context, done <-chan struct{}, chatID pgtype.UUID, sessionID string, tracker *chatTracker) {
	interval := time.Duration(config.Cfg.AgentPollSeconds) * time.Second
	if interval <= 0 {
		interval = 3 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-done:
			return
		case <-ticker.C:
			h.pollTools(ctx, chatID, sessionID, tracker)
		}
	}
}

// pollTools refreshes the transcript and stores any tool calls the agent has
// made so far.
func (h *AgentHandlers) pollTools(ctx context.Context, chatID pgtype.UUID, sessionID string, tracker *chatTracker) []agent.ToolCall {
	messages, err := h.state.Agent.Client().ListMessages(ctx, sessionID)
	if err != nil {
		if ctx.Err() == nil {
			slog.Debug("agent activity poll failed", "session", sessionID, "error", err)
		}
		return nil
	}

	var calls []agent.ToolCall
	for _, message := range messages {
		messageCalls := message.ToolCalls()
		if len(messageCalls) > 0 {
			calls = append(calls, messageCalls...)
		}
	}
	tracker.add(calls)
	h.saveMessages(ctx, chatID, sessionID, messages)
	return calls
}

type chatTracker struct {
	mu  sync.Mutex
	ids map[string]struct{}
}

func (t *chatTracker) add(calls []agent.ToolCall) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, call := range calls {
		if call.ID != "" {
			t.ids[call.ID] = struct{}{}
		}
	}
}

func (t *chatTracker) count() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.ids)
}

// Chat returns the status, tool calls, functions, and latest build target for
// one chat turn. The UI polls this while the agent works.
func (h *AgentHandlers) Chat(c *gin.Context) {
	if !h.historyAvailable() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "agent history store is not configured"})
		return
	}
	chatID, err := parseUUIDHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid chat id"})
		return
	}
	chat, err := h.agentQueries().GetAgentChat(c.Request.Context(), chatID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "chat not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load chat"})
		return
	}
	if !h.authorizeProject(c, chat.ProjectID) {
		return
	}

	calls, err := h.agentQueries().ListAgentToolCallsForChat(c.Request.Context(), chatID)
	if err != nil {
		slog.Warn("agent tool calls read failed", "chat", c.Param("id"), "error", err)
	}

	functions := chat.Functions
	if len(functions) == 0 {
		functions = []byte("[]")
	}
	var latest any
	if chat.LatestFunctionID.Valid {
		latest = gin.H{
			"id":   uuidHex(chat.LatestFunctionID),
			"name": chat.LatestFunctionName,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"id":              uuidHex(chat.ID),
		"session_id":      chat.SessionID,
		"status":          chat.Status,
		"prompt":          chat.Prompt,
		"reply":           chat.Reply,
		"error":           chat.Error,
		"tool_calls":      toolCallResponses(calls),
		"functions":       json.RawMessage(functions),
		"latest_function": latest,
		"last_synced_at":  timestampString(chat.LastSyncedAt),
		"started_at":      timestampString(chat.StartedAt),
		"finished_at":     timestampString(chat.FinishedAt),
	})
}

// History returns the transcript and tool calls stored in the portal database.
func (h *AgentHandlers) History(c *gin.Context) {
	if !h.historyAvailable() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "agent history store is not configured"})
		return
	}
	sessionID := c.Param("id")
	session, err := h.agentQueries().GetAgentSession(c.Request.Context(), sessionID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load session"})
		return
	}
	if !h.authorizeProject(c, session.ProjectID) {
		return
	}
	messages, err := h.agentQueries().ListAgentMessages(c.Request.Context(), sessionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load agent history"})
		return
	}
	calls, err := h.agentQueries().ListAgentToolCallsForSession(c.Request.Context(), sessionID)
	if err != nil {
		slog.Warn("agent tool calls read failed", "session", sessionID, "error", err)
	}

	out := make([]gin.H, 0, len(messages))
	for _, message := range messages {
		parts := message.Parts
		if len(parts) == 0 {
			parts = []byte("[]")
		}
		out = append(out, gin.H{
			"id":         message.ID,
			"role":       message.Role,
			"text":       message.Text,
			"parts":      json.RawMessage(parts),
			"model":      textString(message.Model),
			"provider":   textString(message.Provider),
			"error":      textString(message.Error),
			"created_at": timestampString(message.CreatedAt),
		})
	}

	var chatResponse any
	if chats, err := h.agentQueries().ListAgentChatsForSession(c.Request.Context(), agentadaptor.ListAgentChatsForSessionParams{
		SessionID: sessionID,
		Limit:     1,
	}); err == nil && len(chats) > 0 {
		chat := chats[0]
		functions := chat.Functions
		if len(functions) == 0 {
			functions = []byte("[]")
		}
		chatResponse = gin.H{
			"id":             uuidHex(chat.ID),
			"status":         chat.Status,
			"prompt":         chat.Prompt,
			"reply":          chat.Reply,
			"error":          chat.Error,
			"tool_calls":     chat.ToolCalls,
			"functions":      json.RawMessage(functions),
			"last_synced_at": timestampString(chat.LastSyncedAt),
			"started_at":     timestampString(chat.StartedAt),
			"finished_at":    timestampString(chat.FinishedAt),
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"session_id": sessionID,
		"messages":   out,
		"tool_calls": toolCallResponses(calls),
		"chat":       chatResponse,
	})
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
	projectUUID := c.MustGet("projectUUID").(pgtype.UUID)
	projectName := c.MustGet("projectName").(string)
	userName := c.GetString("userName")
	session, err := h.state.Agent.CreateSession(c.Request.Context(), agent.SessionRequest{
		Title:     req.Title,
		Project:   projectName,
		ProjectID: projectUUID.String(),
		UserName:  userName,
		Directory: req.Directory,
	})
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	if h.historyAvailable() {
		directory := strings.TrimSpace(req.Directory)
		if directory == "" {
			directory = "/workspace"
		}
		if _, err := h.agentQueries().UpsertAgentSession(c.Request.Context(), agentadaptor.UpsertAgentSessionParams{
			ID:        session.ID,
			ProjectID: projectUUID,
			UserID:    c.MustGet("userID").([]byte),
			Title:     session.Title,
			Directory: directory,
			Model:     config.Cfg.AgentModel,
		}); err != nil {
			slog.Warn("agent session store failed", "session", session.ID, "error", err)
		}
	}
	c.JSON(http.StatusCreated, session)
}

// Messages returns the live transcript for a session from OpenCode.
func (h *AgentHandlers) Messages(c *gin.Context) {
	if !h.available() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "agent is not configured; deploy the opencode service"})
		return
	}
	sessionID := c.Param("id")
	session, err := h.agentQueries().GetAgentSession(c.Request.Context(), sessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
		return
	}
	if !h.authorizeProject(c, session.ProjectID) {
		return
	}
	messages, err := h.state.Agent.Messages(c.Request.Context(), sessionID)
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

type functionSummary struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Language  string `json:"language"`
	Path      string `json:"path"`
	IsAsync   bool   `json:"is_async"`
	CreatedAt string `json:"created_at"`
}

func sortFunctionsNewestFirst(functions []functionadaptors.Function) {
	for i := 1; i < len(functions); i++ {
		for j := i; j > 0 && functions[j].CreatedAt.Time.After(functions[j-1].CreatedAt.Time); j-- {
			functions[j], functions[j-1] = functions[j-1], functions[j]
		}
	}
}

func toolCallResponses(calls []agentadaptor.AgentToolCall) []gin.H {
	out := make([]gin.H, 0, len(calls))
	for _, call := range calls {
		input := call.Input
		if len(input) == 0 {
			input = []byte("null")
		}
		out = append(out, gin.H{
			"id":           call.ID,
			"message_id":   call.MessageID,
			"tool":         call.Tool,
			"status":       call.Status,
			"title":        call.Title,
			"input":        json.RawMessage(input),
			"output":       call.Output,
			"error":        call.Error,
			"started_at":   timestampString(call.StartedAt),
			"completed_at": timestampString(call.CompletedAt),
		})
	}
	return out
}

func parseUUIDHex(value string) (pgtype.UUID, error) {
	raw, err := hex.DecodeString(strings.TrimSpace(value))
	if err != nil || len(raw) != 16 {
		return pgtype.UUID{}, errors.New("invalid uuid")
	}
	var id pgtype.UUID
	id.Valid = true
	copy(id.Bytes[:], raw)
	return id, nil
}

func uuidHex(id pgtype.UUID) string {
	if !id.Valid {
		return ""
	}
	return hex.EncodeToString(id.Bytes[:])
}

func timestampString(ts pgtype.Timestamptz) any {
	if !ts.Valid {
		return nil
	}
	return timeValue(ts)
}

func timeValue(ts pgtype.Timestamptz) string {
	if !ts.Valid {
		return ""
	}
	return ts.Time.UTC().Format(time.RFC3339Nano)
}

func textString(value pgtype.Text) string {
	if !value.Valid {
		return ""
	}
	return value.String
}
