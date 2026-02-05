package handlers

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/ashupednekar/litefunctions/portal/internal/project/repo"
	"github.com/ashupednekar/litefunctions/portal/internal/project/vendors"
	"github.com/ashupednekar/litefunctions/portal/pkg"
	"github.com/gin-gonic/gin"
)

type ActionHandelrs struct {
}

func NewActionHandlers() *ActionHandelrs {
	return &ActionHandelrs{}
}

func (h *ActionHandelrs) Status(ctx *gin.Context) {
	vcsClient, err := vendors.NewVendorClient()
	if err != nil {
		slog.Error("VCS Init failed", "error", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to init vcs client: %v", err)})
		return
	}

	r := ctx.MustGet("repo").(*repo.GitRepo)

	opts := vendors.ActionsProgressOptions{
		Branch:     ctx.Query("branch"),
		Status:     ctx.Query("status"),
		Event:      ctx.Query("event"),
		WorkflowID: ctx.Query("workflow_id"),
	}
	if limitRaw := ctx.Query("limit"); limitRaw != "" {
		limit, err := strconv.Atoi(limitRaw)
		if err != nil || limit <= 0 {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "limit must be a positive integer"})
			return
		}
		opts.Limit = limit
	}

	flusher, ok := ctx.Writer.(http.Flusher)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "streaming unsupported"})
		return
	}

	ctx.Writer.Header().Set("Content-Type", "text/event-stream")
	ctx.Writer.Header().Set("Cache-Control", "no-cache")
	ctx.Writer.Header().Set("Connection", "keep-alive")
	ctx.Writer.Header().Set("X-Accel-Buffering", "no")
	ctx.Status(http.StatusOK)

	send := func() {
		progress, err := vcsClient.GetActionsProgress(
			ctx.Request.Context(),
			pkg.Cfg.VcsUser,
			r.Project,
			opts,
		)
		if err != nil {
			ctx.SSEvent("error", gin.H{"error": err.Error()})
			flusher.Flush()
			return
		}
		ctx.SSEvent("status", progress)
		flusher.Flush()
	}

	send()

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Request.Context().Done():
			return
		case <-ticker.C:
			send()
		}
	}
}
