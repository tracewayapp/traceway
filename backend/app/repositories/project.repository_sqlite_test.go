//go:build !pgch

package repositories

import (
	"testing"

	"github.com/tracewayapp/traceway/backend/app/db"
)

func setupProjectsTable(t *testing.T) {
	t.Helper()
	_, err := db.DB.Exec(`CREATE TABLE IF NOT EXISTS projects (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		token TEXT NOT NULL,
		framework TEXT NOT NULL DEFAULT 'custom',
		organization_id INTEGER,
		source_map_token TEXT,
		created_at DATETIME NOT NULL DEFAULT (datetime('now')),
		drop_healthy_healthchecks INTEGER NOT NULL DEFAULT 1,
		healthcheck_paths TEXT NOT NULL DEFAULT '[]'
	)`)
	if err != nil {
		t.Fatalf("failed to create projects table: %v", err)
	}
}

func TestProjectHealthcheckFieldsRoundTrip(t *testing.T) {
	setupTestDB(t)
	setupProjectsTable(t)

	tx, err := db.DB.Begin()
	if err != nil {
		t.Fatalf("failed to begin tx: %v", err)
	}
	defer tx.Rollback()

	created, err := ProjectRepository.Create(tx, "test-project", "gin")
	if err != nil {
		t.Fatalf("failed to create project: %v", err)
	}
	if !created.DropHealthyHealthchecks {
		t.Errorf("new project should default DropHealthyHealthchecks to true")
	}

	found, err := ProjectRepository.FindById(tx, created.Id)
	if err != nil {
		t.Fatalf("failed to find project: %v", err)
	}
	if found == nil {
		t.Fatal("project not found after create")
	}
	if !found.DropHealthyHealthchecks {
		t.Errorf("DropHealthyHealthchecks = false after round trip, expected true")
	}
	if len(found.HealthcheckPaths) != 0 {
		t.Errorf("HealthcheckPaths = %v, expected empty", found.HealthcheckPaths)
	}

	disable := false
	paths := []string{"/internal/probe", "/checks/*"}
	updated, err := ProjectRepository.Update(tx, created.Id, "test-project", "gin", &disable, &paths)
	if err != nil {
		t.Fatalf("failed to update project: %v", err)
	}
	if updated.DropHealthyHealthchecks {
		t.Errorf("DropHealthyHealthchecks = true after disabling")
	}

	found, err = ProjectRepository.FindById(tx, created.Id)
	if err != nil {
		t.Fatalf("failed to re-find project: %v", err)
	}
	if found.DropHealthyHealthchecks {
		t.Errorf("DropHealthyHealthchecks = true after round trip, expected false")
	}
	if len(found.HealthcheckPaths) != 2 || found.HealthcheckPaths[0] != "/internal/probe" || found.HealthcheckPaths[1] != "/checks/*" {
		t.Errorf("HealthcheckPaths = %v, expected %v", found.HealthcheckPaths, paths)
	}

	keepDrop := true
	updated, err = ProjectRepository.Update(tx, created.Id, "renamed", "gin", &keepDrop, nil)
	if err != nil {
		t.Fatalf("failed to update project without paths: %v", err)
	}
	if len(updated.HealthcheckPaths) != 2 {
		t.Errorf("nil healthcheckPaths should keep existing value, got %v", updated.HealthcheckPaths)
	}
}
