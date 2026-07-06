//go:build !oltp_pg

package migrations

import (
	"fmt"

	"github.com/tracewayapp/traceway/backend/app/db"
)

func runMainDBMigrations(_ string) error {
	if err := runMigrationsOn(db.DB, migrationsSqliteFS, "sqlite", "schema_migrations", sqliteTrackingDDL); err != nil {
		return fmt.Errorf("main db migrations: %w", err)
	}
	return nil
}
