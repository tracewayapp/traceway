//go:build telemetry_ch

package migrations

import (
	"database/sql"
	"embed"
	"fmt"
	"net/url"

	"github.com/tracewayapp/traceway/backend/app/config"

	"github.com/golang-migrate/migrate/v4"
	migrateCh "github.com/golang-migrate/migrate/v4/database/clickhouse"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

//go:embed ch/*.sql
var migrationsChFS embed.FS

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

	m.Log = migrateLogger{store: "clickhouse"}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return err
	}

	return nil
}

func Run(dbType string) error {
	cfg := config.Config

	tlsConfig := "&secure=true"
	if cfg.ClickhouseTLS == "false" {
		tlsConfig = ""
	}

	err := runMigrationsClickhouse(fmt.Sprintf(`clickhouse://%s?username=%s&password=%s&database=%s%s`, cfg.ClickhouseServer, url.QueryEscape(cfg.ClickhouseUsername), url.QueryEscape(cfg.ClickhousePassword), cfg.ClickhouseDatabase, tlsConfig))
	if err != nil {
		return fmt.Errorf("clickhouse migrations failed: %w", err)
	}

	return runMainDBMigrations(dbType)
}
