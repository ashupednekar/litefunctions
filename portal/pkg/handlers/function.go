package handlers

import (
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"strings"

	endpointadaptors "github.com/ashupednekar/litefunctions/portal/internal/endpoint/adaptors"
	functionadaptors "github.com/ashupednekar/litefunctions/portal/internal/function/adaptors"
	"github.com/ashupednekar/litefunctions/portal/internal/project/repo"
	"github.com/ashupednekar/litefunctions/portal/pkg"
	"github.com/ashupednekar/litefunctions/portal/pkg/state"
	"github.com/gin-gonic/gin"
	"github.com/go-git/go-git/v6"
	"github.com/jackc/pgx/v5/pgtype"
)

//TODO: add caching
//TODO: add pagination

type FunctionHandlers struct {
	state *state.AppState
}

func NewFunctionHandlers(s *state.AppState) *FunctionHandlers {
	return &FunctionHandlers{state: s}
}

var langExt = map[string]string{
	"python": ".py",
	"go":     ".go",
	"rust":   ".rs",
	"ts":     ".ts",
	"lua":    ".lua",
}

type createFunctionRequest struct {
	Name     string `json:"name"`
	Language string `json:"language"`
	Code     string `json:"code"`
	Path     string `json:"path"`
	IsAsync  bool   `json:"is_async"`
}

func (h *FunctionHandlers) CreateFunction(c *gin.Context) {
	r := c.MustGet("repo").(*repo.GitRepo)
	userID := c.MustGet("userID").([]byte)
	projectUUID := c.MustGet("projectUUID").(pgtype.UUID)

	var req createFunctionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid request"})
		return
	}

	ext := langExt[req.Language]
	if ext == "" {
		c.JSON(400, gin.H{"error": "invalid language"})
		return
	}

	q := functionadaptors.New(h.state.DBPool)
	fns, err := q.ListFunctionsForProject(c.Request.Context(), projectUUID)
	if err != nil {
		slog.Error("ListFunctionsForProject failed", "error", err)
		c.JSON(500, gin.H{"error": "database error"})
		return
	}
	for _, fn := range fns {
		if fn.Name == req.Name {
			c.JSON(409, gin.H{"error": "function with this name already exists"})
			return
		}
	}

	path := fmt.Sprintf("functions/%s/%s%s", req.Language, req.Name, ext)

	dirParts := strings.Split(path, "/")
	cur := ""
	for _, p := range dirParts[:len(dirParts)-1] {
		cur += "/" + p
		r.Fs.MkdirAll(cur, 0755)
	}

	f, err := r.Fs.Create(path)
	if err != nil {
		slog.Error("r.Fs.Create failed", "path", path, "error", err)
		c.JSON(500, gin.H{"error": "file create error"})
		return
	}
	codeContent := req.Code
	if codeContent == "" {
		codeContent = req.Path
	}
	if codeContent == "" {
		codeContent = "// TODO: implement function\n"
	}
	f.Write([]byte(codeContent))
	f.Close()

	if err := r.Commit(path); err != nil {
		if !strings.Contains(err.Error(), "clean working tree") {
			slog.Error("r.Commit failed", "path", path, "error", err)
			c.JSON(500, gin.H{"error": "commit error"})
			return
		}
	}

	if err := r.Push(); err != nil {
		slog.Error("r.Push failed", "error", err)
		c.JSON(500, gin.H{"error": "push error"})
		return
	}

	eq := endpointadaptors.New(h.state.DBPool)

	projectName := c.MustGet("projectName").(string)

	fn, err := q.CreateFunction(
		c.Request.Context(),
		functionadaptors.CreateFunctionParams{
			ProjectID: projectUUID,
			Name:      req.Name,
			Language:  req.Language,
			Path:      path,
			IsAsync:   req.IsAsync,
			CreatedBy: userID,
		},
	)
	if err != nil {
		slog.Error("CreateFunction DB failed", "error", err)
		c.JSON(500, gin.H{"error": "database error"})
		return
	}

	// Automatically create endpoint: /project/name
	epPath := fmt.Sprintf("/%s/%s", projectName, req.Name)
	_, err = eq.CreateEndpoint(c.Request.Context(), endpointadaptors.CreateEndpointParams{
		ProjectID:  projectUUID,
		Name:       epPath,
		Method:     "GET",
		Scope:      "public",
		FunctionID: fn.ID,
	})
	if err != nil {
		slog.Warn("Failed to create automatic endpoint", "name", req.Name, "error", err)
	}
	_, err = functionadaptors.CreateFunctionCRD(
		c.Request.Context(),
		pkg.Cfg.OperatorUrl,
		"default",
		req.Name,
		projectName,
		req.Language,
		pkg.Cfg.VcsToken,
		req.IsAsync,
	)
	if err != nil {
		slog.Warn("Failed to create function CRD", "name", req.Name, "error", err)
	}
	slog.Info("created function crd")
	c.JSON(201, gin.H{
		"id":       hex.EncodeToString(fn.ID.Bytes[:]),
		"name":     fn.Name,
		"language": fn.Language,
		"path":     fn.Path,
		"is_async": fn.IsAsync,
	})
}

