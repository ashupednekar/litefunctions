package handlers

import (
	"encoding/hex"
	"fmt"
	"io"
	"strings"

	functionadaptors "github.com/ashupednekar/litewebservices-portal/internal/function/adaptors"
	"github.com/ashupednekar/litewebservices-portal/internal/project/adaptors"
	"github.com/ashupednekar/litewebservices-portal/internal/project/repo"
	"github.com/ashupednekar/litewebservices-portal/pkg/state"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
)

type FunctionHandlers struct {
	state *state.AppState
}

func NewFunctionHandlers(s *state.AppState) *FunctionHandlers {
	return &FunctionHandlers{state: s}
}

var langExt = map[string]string{
	"python":     ".py",
	"go":         ".go",
	"rust":       ".rs",
	"javascript": ".js",
	"lua":        ".lua",
}

type createFunctionRequest struct {
	Name     string `json:"name"`
	Language string `json:"language"`
	Code     string `json:"path"`
}

func (h *FunctionHandlers) CreateFunction(c *gin.Context) {
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

	pq := adaptors.New(h.state.DBPool)
	proj, err := pq.GetProjectByID(c.Request.Context(), projectUUID)
	if err != nil {
		c.JSON(404, gin.H{"error": "project not found"})
		return
	}
	projectName := proj.Name

	userID := c.MustGet("userID").([]byte)

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

	r, err := repo.NewGitRepo(projectName, nil)
	if err != nil {
		fmt.Printf("[ERROR] repo.NewGitRepo failed: %v\n", err)
		c.JSON(500, gin.H{"error": "repo init error"})
		return
	}

	if err := r.Clone(); err != nil {
		fmt.Printf("[ERROR] r.Clone failed: %v\n", err)
		c.JSON(500, gin.H{"error": "clone error"})
		return
	}

	path := fmt.Sprintf("functions/%s/%s%s", req.Language, req.Name, ext)

	dirParts := strings.Split(path, "/")
	cur := ""
	for _, p := range dirParts[:len(dirParts)-1] {
		cur += "/" + p
		r.Fs.MkdirAll(cur, 0755)
	}

	f, err := r.Fs.Create("/" + path)
	if err != nil {
		fmt.Printf("[ERROR] r.Fs.Create failed for path %s: %v\n", path, err)
		c.JSON(500, gin.H{"error": "file create error"})
		return
	}
	codeContent := req.Code
	if codeContent == "" {
		codeContent = "// TODO: implement function\n"
	}
	f.Write([]byte(codeContent))
	f.Close()

	if err := r.Commit(path); err != nil {
		if !strings.Contains(err.Error(), "clean working tree") {
			fmt.Printf("[ERROR] r.Commit failed: %v\n", err)
			c.JSON(500, gin.H{"error": "commit error"})
			return
		}
	}

	if err := r.Push(); err != nil {
		fmt.Printf("[ERROR] r.Push failed: %v\n", err)
		c.JSON(500, gin.H{"error": "push error"})
		return
	}

	q := functionadaptors.New(h.state.DBPool)

	fn, err := q.CreateFunction(
		c.Request.Context(),
		functionadaptors.CreateFunctionParams{
			ProjectID: projectUUID,
			Name:      req.Name,
			Language:  req.Language,
			Path:      path,
			CreatedBy: userID,
		},
	)
	if err != nil {
		fmt.Printf("[ERROR] CreateFunction DB failed: %v\n", err)
		c.JSON(500, gin.H{"error": "database error"})
		return
	}

	c.JSON(201, gin.H{
		"id":       hex.EncodeToString(fn.ID.Bytes[:]),
		"name":     fn.Name,
		"language": fn.Language,
		"path":     fn.Path,
	})
}

