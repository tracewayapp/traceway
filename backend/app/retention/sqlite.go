package retention

import (
	"context"
	"fmt"
	"time"

	"github.com/tracewayapp/lit/v2"
	"github.com/tracewayapp/traceway/backend/app/config"
	"github.com/tracewayapp/traceway/backend/app/db"
	traceway "go.tracewayapp.com"
)

var telemetryRetentionTargets = []struct {
	table  string
	column string
}{
	{"endpoints", "recorded_at"},
	{"tasks", "recorded_at"},
	{"exception_stack_traces", "recorded_at"},
	{"spans", "recorded_at"},
	{"metric_points", "recorded_at"},
	{"session_recordings", "recorded_at"},
	{"fired_notifications", "fired_at"},
	{"check_results", "recorded_at"},
	{"ai_traces", "recorded_at"},
	{"log_records", "timestamp"},
	{"sessions", "started_at"},
	{"profiling_samples", "start_time"},
	{"profiles", "recorded_at"},
	{"profiling_stacks", "last_seen"},
}

func startSQLiteRetention(ctx context.Context, days int, source string) {
	// Prune whenever an embedded/engine telemetry store exists (SQLite,
	// DuckDB, Firebolt) — none of them have built-in TTLs. ClickHouse
	// builds have no TelemetryDB and expire via table TTLs instead.
	if db.TelemetryDB == nil {
		return
	}
	if days == 0 {
		config.Logf("Telemetry retention disabled (%s=0)", source)
		return
	}

	config.Logf("Starting telemetry retention worker (TTL: %d days, interval: %s, via %s)", days, tickInterval, source)

	go func() {
		defer traceway.Recover()

		runSQLiteRetention(ctx, days)

		ticker := time.NewTicker(tickInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				runSQLiteRetention(ctx, days)
			}
		}
	}()
}

func runSQLiteRetention(ctx context.Context, days int) {
	cutoff := time.Now().UTC().AddDate(0, 0, -days).Format(time.RFC3339Nano)
	params := lit.P{"cutoff": cutoff}

	if db.TelemetryDB != nil {
		for _, tgt := range telemetryRetentionTargets {
			if ctx.Err() != nil {
				return
			}
			query := fmt.Sprintf("DELETE FROM %s WHERE %s < :cutoff", tgt.table, tgt.column)
			if err := lit.DeleteNamed(db.TelemetryDriver, db.TelemetryDB, query, params); err != nil {
				traceway.CaptureException(fmt.Errorf("retention: delete from telemetry.%s failed: %w", tgt.table, err))
			}
		}
		reclaimTelemetryDisk(ctx)
	}
}

func reclaimTelemetryDisk(ctx context.Context) {
	if db.IsDuckDBTelemetry() {
		if _, err := db.TelemetryDB.ExecContext(ctx, "CHECKPOINT"); err != nil {
			traceway.CaptureException(fmt.Errorf("retention: telemetry maintenance %q failed: %w", "CHECKPOINT", err))
		}
		return
	}
	for _, stmt := range []string{
		"PRAGMA incremental_vacuum",
		"PRAGMA optimize",
		"PRAGMA wal_checkpoint(TRUNCATE)",
	} {
		if ctx.Err() != nil {
			return
		}
		if _, err := db.TelemetryDB.ExecContext(ctx, stmt); err != nil {
			traceway.CaptureException(fmt.Errorf("retention: telemetry maintenance %q failed: %w", stmt, err))
		}
	}
}
