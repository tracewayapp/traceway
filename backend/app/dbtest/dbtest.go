// Package dbtest boots the dual in-memory SQLite databases for tests that
// exercise the default (untagged) storage backends.
package dbtest

import (
	"database/sql"
	"testing"

	"github.com/tracewayapp/lit/v2"
	"github.com/tracewayapp/traceway/backend/app/config"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/migrations"
	"github.com/tracewayapp/traceway/backend/app/models"
	_ "modernc.org/sqlite"
)

// SetupSQLite points db.DB and db.TelemetryDB at fresh in-memory SQLite
// databases, initializes config and models, and runs the sqlite migrations.
// Cleanup closes both databases.
func SetupSQLite(t *testing.T) {
	t.Helper()

	openMemory := func(name string) *sql.DB {
		conn, err := sql.Open("sqlite", ":memory:")
		if err != nil {
			t.Fatalf("open in-memory sqlite (%s): %v", name, err)
		}
		conn.SetMaxOpenConns(1)
		if _, err := conn.Exec("PRAGMA foreign_keys = ON"); err != nil {
			t.Fatalf("enable foreign keys: %v", err)
		}
		return conn
	}
	db.DB = openMemory("main")
	db.TelemetryDB = openMemory("telemetry")
	db.Driver = lit.SQLite
	if config.Config == nil {
		config.Init(config.LoadFromEnv())
	}
	models.Init(db.Driver)
	if err := migrations.Run("sqlite"); err != nil {
		t.Fatalf("run migrations: %v", err)
	}
	t.Cleanup(func() {
		db.DB.Close()
		db.TelemetryDB.Close()
	})
}
