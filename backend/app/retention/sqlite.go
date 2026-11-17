package retention

import (
	"context"
	"fmt"
	"time"

	"github.com/tracewayapp/lit/v2"
	"github.com/tracewayapp/traceway/backend/v2/app/config"
	"github.com/tracewayapp/traceway/backend/v2/app/db"
	"github.com/tracewayapp/traceway/backend/v2/app/repositories/telemetry"
	"github.com/tracewayapp/traceway/backend/v2/app/repositories/telemetry/shared"
	"github.com/tracewayapp/traceway/backend/v2/app/traceway"
)

var telemetryRetentionTargets = []struct {
	table  string
	column string
}{
	{"endpoints_v2", "recorded_at"},
	{"tasks_v2", "recorded_at"},
	{"exceptions_v2", "recorded_at"},
	{"spans_v2", "recorded_at"},
	{"ai_traces_v2", "recorded_at"},
	// The tables V2 replaced. They hold history until the move-over script has run and they are dropped.
	{"endpoints", "recorded_at"},
	{"tasks", "recorded_at"},
	{"exception_stack_traces", "recorded_at"},
	{"spans", "recorded_at"},
	{"metric_points", "recorded_at"},
	{"metric_histograms", "recorded_at"},
	{"metric_exemplars", "recorded_at"},
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

// TelemetryTimeColumns maps each pruned telemetry table to its retention time
// column, which is also the column the migrations package's index guard
// expects project-leading indexes to reach.
func TelemetryTimeColumns() map[string]string {
	columns := make(map[string]string, len(telemetryRetentionTargets))
	for _, tgt := range telemetryRetentionTargets {
		columns[tgt.table] = tgt.column
	}
	return columns
}

func startSQLiteRetention(ctx context.Context, days int, source string) {
	if !db.IsSQLite() {
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
	traceway.RunTask(ctx, "retention.telemetry", func(ctx context.Context) {
		pruneTelemetry(ctx, days)
	})
}

func pruneTelemetry(ctx context.Context, days int) {
	cutoff := time.Now().UTC().AddDate(0, 0, -days).Format(time.RFC3339Nano)
	params := lit.P{"cutoff": cutoff}

	if db.TelemetryDB != nil {
		for _, tgt := range telemetryRetentionTargets {
			if ctx.Err() != nil {
				return
			}
			query := fmt.Sprintf("DELETE FROM %s WHERE %s < :cutoff", tgt.table, tgt.column)
			if err := lit.DeleteNamed(db.Driver, shared.Traced(ctx, db.TelemetryDB), query, params); err != nil {
				traceway.CaptureExceptionContext(ctx, fmt.Errorf("retention: delete from telemetry.%s failed: %w", tgt.table, err))
			}
		}
		if _, err := db.TelemetryDB.ExecContext(ctx, "DELETE FROM session_recording_reasons WHERE fired_at < ?", time.Now().UTC().AddDate(0, 0, -days).UnixMilli()); err != nil {
			traceway.CaptureExceptionContext(ctx, fmt.Errorf("retention: recording reasons: %w", err))
		}
		compactHistograms(ctx)
		reclaimTelemetryDisk(ctx)
	}
}

func reclaimTelemetryDisk(ctx context.Context) {
	if db.IsDuckDBTelemetry() {
		if _, err := db.TelemetryDB.ExecContext(ctx, "CHECKPOINT"); err != nil {
			traceway.CaptureExceptionContext(ctx, fmt.Errorf("retention: telemetry maintenance %q failed: %w", "CHECKPOINT", err))
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
			traceway.CaptureExceptionContext(ctx, fmt.Errorf("retention: telemetry maintenance %q failed: %w", stmt, err))
		}
	}
}

// histogramMinuteWindow is how long histogram rows stay at minute grain before the
// hourly pass folds them into one row per cell per hour.
const histogramMinuteWindow = 6 * time.Hour

func compactHistograms(ctx context.Context) {
	if _, err := telemetry.MetricHistogramRepository.Compact(ctx, time.Now().UTC().Add(-histogramMinuteWindow)); err != nil {
		traceway.CaptureExceptionContext(ctx, fmt.Errorf("retention: metric_histograms compaction failed: %w", err))
	}
}
