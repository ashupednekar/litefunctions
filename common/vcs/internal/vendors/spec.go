package vendors

import (
	"context"
	"fmt"

	"github.com/ashupednekar/litefunctions/common/vcs"
)

type VendorClient interface {
	CreateRepo(ctx context.Context, opts CreateRepoOptions) (*Repository, error)
	DeleteRepo(ctx context.Context, owner, repo string) error
	AddWebhook(ctx context.Context, owner, repo string, opts WebhookOptions) (*Webhook, error)
	AddWorkflow(project string) error
	GetActionsProgress(ctx context.Context, owner, repo string, opts ActionsProgressOptions) (*ActionsProgress, error)
}

type CreateRepoOptions struct {
	Name          string
	Description   string
	Private       bool
	AutoInit      bool
	DefaultBranch string
}

type Repository struct {
	ID            int64
	Name          string
	FullName      string
	Description   string
	Private       bool
	HTMLURL       string
	CloneURL      string
	SSHURL        string
	DefaultBranch string
	CreatedAt     string
	UpdatedAt     string
}

type WebhookOptions struct {
	URL         string
	ContentType string
	Secret      string
	Events      []string
	Active      bool
	InsecureSSL bool
}

type Webhook struct {
	ID        int64
	URL       string
	Events    []string
	Active    bool
	CreatedAt string
	UpdatedAt string
}

type ActionsProgressOptions struct {
	Branch     string
	Status     string
	Event      string
	WorkflowID string
	Limit      int
}

type ActionsProgress struct {
	TotalCount int
	Runs       []WorkflowRun
}

type WorkflowRun struct {
	ID           int64
	Name         string
	DisplayTitle string
	Status       string
	Conclusion   string
	Branch       string
	Event        string
	CreatedAt    string
	UpdatedAt    string
	HTMLURL      string
	WorkflowID   int64
	WorkflowName string
	FunctionName string
	CurrentJob   string
	CurrentStep  string
}

func NewClient(cfg vcs.Config) (VendorClient, error) {
	switch cfg.Vendor {
	case "github":
		if cfg.Token == "" {
			return nil, fmt.Errorf("token is required for GitHub")
		}
		return NewGitHubClient(cfg), nil
	case "gitea":
		if cfg.Token == "" {
			return nil, fmt.Errorf("token is required for Gitea")
		}
		if cfg.BaseURL == "" {
			return nil, fmt.Errorf("baseURL is required for Gitea")
		}
		return NewGiteaClient(cfg), nil

	default:
		return nil, fmt.Errorf("unsupported vendor type: %s (supported: github, gitea)", cfg.Vendor)
	}
}
