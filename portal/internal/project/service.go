// Package project owns project provisioning shared by the portal and the MCP
// server's internal API.
package project

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	vcs "github.com/ashupednekar/litefunctions/common/vcs/pkg"
	accessadaptors "github.com/ashupednekar/litefunctions/portal/internal/access/adaptors"
	"github.com/ashupednekar/litefunctions/portal/internal/auth"
	authadaptors "github.com/ashupednekar/litefunctions/portal/internal/auth/adaptors"
	"github.com/ashupednekar/litefunctions/portal/internal/config"
	adaptors "github.com/ashupednekar/litefunctions/portal/internal/project/adaptors"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

var namePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,62}$`)

// NormalizeName lowercases and validates a project name.
func NormalizeName(name string) (string, error) {
	name = strings.ToLower(strings.TrimSpace(name))
	if !namePattern.MatchString(name) {
		return "", fmt.Errorf("invalid project name %q", name)
	}
	return name, nil
}

// EnsureSystemUser creates the non-interactive user that owns projects
// bootstrapped by the operator or the MCP server.
func EnsureSystemUser(ctx context.Context, pool *pgxpool.Pool) error {
	return authadaptors.New(pool).CreateUser(ctx, authadaptors.CreateUserParams{
		ID:          auth.SystemUserID,
		Name:        auth.SystemUserName,
		DisplayName: "LiteFunctions",
	})
}

// Create provisions the database rows, repository, webhook and workflow for a
// project. It does not sync function metadata; callers do that when they have a
// request context suitable for a repo clone.
func Create(ctx context.Context, pool *pgxpool.Pool, name string, userID []byte, vcsCfg vcs.Config) (adaptors.Project, error) {
	name, err := NormalizeName(name)
	if err != nil {
		return adaptors.Project{}, err
	}
	if err := EnsureSystemUser(ctx, pool); err != nil {
		return adaptors.Project{}, fmt.Errorf("ensure system user: %w", err)
	}

	client, err := vcs.NewVendorClient(vcsCfg)
	if err != nil {
		return adaptors.Project{}, fmt.Errorf("init vcs client: %w", err)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return adaptors.Project{}, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	projects := adaptors.New(pool).WithTx(tx)
	access := accessadaptors.New(pool).WithTx(tx)

	project, err := projects.CreateProject(ctx, adaptors.CreateProjectParams{
		Name:        name,
		Description: pgtype.Text{Valid: false},
		CreatedBy:   userID,
	})
	if err != nil {
		return adaptors.Project{}, fmt.Errorf("create project: %w", err)
	}
	if err := access.AddProjectOwner(ctx, accessadaptors.AddProjectOwnerParams{
		UserID:    userID,
		ProjectID: project.ID,
	}); err != nil {
		return adaptors.Project{}, fmt.Errorf("assign project owner: %w", err)
	}

	repo, err := client.CreateRepo(ctx, vcs.CreateRepoOptions{
		Name:        name,
		Description: "Created by LiteFunctions",
		Private:     vcsCfg.UsePrivateRepo,
		AutoInit:    true,
	})
	if err != nil && !repoExists(err) {
		return adaptors.Project{}, fmt.Errorf("create repository: %w", err)
	}
	repoName := name
	if repo != nil {
		repoName = repo.Name
	}

	webhookURL := fmt.Sprintf("https://%s/api/webhooks/vcs", config.Cfg.Fqdn)
	if _, err := client.AddWebhook(ctx, vcsCfg.User, repoName, vcs.WebhookOptions{
		URL:         webhookURL,
		ContentType: "json",
		Secret:      "WEBHOOK_SUPER_SECRET",
		Events:      []string{"push"},
		Active:      true,
		InsecureSSL: true,
	}); err != nil && !repoExists(err) {
		return adaptors.Project{}, fmt.Errorf("add webhook: %w", err)
	}
	if err := client.AddWorkflow(repoName); err != nil {
		return adaptors.Project{}, fmt.Errorf("add workflow: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return adaptors.Project{}, fmt.Errorf("commit project: %w", err)
	}
	return project, nil
}

func repoExists(err error) bool {
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "already exists") || strings.Contains(message, "409")
}
