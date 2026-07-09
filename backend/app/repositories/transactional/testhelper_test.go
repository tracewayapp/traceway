//go:build !transactional_pg

package transactional

import (
	"database/sql"
	"testing"

	"github.com/tracewayapp/lit/v2"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
	_ "modernc.org/sqlite"
)

func setupTestDB(t *testing.T) {
	t.Helper()

	mainDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory sqlite (main): %v", err)
	}
	mainDB.SetMaxOpenConns(1)
	if _, err := mainDB.Exec("PRAGMA foreign_keys = ON"); err != nil {
		t.Fatalf("failed to enable foreign keys: %v", err)
	}
	if _, err := mainDB.Exec("PRAGMA journal_mode = WAL"); err != nil {
		t.Fatalf("failed to set WAL mode: %v", err)
	}

	db.DB = mainDB
	db.Driver = lit.SQLite

	models.Init(db.Driver)

	t.Cleanup(func() {
		mainDB.Close()
	})
}
