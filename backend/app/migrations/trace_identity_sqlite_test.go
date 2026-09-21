//go:build !telemetry_ch && !telemetry_duckdb

package migrations

import (
	"database/sql"
	"io/fs"
	"testing"
)

func traceIdentityDatabase(t *testing.T) (*sql.DB, fs.FS, string, string, func(fs.FS) error) {
	target := openMemorySQLite(t)
	return target, migrationsSqliteTelemetryFS, "sqlite_telemetry", "0026", func(source fs.FS) error {
		return runMigrationsOn(target, source, "sqlite_telemetry", "schema_migrations", sqliteTrackingDDL)
	}
}
