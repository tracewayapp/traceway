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
	{"ai_traces", "recorded_at"},
	{"log_records", "timestamp"},
	{"sessions", "started_at"},
}

func startSQLiteRetention(ctx context.Context, days int) {
	if !db.IsSQLite() {
		return
	}
	if days == 0 {
		config.Logln("SQLite retention disabled (SQLITE_RETENTION_DAYS=0)")
		return
	}

	config.Logf("Starting SQLite retention worker (TTL: %d days, interval: %s)", days, tickInterval)

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
			if err := lit.DeleteNamed(db.Driver, db.TelemetryDB, query, params); err != nil {
				traceway.CaptureException(fmt.Errorf("retention: delete from telemetry.%s failed: %w", tgt.table, err))
			}
		}
	}
}
