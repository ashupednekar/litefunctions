package agent

import (
	"context"
	"fmt"
	"strings"

	"go-simpler.org/env"
)

// LoadConfig reads agent settings from the process environment.
func LoadConfig() (Config, error) {
	var cfg Config
	if err := env.Load(&cfg, nil); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// Service adds LiteFunctions' project conventions on top of the raw OpenCode
// client. It is the interface portal depends on.
type Service struct {
	client *Client
	cfg    Config
}

// NewService builds a service. A blank BaseURL produces a disabled service
// whose calls fail with a helpful message instead of panicking.
func NewService(cfg Config) *Service {
	return &Service{client: NewClient(cfg), cfg: cfg}
}

// Client exposes the lower-level client for callers that need raw routes.
func (s *Service) Client() *Client { return s.client }

// Enabled reports whether a server URL was configured.
func (s *Service) Enabled() bool {
	return s != nil && strings.TrimSpace(s.client.Config().BaseURL) != ""
}

// CreateSession starts a conversation for a project.
func (s *Service) CreateSession(ctx context.Context, req SessionRequest) (*Session, error) {
	if !s.Enabled() {
		return nil, fmt.Errorf("agent server is not configured")
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		switch {
		case req.Project != "" && req.UserName != "":
			title = fmt.Sprintf("%s · %s", req.Project, req.UserName)
		case req.Project != "":
			title = req.Project
		default:
			title = "LiteFunctions agent"
		}
	}
	return s.client.CreateSessionAt(ctx, title, req.Directory)
}

// Prompt sends a project-aware prompt and returns the assistant reply.
func (s *Service) Prompt(ctx context.Context, req PromptRequest) (*PromptResult, error) {
	if !s.Enabled() {
		return nil, fmt.Errorf("agent server is not configured")
	}
	prompt := strings.TrimSpace(req.Text)
	if prompt == "" {
		return nil, fmt.Errorf("prompt is required")
	}

	sessionID := strings.TrimSpace(req.SessionID)
	if sessionID == "" {
		session, err := s.CreateSession(ctx, SessionRequest{
			Title:     req.Title,
			Project:   req.Project,
			ProjectID: req.ProjectID,
			UserName:  req.UserName,
			Directory: req.Directory,
		})
		if err != nil {
			return nil, err
		}
		sessionID = session.ID
	}

	msg, err := s.client.SendMessage(ctx, sessionID, buildPrompt(req, prompt))
	if err != nil {
		return nil, err
	}
	return &PromptResult{
		SessionID: sessionID,
		Text:      msg.Text(),
		Message:   *msg,
	}, nil
}

// Sessions lists server-side conversations.
func (s *Service) Sessions(ctx context.Context) ([]Session, error) {
	if !s.Enabled() {
		return nil, fmt.Errorf("agent server is not configured")
	}
	return s.client.ListSessions(ctx)
}

// Messages lists the messages in a conversation.
func (s *Service) Messages(ctx context.Context, sessionID string) ([]Message, error) {
	if !s.Enabled() {
		return nil, fmt.Errorf("agent server is not configured")
	}
	return s.client.ListMessages(ctx, sessionID)
}

// DeleteSession removes a conversation.
func (s *Service) DeleteSession(ctx context.Context, sessionID string) error {
	if !s.Enabled() {
		return fmt.Errorf("agent server is not configured")
	}
	return s.client.DeleteSession(ctx, sessionID)
}

func buildPrompt(req PromptRequest, prompt string) string {
	project := strings.TrimSpace(req.Project)
	if project == "" {
		project = "unknown"
	}
	projectID := strings.TrimSpace(req.ProjectID)
	if projectID == "" {
		projectID = "unknown"
	}
	dir := strings.TrimSpace(req.Directory)
	if dir == "" {
		dir = "/workspace"
	}

	var b strings.Builder
	b.WriteString("Project: ")
	b.WriteString(project)
	b.WriteString("\nProject ID: ")
	b.WriteString(projectID)
	b.WriteString("\nWorkspace: ")
	b.WriteString(dir)
	b.WriteString("\n\n")
	b.WriteString("Follow the mounted AGENTS.md for LiteFunctions behavior, language ")
	b.WriteString("selection, async decisions, and supported artefacts. ")
	b.WriteString("Use the `litefunctions` MCP server for every project read/write. ")
	b.WriteString("Always pass project=\"")
	b.WriteString(project)
	b.WriteString("\".\n\n")
	b.WriteString("Databases, persistent streams, object storage, traffic policies, and ")
	b.WriteString("scheduled jobs are not supported yet; say so if asked.\n\n")
	b.WriteString("User request:\n")
	b.WriteString(prompt)
	return b.String()
}
