//go:build telemetry_duckdb

package migrations

import "testing"

func TestV2MigrationsOnlyCreate(t *testing.T) {
	assertV2MigrationsOnlyCreate(t, migrationsDuckDBTelemetryFS, "duckdb_telemetry")
}
