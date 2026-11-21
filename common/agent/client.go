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

// Client talks to an OpenCode server, preferring the legacy /session routes
// and falling back to /api/session when the server does not expose them.
type Client struct {
	cfg  Config
	http *http.Client
	mode atomic.Int32
}

// NewClient builds a client. A non-positive timeout falls back to five minutes.
func NewClient(cfg Config) *Client {
	timeout := time.Duration(cfg.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	return &Client{cfg: cfg, http: &http.Client{Timeout: timeout}}
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

// withFallback tries the legacy route first, then the V2 route if the legacy
// one is unsupported. The first mode that succeeds is reused by later calls.
func withFallback[T any](c *Client, ctx context.Context, legacy, v2 func(context.Context) (T, error)) (T, error) {
	var zero T
	switch apiMode(c.mode.Load()) {
	case modeLegacy:
		return legacy(ctx)
	case modeV2:
		return v2(ctx)
	}
	result, err := legacy(ctx)
	if err == nil {
		c.mode.Store(int32(modeLegacy))
		return result, nil
	}
	if !isUnsupported(err) {
		return zero, err
	}
	result, err = v2(ctx)
	if err == nil {
		c.mode.Store(int32(modeV2))
	}
	return result, err
}

type createSessionBody struct {
	Title     string `json:"title"`
	Directory string `json:"directory,omitempty"`
}

func (c *Client) CreateSession(ctx context.Context, title string) (*Session, error) {
	return c.CreateSessionAt(ctx, title, "")
}

func (c *Client) CreateSessionAt(ctx context.Context, title, directory string) (*Session, error) {
	body := createSessionBody{Title: title, Directory: directory}
	return withFallback(c, ctx,
		func(ctx context.Context) (*Session, error) { return c.createSession(ctx, "/session", body) },
		func(ctx context.Context) (*Session, error) { return c.createSessionV2(ctx, body) },
	)
}

func (c *Client) createSession(ctx context.Context, path string, body createSessionBody) (*Session, error) {
	var session Session
	if err := c.doJSON(ctx, http.MethodPost, path, body, &session); err != nil {
		return nil, err
	}
	return &session, nil
}

func (c *Client) createSessionV2(ctx context.Context, body createSessionBody) (*Session, error) {
	var raw v2Session
	if err := c.doJSON(ctx, http.MethodPost, "/api/session", body, &raw); err != nil {
		return nil, err
	}
	session := raw.toSession()
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

func (s v2Session) toSession() Session {
	return Session{
		ID:        s.ID,
		Slug:      s.Slug,
		Title:     s.Title,
		Directory: s.Directory,
		ProjectID: s.ProjectID,
		Version:   s.Version,
		CreatedAt: s.Time.Created,
		UpdatedAt: s.Time.Updated,
	}
}

func (c *Client) ListSessions(ctx context.Context) ([]Session, error) {
	return withFallback(c, ctx,
		func(ctx context.Context) ([]Session, error) {
			var sessions []Session
			err := c.doJSON(ctx, http.MethodGet, "/session", nil, &sessions)
			return sessions, err
		},
		c.listSessionsV2,
	)
}

func (c *Client) listSessionsV2(ctx context.Context) ([]Session, error) {
	var response struct {
		Data []v2Session `json:"data"`
	}
	if err := c.doJSON(ctx, http.MethodGet, "/api/session", nil, &response); err != nil {
		return nil, err
	}
	out := make([]Session, len(response.Data))
	for i, item := range response.Data {
		out[i] = item.toSession()
	}
	return out, nil
}

func (c *Client) DeleteSession(ctx context.Context, sessionID string) error {
	err := c.doJSON(ctx, http.MethodDelete, legacyPath(sessionID, ""), nil, nil)
	if err == nil || !isUnsupported(err) {
		return err
	}
	return c.doJSON(ctx, http.MethodDelete, v2Path(sessionID, ""), nil, nil)
}

// SendMessage sends a text prompt and returns the assistant reply. Legacy
// servers answer synchronously; V2 servers admit the prompt and require a wait
// followed by a message read, which this method handles.
func (c *Client) SendMessage(ctx context.Context, sessionID, text string) (*Message, error) {
	return withFallback(c, ctx,
		func(ctx context.Context) (*Message, error) { return c.sendMessageLegacy(ctx, sessionID, text) },
		func(ctx context.Context) (*Message, error) { return c.sendMessageV2(ctx, sessionID, text) },
	)
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
	path := v2Path(sessionID, "/prompt")
	err := c.doJSON(ctx, http.MethodPost, path, promptBody{
		Parts: []Part{{Type: "text", Text: text}},
	}, nil)
	if isBadRequest(err) {
		// Some V2 builds only accept a plain text prompt.
		err = c.doJSON(ctx, http.MethodPost, path, map[string]string{"text": text}, nil)
	}
	if err != nil {
		return nil, err
	}

	// The wait route is experimental and may be missing; the message read below
	// still returns whatever the agent has produced by then.
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

func (c *Client) ListMessages(ctx context.Context, sessionID string) ([]Message, error) {
	return withFallback(c, ctx,
		func(ctx context.Context) ([]Message, error) {
			var messages []Message
			err := c.doJSON(ctx, http.MethodGet, legacyPath(sessionID, "/message"), nil, &messages)
			return messages, err
		},
		func(ctx context.Context) ([]Message, error) { return c.listMessagesV2(ctx, sessionID) },
	)
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
	out := make([]Message, len(response.Data))
	for i, item := range response.Data {
		out[i] = v2ToMessage(item)
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
		case "assistant", "user":
			role = item.Type
		}
	}
	msg := Message{Info: MessageInfo{ID: item.ID, Role: role, Error: item.Error}}
	for _, content := range item.Content {
		msg.Parts = append(msg.Parts, Part{ID: item.ID, Type: content.Type, Text: content.Text})
	}
	if len(msg.Parts) == 0 && item.Text != "" {
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
	Message    string
}

func (e *httpError) Error() string { return e.Message }

func newHTTPError(status int, payload []byte) error {
	return &httpError{
		StatusCode: status,
		Message:    fmt.Sprintf("opencode returned HTTP %d: %s", status, httpErrorMessage(status, payload)),
	}
}

// httpErrorMessage pulls the useful parts out of OpenCode error payloads. They
// come in a few shapes: {"message": ...}, {"name": ..., "data": {"message",
// "ref"}}, or an empty body (usually an auth failure or a missing route).
func httpErrorMessage(status int, payload []byte) string {
	var body struct {
		Name    string `json:"name"`
		Message string `json:"message"`
		Data    struct {
			Message string `json:"message"`
			Ref     string `json:"ref"`
		} `json:"data"`
	}
	if json.Unmarshal(payload, &body) == nil {
		switch {
		case body.Data.Message != "" && body.Data.Ref != "":
			return body.Data.Message + " (" + body.Data.Ref + ")"
		case body.Data.Message != "":
			return body.Data.Message
		case body.Message != "":
			return body.Message
		case body.Name != "":
			return body.Name
		}
	}
	text := strings.TrimSpace(string(payload))
	if text == "" {
		return http.StatusText(status)
	}
	if len(text) > 512 {
		text = text[:512]
	}
	return text
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
