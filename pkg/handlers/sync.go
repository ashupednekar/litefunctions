package handlers

import (
	"fmt"
	"path/filepath"
	"strings"

	functionadaptors "github.com/ashupednekar/litewebservices-portal/internal/function/adaptors"
	"github.com/ashupednekar/litewebservices-portal/internal/project/repo"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

var extLang = map[string]string{
	".py":  "python",
	".go":  "go",
	".rs":  "rust",
	".js":  "javascript",
	".lua": "lua",
}

func SyncRepoFunctionsToDb(c *gin.Context, pool *pgxpool.Pool, projectUUID pgtype.UUID, projectName string, userID []byte) error {
	r, err := repo.NewGitRepo(projectName, nil)
	if err != nil {
		return fmt.Errorf("failed to clone repo: %w", err)
	}

	q := functionadaptors.New(pool)

	existingFns, err := q.ListFunctionsForProject(c.Request.Context(), projectUUID)
	if err != nil {
		return fmt.Errorf("failed to list existing functions: %w", err)
	}

	existingPaths := make(map[string]bool)
	for _, fn := range existingFns {
		existingPaths[fn.Path] = true
	}

	err = walkFunctions(r, "/functions", func(path string) error {
		if existingPaths[path] {
			fmt.Printf("[DEBUG] Function already exists in DB: %s\n", path)
			return nil
		}

		ext := filepath.Ext(path)
		lang, ok := extLang[ext]
		if !ok {
			fmt.Printf("[DEBUG] Skipping unknown extension: %s\n", path)
			return nil
		}

		name := strings.TrimSuffix(filepath.Base(path), ext)

		_, err := q.CreateFunction(c.Request.Context(), functionadaptors.CreateFunctionParams{
			ProjectID: projectUUID,
			Name:      name,
			Language:  lang,
			Path:      path,
			CreatedBy: userID,
		})
		if err != nil {
			fmt.Printf("[WARN] Failed to create function %s in DB: %v\n", path, err)
			return nil
		}

		fmt.Printf("[DEBUG] Synced function to DB: %s (%s)\n", name, lang)
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to walk functions: %w", err)
	}

	return nil
}

func walkFunctions(r *repo.GitRepo, dir string, fn func(path string) error) error {
	entries, err := r.Fs.ReadDir(dir)
	if err != nil {
		return nil
	}

	for _, e := range entries {
		fullPath := filepath.Join(dir, e.Name())

		if e.IsDir() {
			if err := walkFunctions(r, fullPath, fn); err != nil {
				return err
			}
		} else {
			relPath := strings.TrimPrefix(fullPath, "/")
			if err := fn(relPath); err != nil {
				return err
			}
		}
	}

	return nil
}
