package controllers

import (
	"github.com/tracewayapp/traceway/backend/app/middleware"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/repositories"
	"github.com/tracewayapp/traceway/backend/app/storage"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	traceway "go.tracewayapp.com"
)

type sourceMapController struct{}

var jsDebugIdRe = regexp.MustCompile(`//# debugId=([0-9a-fA-F-]{36})`)

func isSourceArtifact(name string) bool {
	switch filepath.Ext(name) {
	case ".map", ".js", ".cjs", ".mjs":
		return true
	default:
		return false
	}
}

func extractDebugId(fileName string, data []byte) string {
	if strings.HasSuffix(fileName, ".map") {
		var m struct {
			DebugId      string `json:"debugId"`
			DebugIdSnake string `json:"debug_id"`
		}
		if err := json.Unmarshal(data, &m); err == nil {
			if m.DebugId != "" {
				return m.DebugId
			}
			return m.DebugIdSnake
		}
		return ""
	}
	if match := jsDebugIdRe.FindSubmatch(data); match != nil {
		return string(match[1])
	}
	return ""
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

	version := c.Request.FormValue("version")
	if version == "" {
		version = "unversioned"
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

		debugId := extractDebugId(fileHeader.Filename, data)
		storageKey := fmt.Sprintf("sourcemaps/%s/%s/%s", projectId, version, fileHeader.Filename)

		if err := storage.Store.Write(c, storageKey, data); err != nil {
			c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to write source map to storage: %w", err))
			return
		}

		_, err = db.ExecuteTransaction(func(tx *sql.Tx) (*models.SourceMap, error) {
			existing, err := repositories.SourceMapRepository.FindByProjectVersionAndFileName(tx, projectId, version, fileHeader.Filename)
			if err != nil {
				return nil, err
			}
			if existing != nil {
				existing.StorageKey = storageKey
				existing.FileSize = fileHeader.Size
				existing.DebugId = debugId
				existing.UploadedAt = time.Now().UTC()
				return existing, repositories.SourceMapRepository.Update(tx, existing)
			}
			return repositories.SourceMapRepository.Create(tx, &models.SourceMap{
				ProjectId:  projectId,
				Version:    version,
				FileName:   fileHeader.Filename,
				DebugId:    debugId,
				StorageKey: storageKey,
				FileSize:   fileHeader.Size,
			})
		})
		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to upsert source map metadata: %w", err))
			return
		}

		uploaded++
	}

	c.JSON(http.StatusOK, gin.H{"uploaded": uploaded})
}

var SourceMapController = sourceMapController{}
