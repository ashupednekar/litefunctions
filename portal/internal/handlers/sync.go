package handlers

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	common "github.com/ashupednekar/litefunctions/common"
	vcs "github.com/ashupednekar/litefunctions/common/vcs/pkg"
	"github.com/ashupednekar/litefunctions/portal/internal/config"
	endpointadaptors "github.com/ashupednekar/litefunctions/portal/internal/endpoint/adaptors"
	functionadaptors "github.com/ashupednekar/litefunctions/portal/internal/function/adaptors"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

var extLang = map[string]string{
	".py":   "python",
	".go":   "go",
	".rs":   "rust",
	".ts":   "ts",
	".lua":  "lua",
	".java": "java",
}

func SyncRepoFunctionsToDb(c *gin.Context, pool *pgxpool.Pool, projectUUID pgtype.UUID, projectName string, userID []byte) error {
	r, err := vcs.NewRepo(config.Cfg.VCSConfig(), projectName, nil)
	if err != nil {
		return fmt.Errorf("failed to clone repo: %w", err)
	}

	q := functionadaptors.New(pool)

	existingFns, err := q.ListFunctionsForProject(c.Request.Context(), projectUUID)
	if err != nil {
		return fmt.Errorf("failed to list existing functions: %w", err)
	}

	existingPaths := make(map[string]pgtype.UUID)
	for _, fn := range existingFns {
		existingPaths[fn.Path] = fn.ID
	}

	eq := endpointadaptors.New(pool)
	existingEps, err := eq.ListEndpointsForProject(c.Request.Context(), projectUUID)
	if err != nil {
		return fmt.Errorf("failed to list existing endpoints: %w", err)
	}

	// map of functionID -> set of methods already defined for it
	existingFnEps := make(map[pgtype.UUID]map[string]bool)
	for _, ep := range existingEps {
		if _, ok := existingFnEps[ep.FunctionID]; !ok {
			existingFnEps[ep.FunctionID] = make(map[string]bool)
		}
		existingFnEps[ep.FunctionID][ep.Method] = true
	}
	err = walkFunctions(r, "/functions", func(path string) error {
		ext := filepath.Ext(path)
		lang, ok := extLang[ext]
		if !ok || filepath.Base(path) != "handle"+ext {
			slog.Debug("Skipping unknown extension", "path", path)
			return nil
		}

		var fnID pgtype.UUID
		fnName := filepath.Base(filepath.Dir(path))
		if filepath.Base(filepath.Dir(filepath.Dir(path))) != lang {
			slog.Debug("Skipping function outside language directory", "path", path)
			return nil
		}

		createdFn := false
		if id, exists := existingPaths[path]; exists {
			fnID = id
		} else {
			fn, err := q.CreateFunction(c.Request.Context(), functionadaptors.CreateFunctionParams{
				ProjectID: projectUUID,
				Name:      fnName,
				Language:  lang,
				Path:      path,
				IsAsync:   false,
				CreatedBy: userID,
			})
			if err != nil {
				slog.Warn("Failed to create function in DB", "path", path, "error", err)
				return nil
			}
			fnID = fn.ID
			createdFn = true
		}

		// Ensure automatic endpoint: /project/name (GET)
		if methods, ok := existingFnEps[fnID]; !ok || !methods["GET"] {
			epPath := fmt.Sprintf("/%s/%s", projectName, fnName)
			_, err = eq.CreateEndpoint(c.Request.Context(), endpointadaptors.CreateEndpointParams{
				ProjectID:  projectUUID,
				Name:       epPath,
				Method:     "GET",
				Scope:      "public",
				FunctionID: fnID,
			})
			if err != nil {
				slog.Warn("Failed to create automatic endpoint", "name", fnName, "error", err)
			} else {
				slog.Debug("Created missing automatic endpoint", "name", fnName, "endpoint", epPath)
			}
		}

		if createdFn {
			_, err = common.CreateFunctionResource(
				c.Request.Context(),
				config.Cfg.OperatorUrl,
				config.Cfg.PodNamespace,
				fnName,
				projectName,
				lang,
				false,
			)
			if err != nil {
				slog.Warn("Failed to create function CRD", "name", fnName, "error", err)
			}
		}

		slog.Debug("Synced function", "name", fnName, "lang", lang)
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to walk functions: %w", err)
	}

	return nil
}

func walkFunctions(r vcs.Repo, dir string, fn func(path string) error) error {
	entries, err := r.Filesystem().ReadDir(dir)
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
