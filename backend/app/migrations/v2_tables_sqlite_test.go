//go:build !telemetry_ch && !telemetry_duckdb

package migrations

import "testing"

func TestV2MigrationsOnlyCreate(t *testing.T) {
	assertV2MigrationsOnlyCreate(t, migrationsSqliteTelemetryFS, "sqlite_telemetry")
}
