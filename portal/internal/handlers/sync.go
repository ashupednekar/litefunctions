package handlers

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"strings"

	common "github.com/ashupednekar/litefunctions/common"
	vcs "github.com/ashupednekar/litefunctions/common/vcs/pkg"
	"github.com/ashupednekar/litefunctions/portal/internal/config"
	endpointadaptors "github.com/ashupednekar/litefunctions/portal/internal/endpoint/adaptors"
	functionadaptors "github.com/ashupednekar/litefunctions/portal/internal/function/adaptors"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

var extLang = map[string]string{
	".py":  "python",
	".go":  "go",
	".rs":  "rust",
	".ts":  "ts",
	".lua": "lua",
}

// SyncRepoFunctionsToDb walks a project repository and merges functions into
// the portal database. Agent-authored functions can describe async execution
// and custom endpoints in .litefunctions/functions.json.
func SyncRepoFunctionsToDb(ctx context.Context, pool *pgxpool.Pool, projectUUID pgtype.UUID, projectName string, userID []byte) error {
	r, err := vcs.NewRepo(config.Cfg.VCSConfig(), projectName, nil)
	if err != nil {
		return fmt.Errorf("failed to clone repo: %w", err)
	}

	manifest, err := loadRepoManifest(r)
	if err != nil {
		return fmt.Errorf("failed to read function manifest: %w", err)
	}
	manifestByKey := make(map[string]common.ManifestFunction, len(manifest.Functions))
	for _, fn := range manifest.Functions {
		manifestByKey[fn.Language+"/"+fn.Name] = fn
	}

	q := functionadaptors.New(pool)

	existingFns, err := q.ListFunctionsForProject(ctx, projectUUID)
	if err != nil {
		return fmt.Errorf("failed to list existing functions: %w", err)
	}

	existingPaths := make(map[string]pgtype.UUID)
	for _, fn := range existingFns {
		existingPaths[fn.Path] = fn.ID
	}
	fnByKey := make(map[string]functionadaptors.Function)
	for _, fn := range existingFns {
		fnByKey[fn.Language+"/"+fn.Name] = fn
	}

	eq := endpointadaptors.New(pool)
	existingEps, err := eq.ListEndpointsForProject(ctx, projectUUID)
	if err != nil {
		return fmt.Errorf("failed to list existing endpoints: %w", err)
	}

	// map of functionID -> set of endpoint name|method already defined
	existingFnEps := make(map[pgtype.UUID]map[string]bool)
	for _, ep := range existingEps {
		if _, ok := existingFnEps[ep.FunctionID]; !ok {
			existingFnEps[ep.FunctionID] = make(map[string]bool)
		}
		existingFnEps[ep.FunctionID][ep.Name+"|"+ep.Method] = true
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

		manifestFn, inManifest := manifestByKey[lang+"/"+fnName]
		isAsync := false
		var endpoints []common.ManifestEndpoint
		if inManifest {
			isAsync = manifestFn.IsAsync
			endpoints = manifestFn.Endpoints
		}

		createdFn := false
		if id, exists := existingPaths[path]; exists {
			fnID = id
		} else if existing, exists := fnByKey[lang+"/"+fnName]; exists {
			fnID = existing.ID
		} else {
			fn, err := q.CreateFunction(ctx, functionadaptors.CreateFunctionParams{
				ProjectID: projectUUID,
				Name:      fnName,
				Language:  lang,
				Path:      path,
				IsAsync:   isAsync,
				CreatedBy: userID,
			})
			if err != nil {
				slog.Warn("Failed to create function in DB", "path", path, "error", err)
				return nil
			}
			fnID = fn.ID
			createdFn = true
			fnByKey[lang+"/"+fnName] = fn
		}

		if existing, exists := fnByKey[lang+"/"+fnName]; exists {
			if !inManifest {
				// Preserve async functions created through the portal; only the
				// manifest is allowed to override the database flag.
				isAsync = existing.IsAsync
			} else if existing.IsAsync != isAsync {
				if updated, err := q.UpdateFunctionIsAsync(ctx, functionadaptors.UpdateFunctionIsAsyncParams{
					ID:      fnID,
					IsAsync: isAsync,
				}); err != nil {
					slog.Warn("Failed to update function async flag", "name", fnName, "error", err)
				} else {
					fnByKey[lang+"/"+fnName] = updated
				}
			}
		}

		if len(endpoints) == 0 {
			method := "POST"
			if isAsync {
				method = "GET"
			}
			endpoints = []common.ManifestEndpoint{{
				Name:   fmt.Sprintf("/%s/%s", projectName, fnName),
				Method: method,
				Scope:  "public",
			}}
		}

		if existingFnEps[fnID] == nil {
			existingFnEps[fnID] = make(map[string]bool)
		}
		for _, endpoint := range endpoints {
			name := strings.TrimSpace(endpoint.Name)
			if name == "" {
				name = fmt.Sprintf("/%s/%s", projectName, fnName)
			}
			method := strings.ToUpper(strings.TrimSpace(endpoint.Method))
			if method == "" {
				method = "POST"
			}
			scope := strings.TrimSpace(endpoint.Scope)
			if scope == "" {
				scope = "public"
			}
			key := name + "|" + method
			if existingFnEps[fnID][key] {
				continue
			}
			_, err = eq.CreateEndpoint(ctx, endpointadaptors.CreateEndpointParams{
				ProjectID:  projectUUID,
				Name:       name,
				Method:     method,
				Scope:      scope,
				FunctionID: fnID,
			})
			if err != nil {
				slog.Warn("Failed to create endpoint", "name", name, "method", method, "error", err)
				continue
			}
			existingFnEps[fnID][key] = true
		}

		if createdFn {
			_, err = common.CreateFunctionResource(
				ctx,
				config.Cfg.OperatorUrl,
				config.Cfg.PodNamespace,
				fnName,
				projectName,
				lang,
				isAsync,
			)
			if err != nil {
				slog.Warn("Failed to create function CRD", "name", fnName, "error", err)
			}
		}

		slog.Debug("Synced function", "name", fnName, "lang", lang, "async", isAsync)
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to walk functions: %w", err)
	}

	return nil
}

func loadRepoManifest(r vcs.Repo) (*common.FunctionsManifest, error) {
	file, err := r.Filesystem().Open(common.FunctionsManifestPath)
	if err != nil {
		return common.ParseFunctionsManifest(nil)
	}
	defer file.Close()
	body, err := io.ReadAll(io.LimitReader(file, 1<<20))
	if err != nil {
		return nil, err
	}
	return common.ParseFunctionsManifest(body)
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
