package cmd

import (
	"io/fs"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/klauspost/compress/gzhttp"
)

const immutableCacheControl = "public, max-age=31536000, immutable"

// SvelteKit content-hashes every file under _app/immutable, so a browser can
// keep them forever; everything else under _app (version.json) must be
// revalidated or a deploy is never noticed. Files are served from the embedded
// FS, which carries no modification time, so without these headers every page
// load downloaded the whole bundle again.
func appAssetHandler(fsys fs.FS) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := strings.TrimPrefix(c.Param("filepath"), "/")
		data, err := fs.ReadFile(fsys, name)
		if err != nil {
			c.Status(http.StatusNotFound)
			return
		}
		if strings.HasPrefix(name, "immutable/") {
			c.Header("Cache-Control", immutableCacheControl)
		} else {
			c.Header("Cache-Control", "no-cache")
		}
		c.Data(http.StatusOK, detectContentType(name), data)
	}
}

// Ingest answers are a few bytes and the MCP and runner routes stream or
// long-poll, so only the dashboard API and static files are compressed.
var uncompressedPathPrefixes = []string{
	"/api/report",
	"/api/otel/",
	"/api/profiles/ingest",
	"/api/runners/",
	"/mcp",
}

func compressResponses(next http.Handler) http.Handler {
	wrap, err := gzhttp.NewWrapper(
		gzhttp.MinSize(1024),
		gzhttp.ExceptContentTypes([]string{"text/event-stream"}),
	)
	if err != nil {
		panic(err)
	}
	compressed := wrap(next)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, prefix := range uncompressedPathPrefixes {
			if strings.HasPrefix(r.URL.Path, prefix) {
				next.ServeHTTP(w, r)
				return
			}
		}
		compressed.ServeHTTP(w, r)
	})
}
