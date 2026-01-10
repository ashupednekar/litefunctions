package handlers

import (
	"bytes"
	"fmt"
	"log/slog"
	"math/rand"
	"strings"
	"time"

	accessAdaptors "github.com/ashupednekar/litewebservices-portal/internal/access/adaptors"
	"github.com/ashupednekar/litewebservices-portal/internal/project/adaptors"
	"github.com/ashupednekar/litewebservices-portal/internal/project/vendors"
	"github.com/ashupednekar/litewebservices-portal/pkg"
	"github.com/ashupednekar/litewebservices-portal/pkg/state"
	"github.com/ashupednekar/litewebservices-portal/templates"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
)

type ProjectHandlers struct {
	state *state.AppState
}

func NewProjectHandlers(s *state.AppState) *ProjectHandlers {
	return &ProjectHandlers{state: s}
}

func generateInviteCode() string {
	const chars = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	b := make([]byte, 8)
	for i := range b {
		b[i] = chars[rand.Intn(len(chars))]
	}
	return string(b)
}

func (h *ProjectHandlers) CreateProject(c *gin.Context) {
	var req struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid request"})
		return
	}

	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(401, gin.H{"error": "unauthorized"})
		return
	}

	slog.Debug("Creating project", "name", req.Name, "user", pkg.Cfg.VcsUser, "vendor", pkg.Cfg.VcsVendor)
	vcsClient, err := vendors.NewVendorClient()
	if err != nil {
		slog.Error("VCS Init failed", "error", err)
		c.JSON(500, gin.H{"error": fmt.Sprintf("failed to init vcs client: %v", err)})
		return
	}

	tx, err := h.state.DBPool.Begin(c.Request.Context())
	if err != nil {
		slog.Error("DB Begin transaction failed", "error", err)
		c.JSON(500, gin.H{"error": "failed to start transaction"})
		return
	}
	defer tx.Rollback(c.Request.Context())

	q := adaptors.New(h.state.DBPool).WithTx(tx)
	accessQ := accessAdaptors.New(h.state.DBPool).WithTx(tx)

	project, err := q.CreateProject(c.Request.Context(), adaptors.CreateProjectParams{
		Name:        req.Name,
		Description: pgtype.Text{Valid: false},
		CreatedBy:   userID.([]byte),
	})
	if err != nil {
		slog.Error("DB CreateProject failed", "error", err)
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	err = accessQ.AddProjectOwner(c.Request.Context(), accessAdaptors.AddProjectOwnerParams{
		UserID:    userID.([]byte),
		ProjectID: project.ID,
	})
	if err != nil {
		slog.Error("DB AddOwner failed", "error", err)
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	repo, err := vcsClient.CreateRepo(c.Request.Context(), vendors.CreateRepoOptions{
		Name:        req.Name,
		Description: "Created via LiteWebServices Portal",
		Private:     true,
		AutoInit:    true,
	})
	if err != nil {
		if strings.Contains(err.Error(), "already exists") || strings.Contains(err.Error(), "409") {
			slog.Info("Repo already exists, will sync existing functions")
		} else {
			slog.Error("VCS CreateRepo failed", "error", err)
			c.JSON(500, gin.H{"error": fmt.Sprintf("failed to create repo: %v", err)})
			return
		}
	}

	repoName := req.Name
	if repo != nil {
		repoName = repo.Name
	}

	webhookURL := fmt.Sprintf("https://%s/api/webhooks/vcs", pkg.Cfg.Fqdn)
	_, err = vcsClient.AddWebhook(c.Request.Context(), pkg.Cfg.VcsUser, repoName, vendors.WebhookOptions{
		URL:         webhookURL,
		ContentType: "json",
		Secret:      "TODO_GENERATE_SECRET",
		Events:      []string{"push", "pull_request"},
		Active:      true,
		InsecureSSL: true,
	})
	if err != nil {
		if strings.Contains(err.Error(), "already exists") || strings.Contains(err.Error(), "409") {
			slog.Info("Webhook already exists")
		} else {
			slog.Error("VCS AddWebhook failed", "error", err)
			c.JSON(500, gin.H{"error": fmt.Sprintf("failed to add webhook: %v", err)})
			return
		}
	}

	if err := tx.Commit(c.Request.Context()); err != nil {
		slog.Error("DB Commit transaction failed", "error", err)
		c.JSON(500, gin.H{"error": "failed to commit transaction"})
		return
	}

	if err := SyncRepoFunctionsToDb(c, h.state.DBPool, project.ID, req.Name, userID.([]byte)); err != nil {
		slog.Warn("Failed to sync repo functions", "error", err)
	}

	c.JSON(201, gin.H{"id": project.ID, "name": project.Name})
}

func (h *ProjectHandlers) SyncProject(c *gin.Context) {
	projectUUID := c.MustGet("projectUUID").(pgtype.UUID)
	projectName := c.MustGet("projectName").(string)
	userID := c.MustGet("userID").([]byte)

	if err := SyncRepoFunctionsToDb(c, h.state.DBPool, projectUUID, projectName, userID); err != nil {
		slog.Error("Sync failed", "project", projectName, "error", err)
		c.JSON(500, gin.H{"error": "sync failed"})
		return
	}

	c.JSON(200, gin.H{"status": "synced"})
}

func (h *ProjectHandlers) CreateProjectInvite(c *gin.Context) {
	ctx := c.Request.Context()
	user := c.MustGet("userID").([]byte)
	project := c.MustGet("projectUUID").(pgtype.UUID)

	tx, err := h.state.DBPool.Begin(ctx)
	if err != nil {
		c.AbortWithStatusJSON(500, gin.H{"error": "error starting transaction"})
		return
	}
	defer tx.Rollback(ctx)

	accessQ := accessAdaptors.New(h.state.DBPool).WithTx(tx)

	role, err := accessQ.GetUserProjectRole(ctx, accessAdaptors.GetUserProjectRoleParams{
		UserID:    user,
		ProjectID: project,
	})
	if err != nil || role != "owner" {
		c.AbortWithStatusJSON(403, gin.H{"error": "only owners can invite"})
		return
	}

	code := generateInviteCode()

	invite, err := accessQ.CreateProjectInvite(ctx, accessAdaptors.CreateProjectInviteParams{
		ProjectID:  project,
		InviteCode: code,
		CreatedBy:  user,
		ExpiresAt:  pgtype.Timestamptz{Time: time.Now().Add(24 * time.Hour), Valid: true},
	})
	if err != nil {
		c.AbortWithStatusJSON(500, gin.H{"error": "failed to create invite"})
		return
	}
	if err := tx.Commit(ctx); err != nil {
		c.AbortWithStatusJSON(500, gin.H{"error": "failed to commit"})
		return
	}
	c.JSON(200, templates.Invite{
		Code: invite.InviteCode, ExpiresAt: invite.ExpiresAt.Time,
	})
}

func (h *ProjectHandlers) JoinProjectByInvite(c *gin.Context) {
	ctx := c.Request.Context()
	user := c.MustGet("userID").([]byte)
	code := c.Query("code")
	if code == "" {
		c.AbortWithStatusJSON(400, gin.H{"error": "invite code required"})
		return
	}
	tx, err := h.state.DBPool.Begin(ctx)
	if err != nil {
		c.AbortWithStatusJSON(500, gin.H{"error": "failed to start transaction"})
		return
	}
	defer tx.Rollback(ctx)

	accessQ := accessAdaptors.New(h.state.DBPool).WithTx(tx)

	slog.Info("processing invite", "code", code)
	invite, err := accessQ.GetValidInviteByCode(ctx, code)
	if err != nil {
		slog.Error("failed to get invite", "error", err)
		c.AbortWithStatusJSON(400, gin.H{"error": "invalid or expired invite"})
		return
	}

	if bytes.Equal(invite.CreatedBy, user) {
		c.AbortWithStatusJSON(400, gin.H{"error": "cannot use your own invite"})
		return
	}

	err = accessQ.AddViewerToProject(ctx, accessAdaptors.AddViewerToProjectParams{
		UserID:    user,
		ProjectID: invite.ProjectID,
	})
	if err != nil {
		slog.Error("failed to add user to project", "error", err)
		c.AbortWithStatusJSON(500, gin.H{"error": "failed to add user to project"})
		return
	}

	err = accessQ.MarkInviteUsed(ctx, invite.ID)
	if err != nil {
		slog.Error("failed to consume invite", "error", err)
		c.AbortWithStatusJSON(500, gin.H{"error": "failed to consume invite"})
		return
	}

	if err := tx.Commit(ctx); err != nil {
		slog.Error("failed to commit", "error", err)
		c.AbortWithStatusJSON(500, gin.H{"error": "failed to commit"})
		return
	}

	c.JSON(200, gin.H{
		"project_id": invite.ProjectID,
	})
}

func (h *ProjectHandlers) ListProjectAccess(c *gin.Context) {
	project := c.MustGet("projectUUID").(pgtype.UUID)
	accessQ := accessAdaptors.New(h.state.DBPool)
	rows, err := accessQ.ListUsersForProject(c.Request.Context(), project)
	if err != nil {
		c.AbortWithStatusJSON(500, gin.H{"error": "error listing users for project"})
		return
	}

	out := make([]templates.AccessUser, 0, len(rows))
	for _, r := range rows {
		out = append(out, templates.AccessUser{
			ID:   fmt.Sprintf("%x", r.UserID),
			Name: r.UserName,
			Role: r.Role,
		})
	}
	c.JSON(200, out)
}

func (h *ProjectHandlers) UpdateProjectAccess(c *gin.Context) {
	project := c.MustGet("projectUUID").(pgtype.UUID)
	uidStr := c.Param("id")
	// Convert hex string to []byte
	var uid []byte
	fmt.Sscanf(uidStr, "%x", &uid)

	accessQ := accessAdaptors.New(h.state.DBPool)
	var req struct {
		Role string `json:"role"`
	}
	if err := c.BindJSON(&req); err != nil {
		c.AbortWithStatusJSON(400, gin.H{"error": "invalid json body"})
		return
	}
	if err := accessQ.UpdateUserProjectRole(c.Request.Context(), accessAdaptors.UpdateUserProjectRoleParams{
		UserID:    uid,
		ProjectID: project,
		Column3:   req.Role,
	}); err != nil {
		c.AbortWithStatusJSON(500, gin.H{"error": "error updating user access to project"})
		return
	}
	c.Status(200)
}

func (h *ProjectHandlers) RevokeProjectAccess(c *gin.Context) {
	project := c.MustGet("projectUUID").(pgtype.UUID)
	uidStr := c.Param("id")
	var uid []byte
	fmt.Sscanf(uidStr, "%x", &uid)

	accessQ := accessAdaptors.New(h.state.DBPool)
	if err := accessQ.RevokeProjectAccess(c.Request.Context(), accessAdaptors.RevokeProjectAccessParams{
		ProjectID: project,
		UserID:    uid,
	}); err != nil {
		c.AbortWithStatusJSON(500, gin.H{"error": "error revoking project access"})
		return
	}
	c.Status(200)
}

func (h *ProjectHandlers) ListProjects(c *gin.Context) {
	user := c.MustGet("userID").([]byte)
	q := accessAdaptors.New(h.state.DBPool)
	rows, err := q.ListProjectsForUser(c.Request.Context(), user)
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to list projects"})
		return
	}

	out := make([]gin.H, 0, len(rows))
	for _, r := range rows {
		out = append(out, gin.H{
			"id":   fmt.Sprintf("%x", r.ID.Bytes),
			"name": r.Name,
			"role": r.Role,
		})
	}
	c.JSON(200, out)
}

func (h *ProjectHandlers) GetProject(c *gin.Context) {
	project := c.MustGet("projectUUID").(pgtype.UUID)
	q := adaptors.New(h.state.DBPool)
	p, err := q.GetProjectByID(c.Request.Context(), project)
	if err != nil {
		c.JSON(404, gin.H{"error": "project not found"})
		return
	}
	c.JSON(200, p)
}

func (h *ProjectHandlers) DeleteProject(c *gin.Context) {
	// TODO: implement actual deletion from VCS and DB
	c.JSON(200, gin.H{"status": "delete not fully implemented yet"})
}
