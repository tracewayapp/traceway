//go:build duckdb && !pgch

package migrations

import (
	"embed"
	"fmt"

	"github.com/tracewayapp/traceway/backend/app/db"
)

//go:embed duckdb_telemetry/*.sql
var migrationsDuckDBTelemetryFS embed.FS

const duckdbTrackingDDL = `CREATE TABLE IF NOT EXISTS %s (
	version VARCHAR PRIMARY KEY,
	applied_at TIMESTAMP DEFAULT now()
)`

func Run(dbType string) error {
	if err := runMigrationsOn(db.DB, migrationsSqliteFS, "sqlite", "schema_migrations", sqliteTrackingDDL); err != nil {
		return fmt.Errorf("main db migrations: %w", err)
	}

	if err := runMigrationsOn(db.TelemetryDB, migrationsDuckDBTelemetryFS, "duckdb_telemetry", "schema_migrations", duckdbTrackingDDL); err != nil {
		return fmt.Errorf("telemetry db migrations: %w", err)
	}

	return nil
}
