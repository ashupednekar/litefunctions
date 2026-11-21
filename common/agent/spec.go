// Package agent is the shared integration layer between LiteFunctions and an
// OpenCode server. Portal uses it to give every screen a conversational agent,
// while the agent uses the LiteFunctions MCP server to write functions back to
// the project repository.
package agent

import (
	"encoding/json"
	"time"
)

// Config describes how to reach an OpenCode server.
type Config struct {
	BaseURL        string `env:"OPENCODE_URL" default:"http://litefunctions-opencode:4096"`
	Username       string `env:"OPENCODE_USERNAME" default:"opencode"`
	Password       string `env:"OPENCODE_PASSWORD"`
	Model          string `env:"OPENCODE_MODEL"`
	Agent          string `env:"OPENCODE_AGENT" default:"build"`
	TimeoutSeconds int    `env:"OPENCODE_TIMEOUT_SECONDS" default:"300"`
}

// Session is a conversation with the OpenCode agent.
type Session struct {
	ID        string `json:"id"`
	Slug      string `json:"slug"`
	ProjectID string `json:"projectID"`
	Directory string `json:"directory"`
	Title     string `json:"title"`
	Version   string `json:"version"`
	CreatedAt int64  `json:"created"`
	UpdatedAt int64  `json:"updated"`
}

// Part is one element of a message. OpenCode uses several part types; only text
// parts are surfaced to users.
type Part struct {
	ID        string `json:"id,omitempty"`
	SessionID string `json:"sessionID,omitempty"`
	MessageID string `json:"messageID,omitempty"`
	Type      string `json:"type"`
	Text      string `json:"text,omitempty"`
}

// Message is an OpenCode message with its ordered parts.
type Message struct {
	Info  MessageInfo `json:"info"`
	Parts []Part      `json:"parts"`
}

// MessageInfo is the metadata attached to a message.
type MessageInfo struct {
	ID        string    `json:"id"`
	SessionID string    `json:"sessionID"`
	Role      string    `json:"role"`
	Agent     string    `json:"agent"`
	Mode      string    `json:"mode"`
	Provider  string    `json:"providerID"`
	Model     string    `json:"modelID"`
	Error     *APIError `json:"error,omitempty"`
}

// APIError is the structured error OpenCode returns for failed messages.
type APIError struct {
	Name    string          `json:"name"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func (e *APIError) Error() string {
	if e == nil {
		return ""
	}
	if e.Message != "" {
		return e.Message
	}
	return e.Name
}

// Text returns the concatenated text parts of a message.
func (m Message) Text() string {
	out := ""
	for _, part := range m.Parts {
		if part.Type != "text" || part.Text == "" {
			continue
		}
		if out != "" {
			out += "\n\n"
		}
		out += part.Text
	}
	return out
}

// SessionRequest is used when the caller wants to create a conversation
// explicitly. Most callers can leave SessionID empty in a PromptRequest and let
// Service create the session on demand.
type SessionRequest struct {
	Title      string
	Project    string
	ProjectID  string
	UserName   string
	Directory  string
	SessionKey string
}

// PromptRequest asks the agent to work on a project.
type PromptRequest struct {
	SessionID string
	Title     string
	Project   string
	ProjectID string
	UserName  string
	Directory string
	Text      string
}

// PromptResult is the assistant's reply plus the session it belongs to.
type PromptResult struct {
	SessionID string  `json:"session_id"`
	Text      string  `json:"text"`
	Message   Message `json:"message"`
}

// Conversation is a lightweight view of a session used by the portal.
type Conversation struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Tool-event metadata is forwarded by OpenCode through MCP calls. It is kept
// here so callers can correlate prompts with MCP writes when they need to.
const SessionMetaKey = "ai.opencode/sessionID"
