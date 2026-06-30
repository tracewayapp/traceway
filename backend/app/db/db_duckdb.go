//go:build duckdb && !pgch

package db

import (
	"database/sql"
	"fmt"
	"net/url"
	"strings"

	"github.com/duckdb/duckdb-go/v2"
	"github.com/tracewayapp/lit/v2"
	"github.com/tracewayapp/traceway/backend/app/config"
)

// DuckDBConnector is retained so repositories can open a dedicated driver.Conn
// for the Appender bulk-insert API, which is not reachable through database/sql.
var DuckDBConnector *duckdb.Connector

const duckDBMaxReadConns = 4

func Init() error {
	cfg := config.Config
	if cfg.DBType == "sqlite" {
		return initDuckDB()
	}
	return initPostgres()
}

func initDuckDB() error {
	path := config.Config.SQLitePath
	if path == "" {
		path = "./traceway.db"
	}

	mainDB, err := openSQLite(path, false)
	if err != nil {
		return err
	}
	DB = mainDB
	Driver = lit.SQLite
	config.Logf("SQLite database opened at %s", path)

	telemetryPath := strings.TrimSuffix(path, ".db") + "_telemetry.duckdb"
	if path == ":memory:" {
		telemetryPath = ""
	}
	if err := openDuckDB(telemetryPath); err != nil {
		return err
	}
	config.Logf("DuckDB telemetry database opened at %s", telemetryPath)

	return nil
}

func openDuckDB(path string) error {
	dsn := path
	q := url.Values{}
	if v := strings.TrimSpace(config.Config.DuckDBMemoryLimit); v != "" {
		q.Set("memory_limit", v)
	}
	if v := strings.TrimSpace(config.Config.DuckDBThreads); v != "" {
		q.Set("threads", v)
	}
	if len(q) > 0 {
		dsn = path + "?" + q.Encode()
	}

	connector, err := duckdb.NewConnector(dsn, nil)
	if err != nil {
		return fmt.Errorf("failed to open duckdb at %s: %w", path, err)
	}

	d := sql.OpenDB(connector)
	d.SetMaxOpenConns(duckDBMaxReadConns)
	d.SetMaxIdleConns(duckDBMaxReadConns)
	if err := d.Ping(); err != nil {
		return fmt.Errorf("failed to ping duckdb at %s: %w", path, err)
	}

	DuckDBConnector = connector
	TelemetryDB = d
	return nil
}
