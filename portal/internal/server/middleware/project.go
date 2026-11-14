package middleware

import (
	"encoding/hex"
	"errors"
	"log/slog"

	vcs "github.com/ashupednekar/litefunctions/common/vcs/pkg"
	accessadaptors "github.com/ashupednekar/litefunctions/portal/internal/access/adaptors"
	"github.com/ashupednekar/litefunctions/portal/internal/auth"
	"github.com/ashupednekar/litefunctions/portal/internal/config"
	projectadaptors "github.com/ashupednekar/litefunctions/portal/internal/project/adaptors"
	"github.com/ashupednekar/litefunctions/portal/internal/state"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// RequireProjectRole gates mutating project APIs. Reads remain available to
// viewers, while writes require a manager or owner.
func RequireProjectRole(roles ...string) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(roles))
	for _, role := range roles {
		allowed[role] = struct{}{}
	}

	return func(c *gin.Context) {
		if _, ok := allowed[c.GetString("projectRole")]; !ok {
			c.AbortWithStatusJSON(403, gin.H{"error": "insufficient project role"})
			return
		}
		c.Next()
	}
}

func ProjectMiddleware(s *state.AppState) gin.HandlerFunc {
	return func(c *gin.Context) {
		projectIDHex, err := c.Cookie("lws_project")
		if err != nil {
			c.JSON(400, gin.H{"error": "project cookie missing"})
			return
		}

		projectIDBytes, err := hex.DecodeString(projectIDHex)
		if err != nil || len(projectIDBytes) != 16 {
			c.JSON(400, gin.H{"error": "invalid project id in cookie"})
			return
		}
		var projectUUID pgtype.UUID
		copy(projectUUID.Bytes[:], projectIDBytes)
		projectUUID.Valid = true

		pq := projectadaptors.New(s.DBPool)
		proj, err := pq.GetProjectByID(c.Request.Context(), projectUUID)
		if err != nil {
			c.JSON(404, gin.H{"error": "project not found"})
			return
		}
		projectName := proj.Name

		userID, ok := c.Get("userID")
		if !ok {
			c.JSON(401, gin.H{"error": "unauthorized"})
			return
		}
		userIDBytes := userID.([]byte)

		role := "owner"
		if !auth.IsSystemUserID(userIDBytes) {
			role, err = accessadaptors.New(s.DBPool).GetUserProjectRole(c.Request.Context(), accessadaptors.GetUserProjectRoleParams{
				UserID:    userIDBytes,
				ProjectID: projectUUID,
			})
			if errors.Is(err, pgx.ErrNoRows) {
				c.JSON(403, gin.H{"error": "forbidden"})
				return
			}
			if err != nil {
				slog.Error("failed to check project access", "project", projectName, "error", err)
				c.JSON(500, gin.H{"error": "failed to check project access"})
				return
			}
		}

		r, err := vcs.NewRepo(config.Cfg.VCSConfig(), projectName, nil)
		if err != nil {
			slog.Error("repo.NewGitRepo failed", "project", projectName, "error", err)
			c.JSON(500, gin.H{"error": "error instantiating repo"})
			return
		}
		c.Set("repo", r)
		c.Set("projectName", projectName)
		c.Set("projectUUID", projectUUID)
		c.Set("projectRole", role)
		c.Next()
	}
}
