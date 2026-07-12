//go:build telemetry_duckdb

package telemetry

import (
	"database/sql"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/duckdb/duckdb-go/v2"
	"github.com/tracewayapp/lit/v2"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
	_ "modernc.org/sqlite"
)

// setupTestDB mirrors the SQLite test harness but pairs an in-memory SQLite main
// DB with an in-memory DuckDB telemetry DB, so the shared telemetry test suite
// exercises the DuckDB repository implementations.
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

	connector, err := duckdb.NewConnector("", nil)
	if err != nil {
		t.Fatalf("failed to open in-memory duckdb (telemetry): %v", err)
	}
	telemetryDB := sql.OpenDB(connector)

	db.DB = mainDB
	db.TelemetryDB = telemetryDB
	db.DuckDBConnector = connector
	db.Driver = lit.SQLite

	models.Init(db.Driver)

	if err := runDuckDBTelemetryMigrations(telemetryDB); err != nil {
		t.Fatalf("failed to run duckdb telemetry migrations: %v", err)
	}

	t.Cleanup(func() {
		mainDB.Close()
		telemetryDB.Close()
	})
}

func runDuckDBTelemetryMigrations(telemetryDB *sql.DB) error {
	// Apply every migration in order, like the production runner, so tests
	// keep seeing the full schema when migrations beyond 0001 are added.
	const dir = "../../migrations/duckdb_telemetry"
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".up.sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	for _, name := range names {
		content, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return err
		}
		for _, stmt := range strings.Split(string(content), ";") {
			stmt = strings.TrimSpace(stmt)
			if stmt == "" {
				continue
			}
			if _, err := telemetryDB.Exec(stmt); err != nil {
				return err
			}
		}
	}
	return nil
}
