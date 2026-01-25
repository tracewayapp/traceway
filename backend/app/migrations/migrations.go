package migrations

import (
	"database/sql"
	"embed"
	"fmt"
	"net/url"
	"os"

	"github.com/golang-migrate/migrate/v4"
	migrateCh "github.com/golang-migrate/migrate/v4/database/clickhouse"
	migratePg "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/lib/pq"
)

type ExtensionMigration struct {
	Source embed.FS
	Path   string
	Table  string // unique migration table name per extension
}

var ExtensionPostgresMigrations []ExtensionMigration

//go:embed ch/*.sql
var migrationsChFS embed.FS

//go:embed pg/*.sql
var migrationsPgFS embed.FS

func runMigrationsClickhouse(connStr string) error {
	db, err := sql.Open("clickhouse", connStr)
	if err != nil {
		return err
	}
	defer db.Close()

	source, err := iofs.New(migrationsChFS, "ch")
	if err != nil {
		return err
	}

	driver, err := migrateCh.WithInstance(db, &migrateCh.Config{
		MigrationsTableEngine: "MergeTree",
	})
	if err != nil {
		return err
	}

	m, err := migrate.NewWithInstance("iofs", source, "clickhouse", driver)
	if err != nil {
		return err
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return err
	}

	return nil
}

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

func Run() error {
	// Run ClickHouse migrations
	clickhouseServer := os.Getenv("CLICKHOUSE_SERVER")
	clickhouseDatabase := os.Getenv("CLICKHOUSE_DATABASE")
	clickhouseUsername := os.Getenv("CLICKHOUSE_USERNAME")
	clickhousePassword := os.Getenv("CLICKHOUSE_PASSWORD")
	clickhouseTls := os.Getenv("CLICKHOUSE_TLS")

	tlsConfig := "&secure=true"
	if clickhouseTls == "false" {
		tlsConfig = ""
	}

	err := runMigrationsClickhouse(fmt.Sprintf(`clickhouse://%s?username=%s&password=%s&database=%s%s`, clickhouseServer, url.QueryEscape(clickhouseUsername), url.QueryEscape(clickhousePassword), clickhouseDatabase, tlsConfig))
	if err != nil {
		return fmt.Errorf("clickhouse migrations failed: %w", err)
	}

	// Run PostgreSQL migrations
	pgHost := os.Getenv("POSTGRES_HOST")
	pgPort := os.Getenv("POSTGRES_PORT")
	pgDatabase := os.Getenv("POSTGRES_DATABASE")
	pgUsername := os.Getenv("POSTGRES_USERNAME")
	pgPassword := os.Getenv("POSTGRES_PASSWORD")
	pgSSLMode := os.Getenv("POSTGRES_SSLMODE")

	if pgSSLMode == "" {
		pgSSLMode = "disable"
	}
	if pgPort == "" {
		pgPort = "5432"
	}

	pgConnStr := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		url.QueryEscape(pgUsername), url.QueryEscape(pgPassword), pgHost, pgPort, pgDatabase, pgSSLMode)

	if err := runMigrationsPostgres(pgConnStr); err != nil {
		return fmt.Errorf("postgres migrations failed: %w", err)
	}

	return nil
}
