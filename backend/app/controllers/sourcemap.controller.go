package controllers

import (
	"context"
	"fmt"
	"github.com/tracewayapp/traceway/backend/v2/app/middleware"
	"github.com/tracewayapp/traceway/backend/v2/app/services"
	"github.com/tracewayapp/traceway/backend/v2/app/storage"
	"github.com/tracewayapp/traceway/backend/v2/app/traceway"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
)

type sourceMapController struct{}

const (
	sourceMapMaxFileBytes     = 50 << 20
	sourceMapStoreConcurrency = 8
	sourceMapStoreBudgetBytes = 128 << 20
)

// Process-wide: every concurrent upload shares one bound on the file bytes
// held in memory while they are written, so parallel writes cannot multiply
// the per-upload footprint by UPLOAD_MAX_CONCURRENT.
var sourceMapStoreBudget = semaphore.NewWeighted(sourceMapStoreBudgetBytes)

// keyedWriteOrder keeps the outcome of concurrent writes equal to writing the
// files in upload order: per storage key, a write from an earlier file never
// lands after one from a later file.
type keyedWriteOrder struct {
	mu   sync.Mutex
	keys map[string]*keyedWrite
}

type keyedWrite struct {
	mu      sync.Mutex
	written int
}

func newKeyedWriteOrder() *keyedWriteOrder {
	return &keyedWriteOrder{keys: map[string]*keyedWrite{}}
}

func (o *keyedWriteOrder) write(key string, index int, fn func() error) error {
	o.mu.Lock()
	state := o.keys[key]
	if state == nil {
		state = &keyedWrite{written: -1}
		o.keys[key] = state
	}
	o.mu.Unlock()

	state.mu.Lock()
	defer state.mu.Unlock()
	if state.written > index {
		return nil
	}
	if err := fn(); err != nil {
		return err
	}
	state.written = index
	return nil
}

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
	write := func(ctx context.Context, name, key string, data []byte) error {
		writeCtx, writeSpan := traceway.StartSpan(ctx, name)
		defer writeSpan.End()
		traceway.SetAttribute(writeCtx, "upload.bytes", strconv.Itoa(len(data)))
		return storage.Store.Write(writeCtx, key, data)
	}
	invalidate := func(name string) {
		_, invalidateSpan := traceway.StartSpan(spanCtx, "sourcemap.invalidate_cache")
		defer invalidateSpan.End()
		services.InvalidateSourceMap(projectId, name)
	}

	var accepted []*multipart.FileHeader
	for _, fileHeader := range files {
		switch filepath.Ext(fileHeader.Filename) {
		case ".map", ".js", ".cjs", ".mjs":
		default:
			continue
		}
		if fileHeader.Size > sourceMapMaxFileBytes {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("File %s exceeds 50MB limit", fileHeader.Filename)})
			return
		}
		accepted = append(accepted, fileHeader)
	}

	var storedMu sync.Mutex
	var storedNames []string
	defer func() {
		if len(storedNames) == 0 {
			return
		}
		services.GenerateTWArtifactsAsync(c.Request.Context(), projectId, storedNames)
	}()

	order := newKeyedWriteOrder()
	group, groupCtx := errgroup.WithContext(spanCtx)
	group.SetLimit(sourceMapStoreConcurrency)
	traceway.SetAttribute(spanCtx, "upload.store_concurrency", strconv.Itoa(sourceMapStoreConcurrency))

	store := func(index int, spanName, name string, data []byte) error {
		key := services.SourceMapStorageKey(projectId, name)
		return order.write(key, index, func() error {
			if err := write(groupCtx, spanName, key, data); err != nil {
				return err
			}
			invalidate(name)
			storedMu.Lock()
			storedNames = append(storedNames, name)
			storedMu.Unlock()
			return nil
		})
	}

	for index, fileHeader := range accepted {
		group.Go(func() error {
			if groupCtx.Err() != nil {
				return nil
			}
			budget := min(max(fileHeader.Size, 1), sourceMapStoreBudgetBytes)
			if err := sourceMapStoreBudget.Acquire(groupCtx, budget); err != nil {
				return nil
			}
			defer sourceMapStoreBudget.Release(budget)

			readCtx, readSpan := traceway.StartSpan(spanCtx, "sourcemap.read_file")
			f, err := fileHeader.Open()
			if err != nil {
				readSpan.End()
				return traceway.NewStackTraceErrorf("failed to open uploaded file %s: %w", fileHeader.Filename, err)
			}
			data, err := io.ReadAll(f)
			f.Close()
			traceway.SetAttribute(readCtx, "upload.bytes", strconv.Itoa(len(data)))
			readSpan.End()
			if err != nil {
				return traceway.NewStackTraceErrorf("failed to read uploaded file %s: %w", fileHeader.Filename, err)
			}

			if err := store(index, "sourcemap.write_file", fileHeader.Filename, data); err != nil {
				return traceway.NewStackTraceErrorf("failed to write source map to storage: %w", err)
			}

			_, debugSpan := traceway.StartSpan(spanCtx, "sourcemap.extract_debug_id")
			debugId := services.ExtractDebugId(fileHeader.Filename, data)
			debugSpan.End()
			if debugId == "" {
				return nil
			}
			aliasName := services.DebugIdBundleName(debugId)
			if strings.HasSuffix(fileHeader.Filename, ".map") {
				aliasName = services.DebugIdMapName(debugId)
			}
			if err := store(index, "sourcemap.write_debug_id_alias", aliasName, data); err != nil {
				return traceway.NewStackTraceErrorf("failed to write debug id artifact to storage: %w", err)
			}
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	uploaded := len(accepted)

	c.JSON(http.StatusOK, gin.H{"uploaded": uploaded})
}

var SourceMapController = sourceMapController{}
