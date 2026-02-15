package controllers

import (
	"backend/app/middleware"
	"backend/app/models"
	"backend/app/pgdb"
	"backend/app/repositories"
	"backend/app/storage"
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

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

	version := c.Request.FormValue("version")
	if version == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "version is required"})
		return
	}

	files := c.Request.MultipartForm.File["files"]
	if len(files) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No files uploaded"})
		return
	}

	uploaded := 0
	for _, fileHeader := range files {
		if !strings.HasSuffix(fileHeader.Filename, ".map") {
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

		storageKey := fmt.Sprintf("sourcemaps/%s/%s/%s", projectId, version, fileHeader.Filename)

		if err := storage.Store.Write(c, storageKey, data); err != nil {
			c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to write source map to storage: %w", err))
			return
		}

		_, err = pgdb.ExecuteTransaction(func(tx *sql.Tx) (*models.SourceMap, error) {
			existing, err := repositories.SourceMapRepository.FindByProjectVersionAndFileName(tx, projectId, version, fileHeader.Filename)
			if err != nil {
				return nil, err
			}
			if existing != nil {
				existing.StorageKey = storageKey
				existing.FileSize = fileHeader.Size
				existing.UploadedAt = time.Now().UTC()
				return existing, repositories.SourceMapRepository.Update(tx, existing)
			}
			return repositories.SourceMapRepository.Create(tx, &models.SourceMap{
				ProjectId:  projectId,
				Version:    version,
				FileName:   fileHeader.Filename,
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
