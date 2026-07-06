//go:build telemetry_duckdb

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
	if err := runMainDBMigrations(dbType); err != nil {
		return err
	}

	if err := runMigrationsOn(db.TelemetryDB, migrationsDuckDBTelemetryFS, "duckdb_telemetry", "schema_migrations", duckdbTrackingDDL); err != nil {
		return fmt.Errorf("telemetry db migrations: %w", err)
	}

	return nil
}
