package handlers

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/gin-gonic/gin"
)

type RuntimeAssetsHandler struct {
	runtimes fs.FS
}

func NewRuntimeAssetsHandler() *RuntimeAssetsHandler {
	const base = "/app/runtime-assets/runtimes"
	return &RuntimeAssetsHandler{runtimes: os.DirFS(base)}
}

func (h *RuntimeAssetsHandler) RuntimesFile(c *gin.Context) {
	fileServer := http.FileServer(http.FS(h.runtimes))
	prefix := "/api/runtime-assets/runtimes"
	if strings.HasPrefix(c.Request.URL.Path, prefix) {
		c.Request.URL.Path = strings.TrimPrefix(c.Request.URL.Path, prefix)
	}
	if c.Request.URL.Path == "" {
		c.Request.URL.Path = "/"
	}
	fileServer.ServeHTTP(c.Writer, c.Request)
}

func (h *RuntimeAssetsHandler) RuntimesTarGz(c *gin.Context) {
	c.Header("Content-Type", "application/gzip")
	c.Header("Content-Disposition", "attachment; filename=\"runtimes.tar.gz\"")

	gw := gzip.NewWriter(c.Writer)
	defer gw.Close()
	tw := tar.NewWriter(gw)
	defer tw.Close()

	_ = fs.WalkDir(h.runtimes, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		f, err := h.runtimes.Open(p)
		if err != nil {
			return nil
		}
		defer f.Close()

		hdr, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return nil
		}
		hdr.Name = path.Join("runtimes", p)
		if err := tw.WriteHeader(hdr); err != nil {
			return nil
		}
		_, _ = io.Copy(tw, f)
		return nil
	})
}
