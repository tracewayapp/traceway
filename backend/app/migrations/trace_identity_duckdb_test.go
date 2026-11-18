//go:build telemetry_duckdb

package migrations

import (
	"database/sql"
	"io/fs"
	"testing"

	_ "github.com/duckdb/duckdb-go/v2"
)

func traceIdentityDatabase(t *testing.T) (*sql.DB, fs.FS, string, string, func(fs.FS) error) {
	target, err := sql.Open("duckdb", "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { target.Close() })
	return target, migrationsDuckDBTelemetryFS, "duckdb_telemetry", "0006", func(source fs.FS) error {
		return runMigrationsOn(target, source, "duckdb_telemetry", "schema_migrations", duckdbTrackingDDL)
	}
}
