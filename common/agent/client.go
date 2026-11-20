package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"
)

type apiMode int32

const (
	modeAuto apiMode = iota
	modeLegacy
	modeV2
)

// Client is a small typed client for the OpenCode HTTP API. It supports both
// the legacy /session routes and the newer /api/session routes so the portal
// keeps working across OpenCode releases.
type Client struct {
	cfg  Config
	http *http.Client
	mode atomic.Int32
}

// NewClient builds a client. A zero timeout falls back to five minutes.
func NewClient(cfg Config) *Client {
	timeout := time.Duration(cfg.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	return &Client{
		cfg:  cfg,
		http: &http.Client{Timeout: timeout},
	}
}

func (c *Client) Config() Config { return c.cfg }

func (c *Client) baseURL() string {
	return strings.TrimRight(strings.TrimSpace(c.cfg.BaseURL), "/")
}

func (c *Client) username() string {
	if strings.TrimSpace(c.cfg.Username) == "" {
		return "opencode"
	}
	return c.cfg.Username
}

type createSessionBody struct {
	Title     string `json:"title"`
	Directory string `json:"directory,omitempty"`
}

// CreateSession creates a new conversation.
func (c *Client) CreateSession(ctx context.Context, title string) (*Session, error) {
	return c.CreateSessionAt(ctx, title, "")
}

// CreateSessionAt creates a session, optionally pinned to a directory.
func (c *Client) CreateSessionAt(ctx context.Context, title, directory string) (*Session, error) {
	mode := apiMode(c.mode.Load())
	if mode == modeLegacy {
		return c.createSessionLegacy(ctx, title, directory)
	}
	if mode == modeV2 {
		return c.createSessionV2(ctx, title, directory)
	}

	session, err := c.createSessionLegacy(ctx, title, directory)
	if err == nil {
		c.mode.Store(int32(modeLegacy))
		return session, nil
	}
	if !isUnsupported(err) {
		return nil, err
	}
	session, err = c.createSessionV2(ctx, title, directory)
	if err == nil {
		c.mode.Store(int32(modeV2))
	}
	return session, err
}

func (c *Client) createSessionLegacy(ctx context.Context, title, directory string) (*Session, error) {
	var session Session
	if err := c.doJSON(ctx, http.MethodPost, "/session", createSessionBody{Title: title, Directory: directory}, &session); err != nil {
		return nil, err
	}
	return &session, nil
}

type v2Session struct {
	ID    string `json:"id"`
	Slug  string `json:"slug"`
	Title string `json:"title"`
	Time  struct {
		Created int64 `json:"created"`
		Updated int64 `json:"updated"`
	} `json:"time"`
	Directory string `json:"directory"`
	ProjectID string `json:"projectID"`
	Version   string `json:"version"`
}

func (c *Client) createSessionV2(ctx context.Context, title, directory string) (*Session, error) {
	var raw v2Session
	if err := c.doJSON(ctx, http.MethodPost, "/api/session", createSessionBody{Title: title, Directory: directory}, &raw); err != nil {
		return nil, err
	}
	return &Session{
		ID:        raw.ID,
		Slug:      raw.Slug,
		Title:     raw.Title,
		Directory: raw.Directory,
		ProjectID: raw.ProjectID,
		Version:   raw.Version,
		CreatedAt: raw.Time.Created,
		UpdatedAt: raw.Time.Updated,
	}, nil
}

// ListSessions returns sessions known to the server.
func (c *Client) ListSessions(ctx context.Context) ([]Session, error) {
	mode := apiMode(c.mode.Load())
	if mode == modeV2 {
		return c.listSessionsV2(ctx)
	}
	var sessions []Session
	err := c.doJSON(ctx, http.MethodGet, "/session", nil, &sessions)
	if err == nil {
		return sessions, nil
	}
	if !isUnsupported(err) {
		return nil, err
	}
	return c.listSessionsV2(ctx)
}

func (c *Client) listSessionsV2(ctx context.Context) ([]Session, error) {
	var response struct {
		Data []v2Session `json:"data"`
	}
	if err := c.doJSON(ctx, http.MethodGet, "/api/session", nil, &response); err != nil {
		return nil, err
	}
	out := make([]Session, 0, len(response.Data))
	for _, item := range response.Data {
		out = append(out, Session{
			ID:        item.ID,
			Slug:      item.Slug,
			Title:     item.Title,
			Directory: item.Directory,
			ProjectID: item.ProjectID,
			Version:   item.Version,
			CreatedAt: item.Time.Created,
			UpdatedAt: item.Time.Updated,
		})
	}
	return out, nil
}

// DeleteSession removes a conversation.
func (c *Client) DeleteSession(ctx context.Context, sessionID string) error {
	err := c.doJSON(ctx, http.MethodDelete, legacyPath(sessionID, ""), nil, nil)
	if err == nil || !isUnsupported(err) {
		return err
	}
	return c.doJSON(ctx, http.MethodDelete, v2Path(sessionID, ""), nil, nil)
}

// SendMessage sends a text prompt and returns the assistant reply. Legacy
// OpenCode servers answer synchronously. Newer servers admit the prompt and
// require a wait + message read, which this method performs transparently.
func (c *Client) SendMessage(ctx context.Context, sessionID, text string) (*Message, error) {
	mode := apiMode(c.mode.Load())
	if mode == modeV2 {
		return c.sendMessageV2(ctx, sessionID, text)
	}

	msg, err := c.sendMessageLegacy(ctx, sessionID, text)
	if err == nil {
		c.mode.Store(int32(modeLegacy))
		return msg, nil
	}
	if !isUnsupported(err) {
		return nil, err
	}
	msg, err = c.sendMessageV2(ctx, sessionID, text)
	if err == nil {
		c.mode.Store(int32(modeV2))
	}
	return msg, err
}

type promptBody struct {
	Parts []Part `json:"parts"`
}

func (c *Client) sendMessageLegacy(ctx context.Context, sessionID, text string) (*Message, error) {
	var msg Message
	if err := c.doJSON(ctx, http.MethodPost, legacyPath(sessionID, "/message"), promptBody{
		Parts: []Part{{Type: "text", Text: text}},
	}, &msg); err != nil {
		return nil, err
	}
	if msg.Info.Error != nil {
		return nil, msg.Info.Error
	}
	return &msg, nil
}

func (c *Client) sendMessageV2(ctx context.Context, sessionID, text string) (*Message, error) {
	body := promptBody{Parts: []Part{{Type: "text", Text: text}}}
	err := c.doJSON(ctx, http.MethodPost, v2Path(sessionID, "/prompt"), body, nil)
	if err != nil && isBadRequest(err) {
		// Some V2 builds accept a plain text prompt.
		err = c.doJSON(ctx, http.MethodPost, v2Path(sessionID, "/prompt"), map[string]string{"text": text}, nil)
	}
	if err != nil {
		return nil, err
	}

	// Wait for the agent loop to become idle. Older builds may not expose the
	// wait route; in that case the subsequent message read is still useful.
	_ = c.doJSON(ctx, http.MethodPost, "/api/experimental/session/"+url.PathEscape(sessionID)+"/wait", nil, nil)

	msg, err := c.lastAssistantMessageV2(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if msg == nil {
		return &Message{Info: MessageInfo{SessionID: sessionID, Role: "assistant"}}, nil
	}
	return msg, nil
}

// ListMessages returns the messages in a session.
func (c *Client) ListMessages(ctx context.Context, sessionID string) ([]Message, error) {
	mode := apiMode(c.mode.Load())
	if mode == modeV2 {
		return c.listMessagesV2(ctx, sessionID)
	}
	var messages []Message
	err := c.doJSON(ctx, http.MethodGet, legacyPath(sessionID, "/message"), nil, &messages)
	if err == nil {
		return messages, nil
	}
	if !isUnsupported(err) {
		return nil, err
	}
	return c.listMessagesV2(ctx, sessionID)
}

type v2Content struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type v2Message struct {
	ID      string      `json:"id"`
	Type    string      `json:"type"`
	Role    string      `json:"role"`
	Text    string      `json:"text"`
	Content []v2Content `json:"content"`
	Error   *APIError   `json:"error"`
}

type v2MessagesResponse struct {
	Data []v2Message `json:"data"`
}

func (c *Client) listMessagesV2(ctx context.Context, sessionID string) ([]Message, error) {
	var response v2MessagesResponse
	if err := c.doJSON(ctx, http.MethodGet, v2Path(sessionID, "/message"), nil, &response); err != nil {
		return nil, err
	}
	out := make([]Message, 0, len(response.Data))
	for _, item := range response.Data {
		out = append(out, v2ToMessage(item))
	}
	return out, nil
}

func (c *Client) lastAssistantMessageV2(ctx context.Context, sessionID string) (*Message, error) {
	path := v2Path(sessionID, "/message") + "?type=assistant&order=desc&limit=1"
	var response v2MessagesResponse
	if err := c.doJSON(ctx, http.MethodGet, path, nil, &response); err != nil {
		return nil, err
	}
	for _, item := range response.Data {
		msg := v2ToMessage(item)
		if msg.Text() != "" || msg.Info.Error != nil {
			return &msg, nil
		}
	}
	return nil, nil
}

func v2ToMessage(item v2Message) Message {
	role := item.Role
	if role == "" {
		switch item.Type {
		case "assistant":
			role = "assistant"
		case "user":
			role = "user"
		}
	}
	msg := Message{
		Info: MessageInfo{
			ID:        item.ID,
			SessionID: "",
			Role:      role,
			Error:     item.Error,
		},
	}
	if len(item.Content) > 0 {
		for _, content := range item.Content {
			msg.Parts = append(msg.Parts, Part{ID: item.ID, Type: content.Type, Text: content.Text})
		}
	} else if item.Text != "" {
		msg.Parts = append(msg.Parts, Part{ID: item.ID, Type: "text", Text: item.Text})
	}
	return msg
}

func legacyPath(sessionID, suffix string) string {
	return "/session/" + url.PathEscape(sessionID) + suffix
}

func v2Path(sessionID, suffix string) string {
	return "/api/session/" + url.PathEscape(sessionID) + suffix
}

func (c *Client) doJSON(ctx context.Context, method, path string, body any, out any) error {
	base := c.baseURL()
	if base == "" {
		return errors.New("agent server URL is not configured")
	}

	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encode request: %w", err)
		}
		reader = bytes.NewReader(payload)
	}

	req, err := http.NewRequestWithContext(ctx, method, base+path, reader)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.cfg.Password != "" {
		req.SetBasicAuth(c.username(), c.cfg.Password)
	}

	res, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("call opencode: %w", err)
	}
	defer res.Body.Close()

	payload, readErr := io.ReadAll(io.LimitReader(res.Body, 8<<20))
	if readErr != nil {
		return fmt.Errorf("read response: %w", readErr)
	}

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return newHTTPError(res.StatusCode, payload)
	}
	if out == nil || len(bytes.TrimSpace(payload)) == 0 {
		return nil
	}
	if err := json.Unmarshal(payload, out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

type httpError struct {
	StatusCode int
	Body       string
}

func (e *httpError) Error() string {
	if strings.TrimSpace(e.Body) == "" {
		return fmt.Sprintf("opencode returned HTTP %d", e.StatusCode)
	}
	return fmt.Sprintf("opencode returned HTTP %d: %s", e.StatusCode, strings.TrimSpace(e.Body))
}

func newHTTPError(status int, payload []byte) error {
	var apiErr APIError
	if err := json.Unmarshal(payload, &apiErr); err == nil && (apiErr.Message != "" || apiErr.Name != "") {
		return &httpError{StatusCode: status, Body: apiErr.Error()}
	}
	body := string(payload)
	if len(body) > 512 {
		body = body[:512]
	}
	return &httpError{StatusCode: status, Body: body}
}

func isUnsupported(err error) bool {
	var httpErr *httpError
	if !errors.As(err, &httpErr) {
		return false
	}
	switch httpErr.StatusCode {
	case http.StatusNotFound, http.StatusMethodNotAllowed, http.StatusNotImplemented:
		return true
	default:
		return false
	}
}

func isBadRequest(err error) bool {
	var httpErr *httpError
	return errors.As(err, &httpErr) && httpErr.StatusCode == http.StatusBadRequest
}
