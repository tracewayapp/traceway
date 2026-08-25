//go:build telemetry_firebolt

package db

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strings"

	// Registers the "firebolt" database/sql driver.
	_ "github.com/firebolt-db/firebolt-go-sdk"
	"github.com/tracewayapp/traceway/backend/app/config"
)

// Firebolt replaces ClickHouse as the telemetry store in this build;
// TelemetryDB speaks SQL-over-HTTP to a self-managed engine.
const fireboltMaxConns = 8

func initTelemetryDB() error {
	engineURL := strings.TrimSpace(config.Config.FireboltURL)
	if engineURL == "" {
		engineURL = "http://localhost:3473"
	}

	database := strings.TrimSpace(config.Config.FireboltDatabase)
	if database != "" {
		if err := fireboltEnsureDatabase(engineURL, database); err != nil {
			return err
		}
	}

	d, err := sql.Open("firebolt", fireboltDSN(engineURL, database))
	if err != nil {
		return fmt.Errorf("failed to open firebolt at %s: %w", engineURL, err)
	}
	d.SetMaxOpenConns(fireboltMaxConns)
	d.SetMaxIdleConns(fireboltMaxConns)
	if err := d.Ping(); err != nil {
		return fmt.Errorf("failed to ping firebolt at %s: %w", engineURL, err)
	}

	TelemetryDB = d
	telemetryIsFirebolt = true
	telemetryEngineStatsFn = fireboltEngineStats
	config.Logf("Firebolt telemetry database opened at %s", engineURL)
	return nil
}

// The SDK's DSN parser accepts `firebolt://?params` or
// `firebolt:///<db>?params`; a bare third slash fails to parse.
func fireboltDSN(engineURL, database string) string {
	dbPart := ""
	if database != "" {
		dbPart = "/" + url.PathEscape(database)
	}
	return fmt.Sprintf("firebolt://%s?url=%s&client_side_lb=false", dbPart, url.QueryEscape(engineURL))
}

// fireboltEnsureDatabase creates the configured database if missing, using a
// database-less connection — the DSN-scoped connection can't open otherwise.
func fireboltEnsureDatabase(engineURL, database string) error {
	d, err := sql.Open("firebolt", fireboltDSN(engineURL, ""))
	if err != nil {
		return fmt.Errorf("failed to open firebolt at %s: %w", engineURL, err)
	}
	defer d.Close()
	if _, err := d.Exec("CREATE DATABASE IF NOT EXISTS " + database); err != nil {
		return fmt.Errorf("failed to create firebolt database %s: %w", database, err)
	}
	return nil
}

func fireboltEngineStats(ctx context.Context) TelemetryEngineStats {
	var s TelemetryEngineStats
	st := TelemetryDB.Stats()
	s.ReadPoolInUse = st.InUse
	s.ReadPoolWaitCount = st.WaitCount
	s.ReadPoolWaitMs = st.WaitDuration.Milliseconds()
	return s
}
