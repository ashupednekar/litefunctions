package pkg

import (
	"context"
	"github.com/ashupednekar/litefunctions/common/vcs/internal/vendors"
)

type CreateRepoOptions = vendors.CreateRepoOptions
type Repository = vendors.Repository
type WebhookOptions = vendors.WebhookOptions
type Webhook = vendors.Webhook
type ActionsProgressOptions = vendors.ActionsProgressOptions
type ActionsProgress = vendors.ActionsProgress
type WorkflowRun = vendors.WorkflowRun

type VendorClient interface {
	CreateRepo(ctx context.Context, opts vendors.CreateRepoOptions) (*vendors.Repository, error)
	DeleteRepo(ctx context.Context, owner, repo string) error
	AddWebhook(ctx context.Context, owner, repo string, opts vendors.WebhookOptions) (*vendors.Webhook, error)
	AddWorkflow(project string) error
	GetActionsProgress(ctx context.Context, owner, repo string, opts vendors.ActionsProgressOptions) (*vendors.ActionsProgress, error)
}

func NewVendorClient(cfg Config) (VendorClient, error) {
	return vendors.NewClient(&cfg)
}
