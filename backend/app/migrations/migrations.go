package migrations

import (
	"backend/app/db"
	"database/sql"
	"embed"
	"fmt"
	"os"

	"github.com/golang-migrate/migrate/v4"
	migrateCh "github.com/golang-migrate/migrate/v4/database/clickhouse"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

//go:embed pg/*.sql
var migrationsPgFS embed.FS

//go:embed ch/*.sql
var migrationsChFS embed.FS

func runMigrationsPg(db *sql.DB) error {
	source, err := iofs.New(migrationsPgFS, "pg")
	if err != nil {
		return err
	}

	// Create database driver from existing connection
	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return err
	}

	// Create migrator with both source and database instances
	m, err := migrate.NewWithInstance("iofs", source, "postgres", driver)
	if err != nil {
		return err
	}

	// Run migrations (ignore ErrNoChange)
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return err
	}

	return nil
}

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

	driver, err := migrateCh.WithInstance(db, &migrateCh.Config{})
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

func Run() {
	err := runMigrationsPg(db.DB)

	if err != nil {
		panic(err)
	}

	clickhouseServer := os.Getenv("CLICKHOUSE_SERVER")
	clickhouseDatabase := os.Getenv("CLICKHOUSE_DATABASE")
	clickhouseUsername := os.Getenv("CLICKHOUSE_USERNAME")
	clickhousePassword := os.Getenv("CLICKHOUSE_PASSWORD")

	err = runMigrationsClickhouse(fmt.Sprintf(`clickhouse://%s?username=%s&password=%s&database=%s`, clickhouseServer, clickhouseUsername, clickhousePassword, clickhouseDatabase))

	if err != nil {
		panic(err)
	}
}
