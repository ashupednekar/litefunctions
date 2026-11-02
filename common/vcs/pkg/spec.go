package vcs

import (
	"context"

	"github.com/ashupednekar/litefunctions/common/vcs/internal/config"
	"github.com/ashupednekar/litefunctions/common/vcs/internal/repo"
	"github.com/ashupednekar/litefunctions/common/vcs/internal/vendors"
	"github.com/go-git/go-billy/v6"
)

type Config = config.Config

type Repo interface {
	ProjectName() string
	Filesystem() billy.Filesystem
	Clone() error
	Commit(path string) error
	CommitMessage(path, message string) error
	Push() error
	Pull() error
}

type VendorClient interface {
	CreateRepo(ctx context.Context, opts CreateRepoOptions) (*Repository, error)
	DeleteRepo(ctx context.Context, owner, repo string) error
	AddWebhook(ctx context.Context, owner, repo string, opts WebhookOptions) (*Webhook, error)
	AddWorkflow(project string) error
	GetActionsProgress(ctx context.Context, owner, repo string, opts ActionsProgressOptions) (*ActionsProgress, error)
}

type (
	CreateRepoOptions      = vendors.CreateRepoOptions
	Repository             = vendors.Repository
	WebhookOptions         = vendors.WebhookOptions
	Webhook                = vendors.Webhook
	ActionsProgressOptions = vendors.ActionsProgressOptions
	ActionsProgress        = vendors.ActionsProgress
	WorkflowRun            = vendors.WorkflowRun
)

func NewRepo(cfg Config, project string, branch *string) (Repo, error) {
	return repo.New(cfg, project, branch)
}

func NewVendorClient(cfg Config) (VendorClient, error) {
	return vendors.NewClient(cfg)
}

func EnsureFile(cfg Config, project, path string, body []byte) (bool, error) {
	return repo.EnsureFile(cfg, project, path, body)
}

func WriteFile(cfg Config, project, path string, body []byte, commitMessage string) error {
	return repo.WriteFile(cfg, project, path, body, commitMessage)
}
