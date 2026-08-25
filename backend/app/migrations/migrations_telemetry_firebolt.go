//go:build telemetry_firebolt

package migrations

import (
	"embed"
	"fmt"

	"github.com/tracewayapp/traceway/backend/app/db"
)

//go:embed firebolt_telemetry/*.sql
var migrationsFireboltTelemetryFS embed.FS

const fireboltTrackingDDL = `CREATE TABLE IF NOT EXISTS %s (
	version TEXT NOT NULL,
	applied_at TIMESTAMP NOT NULL DEFAULT NOW()
)`

// RunTelemetryMigrationsForTest applies only the Firebolt telemetry
// migrations — the telemetry test helper uses it without a main DB.
func RunTelemetryMigrationsForTest() error {
	return runMigrationsOn(db.TelemetryDB, migrationsFireboltTelemetryFS, "firebolt_telemetry", "schema_migrations", fireboltTrackingDDL)
}

func Run(dbType string) error {
	if err := runMainDBMigrations(dbType); err != nil {
		return err
	}

	if err := runMigrationsOn(db.TelemetryDB, migrationsFireboltTelemetryFS, "firebolt_telemetry", "schema_migrations", fireboltTrackingDDL); err != nil {
		return fmt.Errorf("telemetry db migrations: %w", err)
	}

	return nil
}
