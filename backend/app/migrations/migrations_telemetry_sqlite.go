//go:build !telemetry_ch && !telemetry_duckdb

package migrations

import (
	"embed"
	"fmt"

	"github.com/tracewayapp/traceway/backend/app/db"
)

//go:embed sqlite_telemetry/*.sql
var migrationsSqliteTelemetryFS embed.FS

func Run(dbType string) error {
	if err := runMainDBMigrations(dbType); err != nil {
		return err
	}

	if err := runMigrationsOn(db.TelemetryDB, migrationsSqliteTelemetryFS, "sqlite_telemetry", "schema_migrations", sqliteTrackingDDL); err != nil {
		return fmt.Errorf("telemetry db migrations: %w", err)
	}

	return nil
}
