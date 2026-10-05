//go:build !telemetry_ch && !telemetry_duckdb

package db

import (
	"strings"

	"github.com/tracewayapp/traceway/backend/app/config"
)

func initTelemetryDB() error {
	if config.Config.DBType != "sqlite" {
		return nil
	}

	path := config.Config.SQLitePath
	if path == "" {
		path = "./traceway.db"
	}

	telemetryPath := strings.TrimSuffix(path, ".db") + "_telemetry.db"
	if path == ":memory:" {
		telemetryPath = ":memory:"
	}
	telDB, err := openSQLite(telemetryPath, true)
	if err != nil {
		return err
	}
	TelemetryDB = telDB
	config.Logf("SQLite telemetry database opened at %s (page cache: %d MB per connection, up to %d connections)", telemetryPath, telemetryCacheMB(), telemetryMaxOpenConns)

	return nil
}
