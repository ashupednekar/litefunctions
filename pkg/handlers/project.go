package handlers

import (
	"fmt"

	"github.com/ashupednekar/litewebservices-portal/internal/project/adaptors"
	"github.com/ashupednekar/litewebservices-portal/internal/project/vendors"
	"github.com/ashupednekar/litewebservices-portal/pkg"
	"github.com/ashupednekar/litewebservices-portal/pkg/state"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
)

type ProjectHandlers struct {
	state *state.AppState
}

func NewProjectHandlers(s *state.AppState) *ProjectHandlers {
	return &ProjectHandlers{state: s}
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

	fmt.Printf("[DEBUG] Creating project: %s, User: %s, Vendor: %s\n", req.Name, pkg.Cfg.VcsUser, pkg.Cfg.VcsVendor)
	vcsClient, err := vendors.NewVendorClient()
	if err != nil {
		fmt.Printf("[ERROR] VCS Init: %v\n", err)
		c.JSON(500, gin.H{"error": fmt.Sprintf("failed to init vcs client: %v", err)})
		return
	}

	tx, err := h.state.DBPool.Begin(c.Request.Context())
	if err != nil {
		fmt.Printf("[ERROR] DB Begin: %v\n", err)
		c.JSON(500, gin.H{"error": "failed to start transaction"})
		return
	}
	defer tx.Rollback(c.Request.Context())

	q := adaptors.New(h.state.DBPool).WithTx(tx)

	project, err := q.CreateProject(c.Request.Context(), adaptors.CreateProjectParams{
		Name:        req.Name,
		Description: pgtype.Text{Valid: false},
		CreatedBy:   userID.([]byte),
	})
	if err != nil {
		fmt.Printf("[ERROR] DB CreateProject: %v\n", err)
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	err = q.AddUserToProject(c.Request.Context(), adaptors.AddUserToProjectParams{
		UserID:    userID.([]byte),
		ProjectID: project.ID,
		Role:      pgtype.Text{String: "owner", Valid: true},
	})
	if err != nil {
		fmt.Printf("[ERROR] DB AddUser: %v\n", err)
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
		fmt.Printf("[ERROR] VCS CreateRepo: %v\n", err)
		c.JSON(500, gin.H{"error": fmt.Sprintf("failed to create repo: %v", err)})
		return
	}

	webhookURL := fmt.Sprintf("https://%s/api/webhooks/vcs", pkg.Cfg.Fqdn)
	_, err = vcsClient.AddWebhook(c.Request.Context(), pkg.Cfg.VcsUser, repo.Name, vendors.WebhookOptions{
		URL:         webhookURL,
		ContentType: "json",
		Secret:      "TODO_GENERATE_SECRET",
		Events:      []string{"push", "pull_request"},
		Active:      true,
		InsecureSSL: true,
	})
	if err != nil {
		fmt.Printf("[ERROR] VCS AddWebhook: %v\n", err)
		c.JSON(500, gin.H{"error": fmt.Sprintf("failed to add webhook: %v", err)})
		return
	}

	if err := tx.Commit(c.Request.Context()); err != nil {
		fmt.Printf("[ERROR] DB Commit: %v\n", err)
		c.JSON(500, gin.H{"error": "failed to commit transaction"})
		return
	}

	c.JSON(201, gin.H{"id": project.ID, "name": project.Name})
}
