package handlers

import (
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ashupednekar/litefunctions/common"
	"github.com/ashupednekar/litefunctions/common/sample"
	vcs "github.com/ashupednekar/litefunctions/common/vcs/pkg"
	"github.com/ashupednekar/litefunctions/portal/internal/config"
	endpointadaptors "github.com/ashupednekar/litefunctions/portal/internal/endpoint/adaptors"
	functionadaptors "github.com/ashupednekar/litefunctions/portal/internal/function/adaptors"
	"github.com/ashupednekar/litefunctions/portal/internal/state"
	"github.com/gin-gonic/gin"
	"github.com/go-git/go-git/v6"
	"github.com/gorilla/websocket"
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

type createFunctionRequest struct {
	Name     string         `json:"name"`
	Language string         `json:"language"`
	Code     string         `json:"code"`
	Path     string         `json:"path"`
	IsAsync  bool           `json:"is_async"`
	Files    []functionFile `json:"files"`
}

func (h *FunctionHandlers) CreateFunction(c *gin.Context) {
	r := c.MustGet("repo").(vcs.Repo)
	userID := c.MustGet("userID").([]byte)
	projectUUID := c.MustGet("projectUUID").(pgtype.UUID)

	var req createFunctionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid request"})
		return
	}

	ext := common.LangExt[req.Language]
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

	functionDir := fmt.Sprintf("functions/%s/%s", req.Language, req.Name)

	pkg, err := sample.LoadPackage(req.Language, req.Name, req.IsAsync)
	if err != nil {
		slog.Warn("Failed to load default sample", "language", req.Language, "error", err)
		entrypoint := "handle" + ext
		pkg = sample.Package{
			Entrypoint: entrypoint,
			Files:      []sample.File{{Path: entrypoint, Content: []byte("// TODO: implement function\n")}},
		}
	}

	entrypointContent := req.Code
	if entrypointContent != "" {
		entrypointContent = strings.ReplaceAll(entrypointContent, "{{name}}", req.Name)
	} else {
		for _, file := range pkg.Files {
			if file.Path == pkg.Entrypoint {
				entrypointContent = string(file.Content)
				break
			}
		}
	}
	if entrypointContent == "" {
		entrypointContent = "// TODO: implement function\n"
	}

	for _, file := range req.Files {
		if _, ok := cleanFunctionPath(functionDir, file.Path); !ok {
			c.JSON(400, gin.H{"error": "invalid file path"})
			return
		}
	}

	if err := r.Filesystem().MkdirAll(functionDir, 0755); err != nil {
		slog.Error("r.Filesystem().MkdirAll failed", "path", functionDir, "error", err)
		c.JSON(500, gin.H{"error": "directory create error"})
		return
	}

	for _, file := range pkg.Files {
		content := string(file.Content)
		if file.Path == pkg.Entrypoint {
			content = entrypointContent
		}
		if err := writeFunctionFile(r.Filesystem(), functionDir, file.Path, content); err != nil {
			slog.Error("failed to write sample file", "path", file.Path, "error", err)
			c.JSON(500, gin.H{"error": "file write error"})
			return
		}
	}

	for _, file := range req.Files {
		if err := writeFunctionFile(r.Filesystem(), functionDir, file.Path, file.Content); err != nil {
			slog.Error("failed to write uploaded file", "path", file.Path, "error", err)
			c.JSON(400, gin.H{"error": "invalid file path"})
			return
		}
	}

	repoPath := functionDir + "/" + pkg.Entrypoint

	if err := r.Commit(functionDir); err != nil {
		if !strings.Contains(err.Error(), "clean working tree") {
			slog.Error("r.Commit failed", "path", functionDir, "error", err)
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
			Path:      repoPath,
			IsAsync:   req.IsAsync,
			CreatedBy: userID,
		},
	)
	if err != nil {
		slog.Error("CreateFunction DB failed", "error", err)
		c.JSON(500, gin.H{"error": "database error"})
		return
	}

	epPath := fmt.Sprintf("/%s/%s", projectName, req.Name)
	method := "POST"
	if req.IsAsync {
		method = "GET"
	}
	_, err = eq.CreateEndpoint(c.Request.Context(), endpointadaptors.CreateEndpointParams{
		ProjectID:  projectUUID,
		Name:       epPath,
		Method:     method,
		Scope:      "public",
		FunctionID: fn.ID,
	})
	if err != nil {
		slog.Warn("Failed to create endpoint", "name", req.Name, "error", err)
	}
	_, err = common.CreateFunctionResource(
		c.Request.Context(),
		config.Cfg.OperatorUrl,
		config.Cfg.PodNamespace,
		req.Name,
		projectName,
		req.Language,
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

func (h *FunctionHandlers) ListFunctionFiles(c *gin.Context) {
	f, err := functionadaptors.FunctionFromCtx(c, h.state.DBPool)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}

	r := c.MustGet("repo").(vcs.Repo)
	dir := path.Dir(f.Path)
	entrypoint := strings.TrimPrefix(f.Path, dir+"/")

	files, err := readFunctionFiles(r.Filesystem(), dir)
	if err != nil {
		slog.Error("failed to read function package", "path", dir, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read function files"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"dir":        dir,
		"entrypoint": entrypoint,
		"files":      files,
		"tree":       buildFunctionTree(files, entrypoint),
	})
}

func (h *FunctionHandlers) GetFunction(c *gin.Context) {
	f, err := functionadaptors.FunctionFromCtx(c, h.state.DBPool)
	if err != nil {
		c.JSON(404, gin.H{"error": "not found"})
		return
	}

	r := c.MustGet("repo").(vcs.Repo)
	dir := path.Dir(f.Path)
	entrypoint := strings.TrimPrefix(f.Path, dir+"/")

	slog.Debug("reading function package", "path", dir)
	files, err := readFunctionFiles(r.Filesystem(), dir)
	if err != nil {
		slog.Error("failed to read function package", "path", dir, "error", err)
		c.JSON(404, gin.H{
			"msg": "function not found in repo",
		})
		return
	}

	content := ""
	found := false
	for _, file := range files {
		if file.Path == entrypoint {
			content = file.Content
			found = true
			break
		}
	}
	if !found {
		c.JSON(404, gin.H{
			"msg": "function entrypoint not found in repo",
		})
		return
	}

	fnID := hex.EncodeToString(f.ID.Bytes[:])
	c.JSON(200, gin.H{
		"id":          fnID,
		"name":        f.Name,
		"language":    f.Language,
		"path":        f.Path,
		"dir":         dir,
		"entrypoint":  entrypoint,
		"is_async":    f.IsAsync,
		"content":     content,
		"files":       files,
		"tree":        buildFunctionTree(files, entrypoint),
		"logs_stream": fmt.Sprintf("/api/functions/%s/logs/", fnID),
	})
}

func (h *FunctionHandlers) UpdateFunction(c *gin.Context) {
	f, err := functionadaptors.FunctionFromCtx(c, h.state.DBPool)
	if err != nil {
		c.JSON(404, gin.H{"error": "function not found"})
		return
	}

	q := functionadaptors.New(h.state.DBPool)
	r := c.MustGet("repo").(vcs.Repo)

	if c.ContentType() == "text/plain" {
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.JSON(400, gin.H{"error": "invalid body"})
			return
		}

		fh, err := r.Filesystem().Create(f.Path)
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
		Path    string         `json:"path"`
		Code    string         `json:"code"`
		IsAsync *bool          `json:"is_async"`
		Files   []functionFile `json:"files"`
		Deleted []string       `json:"deleted"`
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
				ID:      f.ID,
				IsAsync: *req.IsAsync,
			},
		)
		if err == nil {
			resp["is_async"] = upd.IsAsync
		}
	}

	fs := r.Filesystem()
	dir := path.Dir(f.Path)
	entrypoint := strings.TrimPrefix(f.Path, dir+"/")

	if len(req.Files) > 0 || len(req.Deleted) > 0 {
		for _, file := range req.Files {
			if _, ok := cleanFunctionPath(dir, file.Path); !ok {
				c.JSON(400, gin.H{"error": "invalid file path"})
				return
			}
		}
		for _, file := range req.Deleted {
			clean, ok := cleanFunctionPath(dir, file)
			if !ok || clean == path.Join(dir, entrypoint) {
				c.JSON(400, gin.H{"error": "invalid file path"})
				return
			}
		}

		for _, file := range req.Files {
			if err := writeFunctionFile(fs, dir, file.Path, file.Content); err != nil {
				slog.Error("failed to write function file", "path", file.Path, "error", err)
				c.JSON(400, gin.H{"error": "invalid file path"})
				return
			}
		}
		for _, file := range req.Deleted {
			if err := removeFunctionFile(fs, dir, file); err != nil {
				slog.Error("failed to remove function file", "path", file, "error", err)
				c.JSON(500, gin.H{"error": "file delete error"})
				return
			}
		}

		slog.Info("commiting function package", "path", dir)
		err = r.Commit(dir)
		if err != nil && !errors.Is(err, git.ErrEmptyCommit) {
			slog.Error("error commiting package", "path", dir, "error", err)
			c.JSON(500, gin.H{"error": "commit error"})
			return
		}

		if err := r.Push(); err != nil {
			c.JSON(500, gin.H{"error": "push error"})
			return
		}

		resp["status"] = "saved"
	} else if req.Code != "" {
		fh, err := fs.Create(f.Path)
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
				ID:   f.ID,
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
	f, err := functionadaptors.FunctionFromCtx(c, h.state.DBPool)
	if err != nil {
		c.JSON(404, gin.H{"error": "function not found"})
		return
	}

	q := functionadaptors.New(h.state.DBPool)

	if err := q.DeleteFunction(c.Request.Context(), f.ID); err != nil {
		slog.Error("db delete failed", "error", err)
		c.JSON(500, gin.H{"error": "db delete error"})
		return
	}

	r := c.MustGet("repo").(vcs.Repo)

	if err := r.Filesystem().Remove(f.Path); err != nil {
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

func (h *FunctionHandlers) StreamLogs(c *gin.Context) {
	f, err := functionadaptors.FunctionFromCtx(c, h.state.DBPool)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}

	scheme := "ws"
	lokiHost := strings.TrimSpace(config.Cfg.LokiHost)
	switch {
	case strings.HasPrefix(lokiHost, "http://"):
		lokiHost = strings.TrimPrefix(lokiHost, "http://")
	case strings.HasPrefix(lokiHost, "https://"):
		lokiHost = strings.TrimPrefix(lokiHost, "https://")
		scheme = "wss"
	}
	lokiHost = strings.TrimRight(lokiHost, "/")
	if lokiHost == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "misconfigured loki endpoint"})
		return
	}

	tailURL := url.URL{
		Scheme: scheme,
		Host:   lokiHost,
		Path:   "/loki/api/v1/tail",
	}
	params := tailURL.Query()
	// Loki's tail endpoint parses the query as a single LogQL selector; a
	// binary "or" expression is rejected before the websocket upgrade.
	if c.Query("source") == "build" {
		params.Set("query", `{app="actions-act-runner"}`)
	} else {
		params.Set("query", fmt.Sprintf(
			`{app=~"runtime|litefunctions-ingestor", function=~"%s|"}`,
			regexp.QuoteMeta(f.Name),
		))
	}
	params.Set("limit", "200")
	params.Set("delay_for", "0")
	tailURL.RawQuery = params.Encode()

	conn, res, err := (&websocket.Dialer{
		Proxy:            http.ProxyFromEnvironment,
		HandshakeTimeout: 10 * time.Second,
	}).DialContext(c.Request.Context(), tailURL.String(), nil)

	if err != nil {
		status := http.StatusBadGateway
		if res != nil {
			status = res.StatusCode
			body, _ := io.ReadAll(io.LimitReader(res.Body, 2048))
			slog.Error("loki tail dial failed",
				"url", tailURL.String(),
				"status", res.Status,
				"body", strings.TrimSpace(string(body)),
				"error", err,
			)
		} else {
			slog.Error("loki tail dial failed", "url", tailURL.String(), "error", err)
		}
		c.JSON(status, gin.H{"error": "failed to connect to Loki"})
		return
	}
	defer conn.Close()

	ctx := c.Request.Context()

	// Close Loki connection when the client disconnects.
	go func() {
		<-ctx.Done()
		conn.Close()
	}()

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache, no-transform")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	c.Status(http.StatusOK)

	fmt.Fprint(c.Writer, "retry: 3000\n\n")
	c.Writer.Flush()

	for {
		messageType, message, err := conn.ReadMessage()
		if err != nil {
			if ctx.Err() == nil {
				// Upstream connection failed.
				return
			}
			return
		}

		if messageType != websocket.TextMessage {
			continue
		}

		if _, err := fmt.Fprintf(
			c.Writer,
			"event: logs\ndata: %s\n\n",
			message,
		); err != nil {
			return
		}

		c.Writer.Flush()
	}
}
