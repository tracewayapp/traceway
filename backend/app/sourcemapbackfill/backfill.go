package sourcemapbackfill

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/tracewayapp/lit/v2"
	"github.com/tracewayapp/traceway/backend/app/config"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/services"
	"github.com/tracewayapp/traceway/backend/app/storage"

	"github.com/google/uuid"
	traceway "go.tracewayapp.com"
)

const storageOpTimeout = 15 * time.Second

func Start(ctx context.Context) {
	if db.DB == nil || storage.Store == nil {
		return
	}
	go func() {
		defer traceway.Recover()
		run(ctx)
	}()
}

func run(ctx context.Context) {
	projects, err := lit.SelectNamed[models.SourceMapProjectId](
		db.DB,
		"SELECT DISTINCT project_id FROM source_maps WHERE project_id NOT IN (SELECT project_id FROM source_map_flatten_migrations)",
		lit.P{},
	)
	if err != nil {
		traceway.CaptureException(fmt.Errorf("source map flatten: failed to list un-migrated projects: %w", err))
		return
	}
	if len(projects) == 0 {
		return
	}

	migrated := 0
	for _, p := range projects {
		if ctx.Err() != nil {
			return
		}
		if migrateProject(ctx, p.ProjectId) {
			migrated++
		}
	}
	if migrated > 0 {
		config.Logf("source map flatten: migrated %d project(s) to versionless layout", migrated)
	}
}

func migrateProject(ctx context.Context, projectId uuid.UUID) bool {
	rows, err := lit.SelectNamed[models.SourceMap](
		db.DB,
		`SELECT s.id, s.project_id, s.version, s.file_name, s.storage_key, s.file_size, s.uploaded_at FROM source_maps s
		 JOIN (SELECT file_name, MAX(uploaded_at) AS mx FROM source_maps WHERE project_id = :pid GROUP BY file_name) g
		   ON s.file_name = g.file_name AND s.uploaded_at = g.mx
		 WHERE s.project_id = :pid`,
		lit.P{"pid": projectId},
	)
	if err != nil {
		traceway.CaptureException(fmt.Errorf("source map flatten: failed to read source maps for project %s: %w", projectId, err))
		return false
	}

	seen := make(map[string]bool, len(rows))
	for _, row := range rows {
		base := filepath.Base(row.FileName)
		if seen[base] {
			continue
		}
		seen[base] = true

		oldKey := row.StorageKey
		flatKey := fmt.Sprintf("sourcemaps/%s/%s", projectId, base)
		if oldKey == flatKey {
			continue
		}

		if !copyObject(ctx, projectId, oldKey, flatKey) {
			return false
		}
		services.InvalidateSourceMap(projectId, base)
	}

	flattenRow := models.SourceMapFlattenMigration{
		ProjectId:  projectId,
		MigratedAt: time.Now().UTC(),
	}
	if _, err := lit.Insert(db.DB, &flattenRow); err != nil {
		traceway.CaptureException(fmt.Errorf("source map flatten: failed to record migration for project %s: %w", projectId, err))
		return false
	}
	return true
}

func copyObject(ctx context.Context, projectId uuid.UUID, oldKey, flatKey string) bool {
	opCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), storageOpTimeout)
	defer cancel()

	if _, err := storage.Store.Read(opCtx, flatKey); err == nil {
		return true
	} else if !errors.Is(err, storage.ErrNotFound) {
		traceway.CaptureException(fmt.Errorf("source map flatten: failed to check %s for project %s: %w", flatKey, projectId, err))
		return false
	}

	data, err := storage.Store.Read(opCtx, oldKey)
	if errors.Is(err, storage.ErrNotFound) {
		return true
	}
	if err != nil {
		traceway.CaptureException(fmt.Errorf("source map flatten: failed to read %s for project %s: %w", oldKey, projectId, err))
		return false
	}
	if err := storage.Store.Write(opCtx, flatKey, data); err != nil {
		traceway.CaptureException(fmt.Errorf("source map flatten: failed to write %s for project %s: %w", flatKey, projectId, err))
		return false
	}
	return true
}
