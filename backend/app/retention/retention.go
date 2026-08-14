package retention

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/tracewayapp/traceway/backend/app/config"
	"github.com/tracewayapp/traceway/backend/app/db"
)

const (
	defaultRetentionDays = 30
	tickInterval         = time.Hour
)

func Start(ctx context.Context) {
	cfg := config.Config

	telemetryDays, telemetrySource := telemetryRetentionConfig(db.IsDuckDBTelemetry(), cfg.SQLiteRetentionDays, cfg.DuckDBRetentionDays)
	startSQLiteRetention(ctx, telemetryDays, telemetrySource)
	startLogRecordsCap(ctx, parseMaxRows(cfg.LogRecordsMaxRows))
	startRecordingDiskCleanup(ctx, parseRetentionDays(cfg.SessionRecordingRetentionDays))
	startProfileArchiveDiskCleanup(ctx, parseRetentionDays(cfg.ProfileRetentionDays))
	startSyntheticsScreenshotCleanup(ctx, parseRetentionDays(cfg.SyntheticsScreenshotRetentionDays))
	startOAuthSessionsPrune(ctx)
	startAuthTokensPrune(ctx)
	startOutboxPrune(ctx)
}

func parseRetentionDays(value string) int {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return defaultRetentionDays
	}
	days, err := strconv.Atoi(trimmed)
	if err != nil || days < 0 {
		return defaultRetentionDays
	}
	return days
}

// telemetryRetentionConfig picks the TTL for the embedded telemetry store. On
// the DuckDB backend DUCKDB_RETENTION_DAYS wins when set, falling back to
// SQLITE_RETENTION_DAYS so deployments that configured the old variable keep
// their window. Both default to 30 days when unset or invalid.
func telemetryRetentionConfig(isDuckDB bool, sqliteDays, duckdbDays string) (int, string) {
	if isDuckDB && strings.TrimSpace(duckdbDays) != "" {
		return parseRetentionDays(duckdbDays), "DUCKDB_RETENTION_DAYS"
	}
	return parseRetentionDays(sqliteDays), "SQLITE_RETENTION_DAYS"
}

// parseMaxRows returns 0 (disabled) for empty, invalid, or non-positive values.
func parseMaxRows(value string) int {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return 0
	}
	rows, err := strconv.Atoi(trimmed)
	if err != nil || rows < 0 {
		return 0
	}
	return rows
}
