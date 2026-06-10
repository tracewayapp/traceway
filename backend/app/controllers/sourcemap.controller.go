package controllers

import (
	"context"
	"fmt"
	"github.com/tracewayapp/traceway/backend/app/middleware"
	"github.com/tracewayapp/traceway/backend/app/services"
	"github.com/tracewayapp/traceway/backend/app/storage"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	traceway "go.tracewayapp.com"
)

type sourceMapController struct{}

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
	var storedNames []string
	defer func() {
		if len(storedNames) == 0 {
			return
		}
		genCtx := context.WithoutCancel(c.Request.Context())
		go func() {
			defer traceway.Recover()
			services.GenerateTWArtifacts(genCtx, projectId, storedNames)
		}()
	}()
	for _, fileHeader := range files {
		switch filepath.Ext(fileHeader.Filename) {
		case ".map", ".js", ".cjs", ".mjs":
		default:
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

		storageKey := services.SourceMapStorageKey(projectId, fileHeader.Filename)
		if err := storage.Store.Write(c, storageKey, data); err != nil {
			c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to write source map to storage: %w", err))
			return
		}
		services.InvalidateSourceMap(projectId, fileHeader.Filename)
		storedNames = append(storedNames, fileHeader.Filename)

		if debugId := services.ExtractDebugId(fileHeader.Filename, data); debugId != "" {
			aliasName := services.DebugIdBundleName(debugId)
			if strings.HasSuffix(fileHeader.Filename, ".map") {
				aliasName = services.DebugIdMapName(debugId)
			}
			aliasKey := services.SourceMapStorageKey(projectId, aliasName)
			if err := storage.Store.Write(c, aliasKey, data); err != nil {
				c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to write debug id artifact to storage: %w", err))
				return
			}
			services.InvalidateSourceMap(projectId, aliasName)
			storedNames = append(storedNames, aliasName)
		}

		uploaded++
	}

	c.JSON(http.StatusOK, gin.H{"uploaded": uploaded})
}

var SourceMapController = sourceMapController{}
