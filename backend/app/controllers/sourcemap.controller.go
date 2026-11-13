package controllers

import (
	"context"
	"fmt"
	"github.com/tracewayapp/traceway/backend/v2/app/middleware"
	"github.com/tracewayapp/traceway/backend/v2/app/services"
	"github.com/tracewayapp/traceway/backend/v2/app/storage"
	"github.com/tracewayapp/traceway/backend/v2/app/traceway"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

type sourceMapController struct{}

func (s sourceMapController) Upload(c *gin.Context) {
	projectId, err := middleware.GetProjectId(c)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("UseSourceMapAuth middleware must be applied: %w", err))
		return
	}

	_, parseSpan := traceway.StartSpan(c, "reading source map upload")
	parsed := parseMultipartCapped(c, maxUploadBytes, uploadMultipartMemory)
	parseSpan.End()
	if !parsed {
		return
	}
	defer c.Request.MultipartForm.RemoveAll()

	files := c.Request.MultipartForm.File["files"]
	if len(files) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No files uploaded"})
		return
	}

	spanCtx, span := traceway.StartSpan(c, "storing source map upload")
	defer span.End()
	traceway.SetAttribute(spanCtx, "upload.file_count", strconv.Itoa(len(files)))
	write := func(name, key string, data []byte) error {
		writeCtx, writeSpan := traceway.StartSpan(spanCtx, name)
		defer writeSpan.End()
		traceway.SetAttribute(writeCtx, "upload.bytes", strconv.Itoa(len(data)))
		return storage.Store.Write(writeCtx, key, data)
	}
	invalidate := func(name string) {
		_, invalidateSpan := traceway.StartSpan(spanCtx, "sourcemap.invalidate_cache")
		defer invalidateSpan.End()
		services.InvalidateSourceMap(projectId, name)
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

		readCtx, readSpan := traceway.StartSpan(spanCtx, "sourcemap.read_file")
		f, err := fileHeader.Open()
		if err != nil {
			readSpan.End()
			c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to open uploaded file %s: %w", fileHeader.Filename, err))
			return
		}

		data, err := io.ReadAll(f)
		f.Close()
		traceway.SetAttribute(readCtx, "upload.bytes", strconv.Itoa(len(data)))
		readSpan.End()
		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to read uploaded file %s: %w", fileHeader.Filename, err))
			return
		}

		storageKey := services.SourceMapStorageKey(projectId, fileHeader.Filename)
		if err := write("sourcemap.write_file", storageKey, data); err != nil {
			c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to write source map to storage: %w", err))
			return
		}
		invalidate(fileHeader.Filename)
		storedNames = append(storedNames, fileHeader.Filename)

		_, debugSpan := traceway.StartSpan(spanCtx, "sourcemap.extract_debug_id")
		debugId := services.ExtractDebugId(fileHeader.Filename, data)
		debugSpan.End()
		if debugId != "" {
			aliasName := services.DebugIdBundleName(debugId)
			if strings.HasSuffix(fileHeader.Filename, ".map") {
				aliasName = services.DebugIdMapName(debugId)
			}
			aliasKey := services.SourceMapStorageKey(projectId, aliasName)
			if err := write("sourcemap.write_debug_id_alias", aliasKey, data); err != nil {
				c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to write debug id artifact to storage: %w", err))
				return
			}
			invalidate(aliasName)
			storedNames = append(storedNames, aliasName)
		}

		uploaded++
	}

	c.JSON(http.StatusOK, gin.H{"uploaded": uploaded})
}

var SourceMapController = sourceMapController{}
