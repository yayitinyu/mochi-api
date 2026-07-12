package router

import (
	"io/fs"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"mochi-api/web"
)

// SetWebRouter serves the embedded SPA. API and relay routes must be
// registered before this so they take precedence. Any non-API path falls
// back to index.html for client-side routing.
func SetWebRouter(r *gin.Engine) {
	dist, ok := web.DistFS()
	if !ok {
		r.NoRoute(func(c *gin.Context) {
			c.String(http.StatusNotFound, "前端尚未构建，请在 web/ 目录运行 npm run build")
		})
		return
	}

	indexHTML, _ := fs.ReadFile(dist, "index.html")
	fileServer := http.FileServer(http.FS(dist))

	r.NoRoute(func(c *gin.Context) {
		path := strings.TrimPrefix(c.Request.URL.Path, "/")
		if path != "" {
			if f, err := dist.Open(path); err == nil {
				_ = f.Close()
				fileServer.ServeHTTP(c.Writer, c.Request)
				return
			}
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", indexHTML)
	})
}
