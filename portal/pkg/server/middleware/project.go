package middleware

import (
	"encoding/hex"
	"log/slog"

	projectadaptors "github.com/ashupednekar/litefunctions/portal/internal/project/adaptors"
	"github.com/ashupednekar/litefunctions/portal/internal/project/repo"
	"github.com/ashupednekar/litefunctions/portal/pkg/state"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
)

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

		r, err := repo.NewGitRepo(projectName, nil)
		if err != nil {
			slog.Error("repo.NewGitRepo failed", "project", projectName, "error", err)
			c.JSON(500, gin.H{"error": "error instantiating repo"})
			return
		}
		c.Set("repo", r)
		c.Set("projectName", projectName)
		c.Set("projectUUID", projectUUID)
		c.Next()
	}
}
