package vcs

import (
	"context"

	"github.com/ashupednekar/litefunctions/common/vcs"
	"github.com/ashupednekar/litefunctions/common/vcs/internal/vendors"
)

type VendorClient interface {
	CreateRepo(ctx context.Context, opts vendors.CreateRepoOptions) (*vendors.Repository, error)
	DeleteRepo(ctx context.Context, owner, repo string) error
	AddWebhook(ctx context.Context, owner, repo string, opts vendors.WebhookOptions) (*vendors.Webhook, error)
	AddWorkflow(project string) error
	GetActionsProgress(ctx context.Context, owner, repo string, opts vendors.ActionsProgressOptions) (*vendors.ActionsProgress, error)
}

func NewVendorClient(cfg vcs.Config) (VendorClient, error) {
	return vendors.NewClient(cfg)
}
