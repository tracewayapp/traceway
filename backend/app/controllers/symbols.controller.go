package controllers

import (
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/tracewayapp/traceway/backend/app/middleware"
	"github.com/tracewayapp/traceway/backend/app/services"
	"github.com/tracewayapp/traceway/backend/app/storage"
	"github.com/tracewayapp/traceway/backend/app/symbolicator/dart"

	"github.com/gin-gonic/gin"
	traceway "go.tracewayapp.com"
)

type symbolsController struct{}

func (s symbolsController) Upload(c *gin.Context) {
	projectId, err := middleware.GetProjectId(c)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("UseSourceMapAuth middleware must be applied: %w", err))
		return
	}

	if err := c.Request.ParseMultipartForm(200 << 20); err != nil {
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
		if filepath.Ext(fileHeader.Filename) != ".symbols" {
			continue
		}
		if fileHeader.Size > 200<<20 {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("File %s exceeds 200MB limit", fileHeader.Filename)})
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

		arch := c.PostForm("arch")
		if arch == "" {
			arch = archFromSymbolsFilename(fileHeader.Filename)
		}
		if arch == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("cannot determine arch for %s; pass an 'arch' field", fileHeader.Filename)})
			return
		}
		if !dart.IsValidArch(arch) {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid arch %q for %s; expected an architecture token like arm64, x64, arm, or ia32", arch, fileHeader.Filename)})
			return
		}

		buildID, err := dart.ReadBuildID(data)
		if err != nil {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": fmt.Sprintf("%s is not a valid Dart symbols file: %v", fileHeader.Filename, err)})
			return
		}

		debugId := services.NormalizeDartDebugId(c.PostForm("debug_id"))
		if note := services.NormalizeDartDebugId(buildID); note != "" {
			if debugId != "" && debugId != note {
				c.JSON(http.StatusUnprocessableEntity, gin.H{"error": fmt.Sprintf("debug_id %s does not match the build-id note %s in %s", debugId, note, fileHeader.Filename)})
				return
			}
			debugId = note
		}
		if debugId == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("no debug_id for %s; the file has no build-id note, so pass a 'debug_id' field (the Mach-O UUID)", fileHeader.Filename)})
			return
		}

		key := services.DartSymbolsKey(projectId, debugId, arch)
		if err := storage.Store.Write(c, key, data); err != nil {
			c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to write symbols to storage: %w", err))
			return
		}
		services.InvalidateDartSymbols(key)

		uploaded++
	}

	c.JSON(http.StatusOK, gin.H{"uploaded": uploaded})
}

func archFromSymbolsFilename(name string) string {
	base := strings.TrimSuffix(filepath.Base(name), ".symbols")
	if i := strings.LastIndex(base, "-"); i != -1 {
		return base[i+1:]
	}
	return ""
}

var SymbolsController = symbolsController{}
