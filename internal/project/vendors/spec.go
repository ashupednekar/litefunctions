package vendors

import (
	"context"
	"fmt"
	"github.com/ashupednekar/litewebservices-portal/pkg"
)


type VendorClient interface {
	CreateRepo(ctx context.Context, opts CreateRepoOptions) (*Repository, error)
	DeleteRepo(ctx context.Context, owner, repo string) error
	AddWebhook(ctx context.Context, owner, repo string, opts WebhookOptions) (*Webhook, error)
	AddWorkflow(ctx context.Context, owner, repo string) error
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
	Status       string 
	Conclusion   string 
	Branch       string
	Event        string
	CreatedAt    string
	UpdatedAt    string
	HTMLURL      string
	WorkflowID   int64
	WorkflowName string
}



func NewVendorClient() (VendorClient, error) {
	switch pkg.Cfg.VcsVendor {
	case "github":
		if pkg.Cfg.VcsToken == "" {
			return nil, fmt.Errorf("token is required for GitHub")
		}
		return NewGitHubClient(pkg.Cfg.VcsToken), nil
	case "gitea":
		if pkg.Cfg.VcsToken == "" {
			return nil, fmt.Errorf("token is required for Gitea")
		}
		if pkg.Cfg.VcsBaseUrl == "" {
			return nil, fmt.Errorf("baseURL is required for Gitea")
		}
		return NewGiteaClient(pkg.Cfg.VcsBaseUrl, pkg.Cfg.VcsToken), nil

	default:
		return nil, fmt.Errorf("unsupported vendor type: %s (supported: github, gitea)", pkg.Cfg.VcsVendor)
	}
}
