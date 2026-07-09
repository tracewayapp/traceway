//go:build transactional_pg

package migrations

import (
	"database/sql"
	"embed"
	"fmt"
	"net/url"

	"github.com/tracewayapp/traceway/backend/app/config"
	"github.com/tracewayapp/traceway/backend/app/db"

	"github.com/golang-migrate/migrate/v4"
	migratePg "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/lib/pq"
)

//go:embed pg/*.sql
var migrationsPgFS embed.FS

func runMigrationsPostgres(connStr string) error {
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return fmt.Errorf("failed to open postgres for migrations: %w", err)
	}
	defer db.Close()

	source, err := iofs.New(migrationsPgFS, "pg")
	if err != nil {
		return fmt.Errorf("failed to create migration source: %w", err)
	}

	driver, err := migratePg.WithInstance(db, &migratePg.Config{})
	if err != nil {
		return fmt.Errorf("failed to create migration driver: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", source, "postgres", driver)
	if err != nil {
		return fmt.Errorf("failed to create migrate instance: %w", err)
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("postgres migration failed: %w", err)
	}

	for _, ext := range ExtensionPostgresMigrations {
		if err := runExtensionMigrations(db, ext); err != nil {
			return fmt.Errorf("extension migration failed: %w", err)
		}
	}

	return nil
}

func runExtensionMigrations(db *sql.DB, ext ExtensionMigration) error {
	source, err := iofs.New(ext.Source, ext.Path)
	if err != nil {
		return fmt.Errorf("failed to create extension migration source: %w", err)
	}

	tableName := ext.Table
	if tableName == "" {
		tableName = "schema_migrations_ext"
	}

	driver, err := migratePg.WithInstance(db, &migratePg.Config{
		MigrationsTable: tableName,
	})
	if err != nil {
		return fmt.Errorf("failed to create extension migration driver: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", source, "postgres", driver)
	if err != nil {
		return fmt.Errorf("failed to create extension migrate instance: %w", err)
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("extension postgres migration failed: %w", err)
	}

	return nil
}

func runMainDBMigrations(dbType string) error {
	if dbType == "sqlite" {
		if err := runMigrationsOn(db.DB, migrationsSqliteFS, "sqlite", "schema_migrations", sqliteTrackingDDL); err != nil {
			return fmt.Errorf("sqlite migrations failed: %w", err)
		}
		return nil
	}

	cfg := config.Config

	pgPort := cfg.PostgresPort
	pgSSLMode := cfg.PostgresSSLMode

	if pgSSLMode == "" {
		pgSSLMode = "disable"
	}
	if pgPort == "" {
		pgPort = "5432"
	}

	pgConnStr := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		url.QueryEscape(cfg.PostgresUsername), url.QueryEscape(cfg.PostgresPassword), cfg.PostgresHost, pgPort, cfg.PostgresDatabase, pgSSLMode)

	if err := runMigrationsPostgres(pgConnStr); err != nil {
		return fmt.Errorf("postgres migrations failed: %w", err)
	}

	return nil
}
