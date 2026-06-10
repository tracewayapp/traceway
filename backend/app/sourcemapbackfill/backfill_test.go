//go:build !pgch

package sourcemapbackfill

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/lit/v2"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/storage"
	_ "modernc.org/sqlite"
)

type memStore struct {
	mu   sync.Mutex
	data map[string][]byte
}

func (m *memStore) Write(_ context.Context, key string, data []byte) error {
	m.mu.Lock()
	m.data[key] = append([]byte(nil), data...)
	m.mu.Unlock()
	return nil
}

func (m *memStore) Delete(_ context.Context, key string) error {
	m.mu.Lock()
	delete(m.data, key)
	m.mu.Unlock()
	return nil
}

func (m *memStore) Read(_ context.Context, key string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	d, ok := m.data[key]
	if !ok {
		return nil, storage.ErrNotFound
	}
	return append([]byte(nil), d...), nil
}

func (m *memStore) get(key string) (string, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	d, ok := m.data[key]
	return string(d), ok
}

func setup(t *testing.T) *memStore {
	t.Helper()

	mainDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open main db: %v", err)
	}
	mainDB.SetMaxOpenConns(1)

	ddl := []string{
		`CREATE TABLE source_maps (id INTEGER PRIMARY KEY, project_id TEXT NOT NULL, version TEXT NOT NULL, file_name TEXT NOT NULL, storage_key TEXT NOT NULL, file_size INTEGER NOT NULL, uploaded_at DATETIME NOT NULL)`,
		`CREATE TABLE source_map_flatten_migrations (id INTEGER PRIMARY KEY, project_id TEXT NOT NULL UNIQUE, migrated_at DATETIME NOT NULL)`,
	}
	for _, stmt := range ddl {
		if _, err := mainDB.Exec(stmt); err != nil {
			t.Fatalf("ddl: %v", err)
		}
	}

	prevDB, prevDriver, prevStore := db.DB, db.Driver, storage.Store
	db.DB = mainDB
	db.Driver = lit.SQLite
	models.Init(db.Driver)

	ms := &memStore{data: map[string][]byte{}}
	storage.Store = ms

	t.Cleanup(func() {
		mainDB.Close()
		db.DB = prevDB
		db.Driver = prevDriver
		storage.Store = prevStore
	})
	return ms
}

func seedMap(t *testing.T, ms *memStore, projectId uuid.UUID, version, fileName string, body string, uploadedAt time.Time) {
	t.Helper()
	storageKey := fmt.Sprintf("sourcemaps/%s/%s/%s", projectId, version, fileName)
	sm := models.SourceMap{
		ProjectId:  projectId,
		Version:    version,
		FileName:   fileName,
		StorageKey: storageKey,
		FileSize:   int64(len(body)),
		UploadedAt: uploadedAt,
	}
	if _, err := lit.Insert(db.DB, &sm); err != nil {
		t.Fatalf("seed source_map: %v", err)
	}
	ms.data[storageKey] = []byte(body)
}

func countMigrations(t *testing.T) int {
	t.Helper()
	var n int
	if err := db.DB.QueryRow("SELECT count(*) FROM source_map_flatten_migrations").Scan(&n); err != nil {
		t.Fatalf("count migrations: %v", err)
	}
	return n
}

func TestFlattenCopiesNewestVersionToFlatKey(t *testing.T) {
	ms := setup(t)
	p := uuid.New()
	now := time.Now().UTC()
	seedMap(t, ms, p, "v1", "app.js.map", "OLD", now.Add(-2*time.Hour))
	seedMap(t, ms, p, "v2", "app.js.map", "NEW", now.Add(-1*time.Hour))

	run(context.Background())

	flatKey := fmt.Sprintf("sourcemaps/%s/app.js.map", p)
	got, ok := ms.get(flatKey)
	if !ok || got != "NEW" {
		t.Errorf("flat key = %q (present=%v), want newest content NEW", got, ok)
	}
	if n := countMigrations(t); n != 1 {
		t.Errorf("migration rows = %d, want 1", n)
	}
	if _, ok := ms.get(fmt.Sprintf("sourcemaps/%s/v1/app.js.map", p)); !ok {
		t.Error("old v1 object should be left in place (copy-only)")
	}
	if _, ok := ms.get(fmt.Sprintf("sourcemaps/%s/v2/app.js.map", p)); !ok {
		t.Error("old v2 object should be left in place (copy-only)")
	}
}

func TestFlattenIsIdempotentAcrossRuns(t *testing.T) {
	ms := setup(t)
	p := uuid.New()
	seedMap(t, ms, p, "v1", "app.js.map", "BODY", time.Now().UTC())

	run(context.Background())
	run(context.Background())

	if n := countMigrations(t); n != 1 {
		t.Errorf("migration rows after two runs = %d, want 1", n)
	}
}

func TestFlattenRetryDoesNotOverwriteFreshUploads(t *testing.T) {
	ms := setup(t)
	p := uuid.New()
	seedMap(t, ms, p, "v1", "app.js.map", "STALE", time.Now().UTC())

	flatKey := fmt.Sprintf("sourcemaps/%s/app.js.map", p)
	ms.data[flatKey] = []byte("FRESH_UPLOAD")

	run(context.Background())

	if got, _ := ms.get(flatKey); got != "FRESH_UPLOAD" {
		t.Errorf("flat key = %q, a backfill retry must not overwrite a newer upload", got)
	}
	if n := countMigrations(t); n != 1 {
		t.Errorf("migration rows = %d, want 1 (project should still be marked migrated)", n)
	}
}

func TestFlattenSkipsAlreadyMigratedProjects(t *testing.T) {
	ms := setup(t)
	p := uuid.New()
	seedMap(t, ms, p, "v1", "app.js.map", "FIRST", time.Now().UTC())

	run(context.Background())

	flatKey := fmt.Sprintf("sourcemaps/%s/app.js.map", p)
	ms.data[flatKey] = []byte("SECOND")
	seedMap(t, ms, p, "v2", "app.js.map", "FROM_OLD_ROW", time.Now().UTC().Add(time.Hour))

	run(context.Background())

	if got, _ := ms.get(flatKey); got != "SECOND" {
		t.Errorf("flat key = %q, want SECOND untouched", got)
	}
	if n := countMigrations(t); n != 1 {
		t.Errorf("migration rows = %d, want 1", n)
	}
}
