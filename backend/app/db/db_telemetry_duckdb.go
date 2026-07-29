//go:build telemetry_duckdb

package db

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/duckdb/duckdb-go/v2"
	"github.com/tracewayapp/traceway/backend/app/config"
)

// DuckDBConnector is retained so repositories can open a dedicated driver.Conn
// for the Appender bulk-insert API, which is not reachable through database/sql.
var DuckDBConnector *duckdb.Connector

var duckDBPath string

const duckDBMaxReadConns = 4

func initTelemetryDB() error {
	if config.Config.DBType != "sqlite" {
		return nil
	}

	path := config.Config.SQLitePath
	if path == "" {
		path = "./traceway.db"
	}

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
	q := url.Values{}
	// Safe to drop: every telemetry read query has an explicit ORDER BY.
	q.Set("preserve_insertion_order", "false")
	if v := strings.TrimSpace(config.Config.DuckDBMemoryLimit); v != "" {
		q.Set("memory_limit", v)
	}
	if v := strings.TrimSpace(config.Config.DuckDBThreads); v != "" {
		q.Set("threads", v)
	}
	// DuckDB's own default (16MB) stalls sustained Appender ingest with
	// frequent WAL checkpoints; the benchmark campaign ran 256MB, so that is
	// the shipped default. Costs a larger WAL and longer restart replay.
	checkpointThreshold := strings.TrimSpace(config.Config.DuckDBCheckpointThreshold)
	if checkpointThreshold == "" {
		checkpointThreshold = "256MB"
	}
	q.Set("checkpoint_threshold", checkpointThreshold)
	dsn := path + "?" + q.Encode()

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
	telemetryIsDuckDB = true
	duckDBPath = path
	telemetryEngineStatsFn = duckDBEngineStats
	return nil
}

func duckDBEngineStats(ctx context.Context) TelemetryEngineStats {
	var s TelemetryEngineStats
	if fi, err := os.Stat(duckDBPath); err == nil {
		s.DBSizeBytes = fi.Size()
	}
	if fi, err := os.Stat(duckDBPath + ".wal"); err == nil {
		s.WALSizeBytes = fi.Size()
	}
	var mem int64
	if err := TelemetryDB.QueryRowContext(ctx, "SELECT COALESCE(CAST(SUM(memory_usage_bytes) AS BIGINT), 0) FROM duckdb_memory()").Scan(&mem); err == nil {
		s.MemoryUsedBytes = mem
	}
	st := TelemetryDB.Stats()
	s.ReadPoolInUse = st.InUse
	s.ReadPoolWaitCount = st.WaitCount
	s.ReadPoolWaitMs = st.WaitDuration.Milliseconds()
	return s
}
