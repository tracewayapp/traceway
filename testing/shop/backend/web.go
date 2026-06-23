package main

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

//go:embed all:dist
var distFS embed.FS

func registerFrontend(router *gin.Engine) {
	dist, err := fs.Sub(distFS, "dist")
	if err != nil {
		panic(err)
	}
	fileServer := http.FileServer(http.FS(dist))

	router.NoRoute(func(c *gin.Context) {
		path := strings.TrimPrefix(c.Request.URL.Path, "/")
		if path != "" {
			if _, err := fs.Stat(dist, path); err != nil {
				c.Request.URL.Path = "/"
			}
		}
		fileServer.ServeHTTP(c.Writer, c.Request)
	})
}