func (h *FunctionHandlers) ListFunctions(c *gin.Context) {
	projectIDHex, err := c.Cookie("lws_project")
	if err != nil {
		c.JSON(400, gin.H{"error": "project cookie missing"})
		return
	}

	projectID, err := hex.DecodeString(projectIDHex)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid project id"})
		return
	}

	q := functionadaptors.New(h.state.DBPool)

	fns, err := q.ListFunctionsForProject(c.Request.Context(), pgtype.UUID{Bytes: [16]byte(projectID)})
	if err != nil {
		c.JSON(500, gin.H{"error": "database error"})
		return
	}

	out := make([]gin.H, 0, len(fns))
	for _, f := range fns {
		out = append(out, gin.H{
			"id":       hex.EncodeToString(f.ID.Bytes[:]),
			"name":     f.Name,
			"language": f.Language,
			"path":     f.Path,
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

	q := functionadaptors.New(h.state.DBPool)

	f, err := q.GetFunctionByID(c.Request.Context(), pgtype.UUID{Bytes: [16]byte(fnID)})
	if err != nil {
		c.JSON(404, gin.H{"error": "not found"})
		return
	}

	c.JSON(200, gin.H{
		"id":       hex.EncodeToString(f.ID.Bytes[:]),
		"name":     f.Name,
		"language": f.Language,
		"path":     f.Path,
	})
}

func (h *FunctionHandlers) UpdateFunction(c *gin.Context) {
	fnHex := c.Param("fnID")
	fnID, err := hex.DecodeString(fnHex)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid function id"})
		return
	}

	q := functionadaptors.New(h.state.DBPool)

	f, err := q.GetFunctionByID(c.Request.Context(), pgtype.UUID{Bytes: [16]byte(fnID)})
	if err != nil {
		c.JSON(404, gin.H{"error": "not found"})
		return
	}

	pq := adaptors.New(h.state.DBPool)
	proj, err := pq.GetProjectByID(c.Request.Context(), f.ProjectID)
	if err != nil {
		c.JSON(404, gin.H{"error": "project not found"})
		return
	}
	projectName := proj.Name

	r, err := repo.NewGitRepo(projectName, nil)
	if err != nil {
		c.JSON(500, gin.H{"error": "repo init error"})
		return
	}

	if err := r.Clone(); err != nil {
		c.JSON(500, gin.H{"error": "clone error"})
		return
	}

	if c.ContentType() == "text/plain" {
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.JSON(400, gin.H{"error": "invalid body"})
			return
		}

		fh, err := r.Fs.Create("/" + f.Path)
		if err != nil {
			c.JSON(500, gin.H{"error": "write error"})
			return
		}
		fh.Write(body)
		fh.Close()

		if err := r.Commit(f.Path); err != nil {
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
		Path string `json:"path"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid request"})
		return
	}

	resp := gin.H{}

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

	q := functionadaptors.New(h.state.DBPool)

	f, err := q.GetFunctionByID(c.Request.Context(), pgtype.UUID{Bytes: [16]byte(fnID)})
	if err != nil {
		c.JSON(404, gin.H{"error": "not found"})
		return
	}

	pq := adaptors.New(h.state.DBPool)
	proj, err := pq.GetProjectByID(c.Request.Context(), f.ProjectID)
	if err != nil {
		c.JSON(404, gin.H{"error": "project not found"})
		return
	}
	projectName := proj.Name

	r, err := repo.NewGitRepo(projectName, nil)
	if err != nil {
		c.JSON(500, gin.H{"error": "repo init error"})
		return
	}

	if err := r.Clone(); err != nil {
		c.JSON(500, gin.H{"error": "clone error"})
		return
	}

	r.Fs.Remove("/" + f.Path)

	if err := r.Commit(f.Path); err != nil {
		c.JSON(500, gin.H{"error": "commit error"})
		return
	}

	if err := r.Push(); err != nil {
		c.JSON(500, gin.H{"error": "push error"})
		return
	}

	if err := q.DeleteFunction(c.Request.Context(), pgtype.UUID{Bytes: [16]byte(fnID)}); err != nil {
		c.JSON(500, gin.H{"error": "db delete error"})
		return
	}

	c.JSON(200, gin.H{"status": "deleted"})
}