func (h *FunctionHandlers) ListFunctions(c *gin.Context) {
	projectUUID := c.MustGet("projectUUID").(pgtype.UUID)
	search := c.Query("search")
	limitStr := c.DefaultQuery("limit", "100")
	offsetStr := c.DefaultQuery("offset", "0")

	limit, _ := strconv.Atoi(limitStr)
	offset, _ := strconv.Atoi(offsetStr)

	q := functionadaptors.New(h.state.DBPool)
	epQ := endpointadaptors.New(h.state.DBPool)
	fns, err := q.ListFunctionsSearchPaged(c.Request.Context(), functionadaptors.ListFunctionsSearchPagedParams{
		ProjectID: projectUUID,
		Column2:   search,
		Limit:     int32(limit),
		Offset:    int32(offset),
	})
	if err != nil {
		c.JSON(500, gin.H{"error": "database error"})
		return
	}

	endpointByFn := map[string]string{}
	if eps, err := epQ.ListEndpointsForProject(c.Request.Context(), projectUUID); err == nil {
		for _, ep := range eps {
			fnID := hex.EncodeToString(ep.FunctionID.Bytes[:])
			if _, exists := endpointByFn[fnID]; exists {
				continue
			}
			endpointByFn[fnID] = hex.EncodeToString(ep.ID.Bytes[:])
		}
	}

	out := make([]gin.H, 0, len(fns))
	for _, f := range fns {
		fnID := hex.EncodeToString(f.ID.Bytes[:])
		out = append(out, gin.H{
			"id":          fnID,
			"name":        f.Name,
			"language":    f.Language,
			"path":        f.Path,
			"is_async":    f.IsAsync,
			"endpoint_id": endpointByFn[fnID],
		})
	}

	c.JSON(200, out)
}

func (h *FunctionHandlers) GetFunction(c *gin.Context) {
	fnHex := c.Param("fnID")
	fnID, err := hex.DecodeString(fnHex)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid function id"})
		return
	}
	pgFnId := pgtype.UUID{Valid: true}
	copy(pgFnId.Bytes[:], fnID)

	q := functionadaptors.New(h.state.DBPool)

	f, err := q.GetFunctionByID(c.Request.Context(), pgFnId)
	if err != nil {
		c.JSON(404, gin.H{"error": "not found"})
		return
	}

	r := c.MustGet("repo").(*repo.GitRepo)

	slog.Debug("reading file content", "path", f.Path)
	file, err := r.Fs.Open(f.Path)
	if err != nil {
		c.JSON(404, gin.H{
			"msg": "function not found in repo",
		})
		return
	}
	data, err := io.ReadAll(file)
	if err != nil {
		c.JSON(404, gin.H{
			"msg": "error reading file data",
		})
		return
	}

	//TODO: read file contents
	c.JSON(200, gin.H{
		"id":       hex.EncodeToString(f.ID.Bytes[:]),
		"name":     f.Name,
		"language": f.Language,
		"path":     f.Path,
		"is_async": f.IsAsync,
		"content":  string(data),
	})
}

