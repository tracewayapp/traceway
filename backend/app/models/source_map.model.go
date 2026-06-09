package models

import (
	"time"

	"github.com/google/uuid"
)

type SourceMap struct {
	Id         int       `json:"id"`
	ProjectId  uuid.UUID `json:"projectId"`
	Version    string    `json:"version"`
	FileName   string    `json:"fileName"`
	DebugId    string    `json:"debugId"`
	StorageKey string    `json:"storageKey"`
	FileSize   int64     `json:"fileSize"`
	UploadedAt time.Time `json:"uploadedAt"`
}

// SourceMapFlattenMigration records that a project's legacy versioned source map
// objects (sourcemaps/{project}/{version}/{file}) have been copied to the flat
// sourcemaps/{project}/{file} layout. One row per migrated project makes the
// startup flatten migration idempotent and resumable.
type SourceMapFlattenMigration struct {
	Id         int       `json:"id"`
	ProjectId  uuid.UUID `json:"projectId"`
	MigratedAt time.Time `json:"migratedAt"`
}

// SourceMapProjectId is a result model for selecting distinct project ids from
// the source_maps table during the flatten migration.
type SourceMapProjectId struct {
	ProjectId uuid.UUID `json:"projectId"`
}
