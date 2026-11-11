package cmd

import (
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/gin-gonic/gin"
)

func assetRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	bundle := strings.Repeat("export const a = 1;\n", 200)
	fsys := fstest.MapFS{
		"immutable/chunks/a.js": {Data: []byte(bundle)},
		"version.json":          {Data: []byte(`{"version":"1"}`)},
	}
	r := gin.New()
	r.Match([]string{http.MethodGet, http.MethodHead}, "/_app/*filepath", appAssetHandler(fsys))
	r.POST("/api/report", func(c *gin.Context) { c.String(http.StatusOK, strings.Repeat("x", 4096)) })
	r.GET("/api/stream", func(c *gin.Context) {
		c.Header("Content-Type", "text/event-stream")
		c.String(http.StatusOK, strings.Repeat("data: x\n\n", 500))
	})
	r.GET("/api/big", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"v": strings.Repeat("y", 8192)}) })
	return r
}

func serve(h http.Handler, method, target string, gzipOK bool) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, nil)
	if gzipOK {
		req.Header.Set("Accept-Encoding", "gzip")
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

func TestAppAssetCacheHeaders(t *testing.T) {
	r := assetRouter()

	w := serve(r, http.MethodGet, "/_app/immutable/chunks/a.js", false)
	if w.Code != http.StatusOK || w.Header().Get("Cache-Control") != immutableCacheControl {
		t.Fatalf("immutable asset: status %d, Cache-Control %q", w.Code, w.Header().Get("Cache-Control"))
	}
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/javascript") {
		t.Fatalf("immutable asset content type %q", ct)
	}

	w = serve(r, http.MethodGet, "/_app/version.json", false)
	if w.Code != http.StatusOK || w.Header().Get("Cache-Control") != "no-cache" {
		t.Fatalf("version.json: status %d, Cache-Control %q", w.Code, w.Header().Get("Cache-Control"))
	}

	for _, target := range []string{"/_app/immutable/missing.js", "/_app/immutable/chunks", "/_app/immutable/"} {
		w = serve(r, http.MethodGet, target, false)
		if w.Code != http.StatusNotFound || w.Header().Get("Cache-Control") != "" {
			t.Fatalf("%s: status %d, Cache-Control %q; a missing file must never be cached", target, w.Code, w.Header().Get("Cache-Control"))
		}
	}

	w = serve(r, http.MethodHead, "/_app/immutable/chunks/a.js", false)
	if w.Code != http.StatusOK {
		t.Fatalf("HEAD immutable asset: status %d", w.Code)
	}
}

func TestCompressResponses(t *testing.T) {
	h := compressResponses(assetRouter())

	for _, target := range []string{"/_app/immutable/chunks/a.js", "/api/big"} {
		w := serve(h, http.MethodGet, target, true)
		if w.Header().Get("Content-Encoding") != "gzip" {
			t.Fatalf("%s was not compressed", target)
		}
		zr, err := gzip.NewReader(w.Body)
		if err != nil {
			t.Fatal(err)
		}
		if body, err := io.ReadAll(zr); err != nil || len(body) < 4000 {
			t.Fatalf("%s: decompressed %d bytes, err %v", target, len(body), err)
		}
	}

	if w := serve(h, http.MethodGet, "/api/big", false); w.Header().Get("Content-Encoding") != "" {
		t.Fatal("compressed a response for a client that did not ask for it")
	}
	if w := serve(h, http.MethodPost, "/api/report", true); w.Header().Get("Content-Encoding") != "" {
		t.Fatal("compressed an ingest response")
	}
	if w := serve(h, http.MethodGet, "/api/stream", true); w.Header().Get("Content-Encoding") != "" {
		t.Fatal("compressed an event stream")
	}
}