func (h *FunctionHandlers) UpdateFunction(c *gin.Context) {
	fnHex := c.Param("fnID")
	fnID, err := hex.DecodeString(fnHex)
	if err != nil || len(fnID) != 16 {
		c.JSON(400, gin.H{"error": "invalid function id"})
		return
	}
	pgFnId := pgtype.UUID{Valid: true}
	copy(pgFnId.Bytes[:], fnID)

	q := functionadaptors.New(h.state.DBPool)

	f, err := q.GetFunctionByID(c.Request.Context(), pgFnId)
	if err != nil {
		c.JSON(404, gin.H{"error": "function not found"})
		return
	}

	r := c.MustGet("repo").(*repo.GitRepo)

	if c.ContentType() == "text/plain" {
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.JSON(400, gin.H{"error": "invalid body"})
			return
		}

		fh, err := r.Fs.Create(f.Path)
		if err != nil {
			c.JSON(500, gin.H{"error": "write error"})
			return
		}
		fh.Write(body)
		fh.Close()

		slog.Info("commiting file", "path", f.Path)
		err = r.Commit(f.Path)
		if err != nil && !errors.Is(err, git.ErrEmptyCommit) {
			slog.Error("error commiting file", "path", f.Path, "error", err)
			c.JSON(500, gin.H{"error": "commit error"})
			return
		}

		if err := r.Push(); err != nil {
			c.JSON(500, gin.H{"error": "push error"})
			return
		}

		c.JSON(200, gin.H{"status": "saved"})
		return
	}

	var req struct {
		Path    string `json:"path"`
		Code    string `json:"code"`
		IsAsync *bool  `json:"is_async"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid request"})
		return
	}

	resp := gin.H{}

	if req.IsAsync != nil {
		upd, err := q.UpdateFunctionIsAsync(
			c.Request.Context(),
			functionadaptors.UpdateFunctionIsAsyncParams{
				ID:      pgtype.UUID{Bytes: [16]byte(fnID)},
				IsAsync: *req.IsAsync,
			},
		)
		if err == nil {
			resp["is_async"] = upd.IsAsync
		}
	}

	if req.Code != "" {
		fh, err := r.Fs.Create(f.Path)
		if err != nil {
			c.JSON(500, gin.H{"error": "write error"})
			return
		}
		fh.Write([]byte(req.Code))
		fh.Close()

		slog.Info("commiting file", "path", f.Path)
		err = r.Commit(f.Path)
		if err != nil && !errors.Is(err, git.ErrEmptyCommit) {
			slog.Error("error commiting file", "path", f.Path, "error", err)
			c.JSON(500, gin.H{"error": "commit error"})
			return
		}

		if err := r.Push(); err != nil {
			c.JSON(500, gin.H{"error": "push error"})
			return
		}

		resp["status"] = "saved"
	}

	if req.Path != "" {
		upd, err := q.UpdateFunctionPath(
			c.Request.Context(),
			functionadaptors.UpdateFunctionPathParams{
				ID:   pgtype.UUID{Bytes: [16]byte(fnID)},
				Path: req.Path,
			},
		)
		if err == nil {
			resp["path"] = upd.Path
		}
	}

	c.JSON(200, resp)
}

func (h *FunctionHandlers) DeleteFunction(c *gin.Context) {
	fnHex := c.Param("fnID")
	fnID, err := hex.DecodeString(fnHex)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid function id"})
		return
	}
	pgFnId := pgtype.UUID{Valid: true}
	copy(pgFnId.Bytes[:], fnID)

	q := functionadaptors.New(h.state.DBPool)

	f, err := q.GetFunctionByID(c.Request.Context(), pgFnId)
	if err != nil {
		c.JSON(404, gin.H{"error": "not found"})
		return
	}

	if err := q.DeleteFunction(c.Request.Context(), pgFnId); err != nil {
		slog.Error("db delete failed", "error", err)
		c.JSON(500, gin.H{"error": "db delete error"})
		return
	}

	r := c.MustGet("repo").(*repo.GitRepo)

	if err := r.Fs.Remove(f.Path); err != nil {
		slog.Error("failed to remove file", "path", f.Path, "error", err)
		_, rollbackErr := q.CreateFunction(c.Request.Context(), functionadaptors.CreateFunctionParams{
			ProjectID: f.ProjectID,
			Name:      f.Name,
			Language:  f.Language,
			Path:      f.Path,
			IsAsync:   f.IsAsync,
			CreatedBy: f.CreatedBy,
		})
		if rollbackErr != nil {
			slog.Error("rollback failed", "error", rollbackErr)
		}
		c.JSON(500, gin.H{"error": "file remove error"})
		return
	}

	if err := r.Commit(f.Path); err != nil {
		slog.Error("commit failed", "path", f.Path, "error", err)
		_, rollbackErr := q.CreateFunction(c.Request.Context(), functionadaptors.CreateFunctionParams{
			ProjectID: f.ProjectID,
			Name:      f.Name,
			Language:  f.Language,
			Path:      f.Path,
			IsAsync:   f.IsAsync,
			CreatedBy: f.CreatedBy,
		})
		if rollbackErr != nil {
			slog.Error("rollback failed", "error", rollbackErr)
		}
		c.JSON(500, gin.H{"error": "commit error"})
		return
	}

	if err := r.Push(); err != nil {
		slog.Error("push failed", "error", err)
		_, rollbackErr := q.CreateFunction(c.Request.Context(), functionadaptors.CreateFunctionParams{
			ProjectID: f.ProjectID,
			Name:      f.Name,
			Language:  f.Language,
			Path:      f.Path,
			IsAsync:   f.IsAsync,
			CreatedBy: f.CreatedBy,
		})
		if rollbackErr != nil {
			slog.Error("rollback failed", "error", rollbackErr)
		}
		c.JSON(500, gin.H{"error": "push error"})
		return
	}

	c.JSON(200, gin.H{"status": "deleted"})
}
