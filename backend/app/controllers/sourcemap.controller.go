package controllers

import (
	"fmt"
	"github.com/tracewayapp/traceway/backend/app/middleware"
	"github.com/tracewayapp/traceway/backend/app/services"
	"github.com/tracewayapp/traceway/backend/app/storage"
	"io"
	"net/http"
	"path/filepath"

	"github.com/gin-gonic/gin"
	traceway "go.tracewayapp.com"
)

type sourceMapController struct{}

func isSourceArtifact(name string) bool {
	switch filepath.Ext(name) {
	case ".map", ".js", ".cjs", ".mjs":
		return true
	default:
		return false
	}
}

func (s sourceMapController) Upload(c *gin.Context) {
	projectId, err := middleware.GetProjectId(c)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("UseSourceMapAuth middleware must be applied: %w", err))
		return
	}

	if err := c.Request.ParseMultipartForm(50 << 20); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse multipart form"})
		return
	}

	files := c.Request.MultipartForm.File["files"]
	if len(files) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No files uploaded"})
		return
	}

	uploaded := 0
	for _, fileHeader := range files {
		if !isSourceArtifact(fileHeader.Filename) {
			continue
		}

		if fileHeader.Size > 50<<20 {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("File %s exceeds 50MB limit", fileHeader.Filename)})
			return
		}

		f, err := fileHeader.Open()
		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to open uploaded file %s: %w", fileHeader.Filename, err))
			return
		}

		data, err := io.ReadAll(f)
		f.Close()
		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to read uploaded file %s: %w", fileHeader.Filename, err))
			return
		}

		storageKey := fmt.Sprintf("sourcemaps/%s/%s", projectId, fileHeader.Filename)
		if err := storage.Store.Write(c, storageKey, data); err != nil {
			c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to write source map to storage: %w", err))
			return
		}
		services.InvalidateSourceMap(projectId, fileHeader.Filename)

		uploaded++
	}

	c.JSON(http.StatusOK, gin.H{"uploaded": uploaded})
}

var SourceMapController = sourceMapController{}
