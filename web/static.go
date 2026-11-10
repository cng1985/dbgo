package web

import (
	"io/fs"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/cng1985/dbgo/internal/platform/response"
)

func RegisterStatic(engine *gin.Engine) error {
	sub, err := fs.Sub(Dist, "dist")
	if err != nil {
		return err
	}
	engine.NoRoute(func(c *gin.Context) {
		if isAPI(c.Request.URL.Path) {
			response.JSONStatus(c, http.StatusNotFound, 404, "接口不存在")
			return
		}
		if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
			c.Status(http.StatusNotFound)
			return
		}
		if strings.Contains(c.Request.URL.Path, "..") {
			c.Status(http.StatusNotFound)
			return
		}
		relative := strings.TrimPrefix(c.Request.URL.Path, "/")
		if relative == "" || !strings.Contains(relative, ".") {
			serveIndex(c, sub)
			return
		}
		info, err := fs.Stat(sub, relative)
		if err != nil || info.IsDir() {
			c.Status(http.StatusNotFound)
			return
		}
		http.FileServer(http.FS(sub)).ServeHTTP(c.Writer, c.Request)
	})
	return nil
}

func serveIndex(c *gin.Context, sub fs.FS) {
	data, err := fs.ReadFile(sub, "index.html")
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", data)
}

func isAPI(path string) bool {
	return path == "/api" || strings.HasPrefix(path, "/api/")
}
