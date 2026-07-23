//go:build telemetry_duckdb

package telemetry

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/tracewayapp/traceway/backend/app/config"
	duckdbrepo "github.com/tracewayapp/traceway/backend/app/repositories/telemetry/duckdb"
)

var (
	AiTraceRepository             = duckdbrepo.AiTraceRepository
	EndpointRepository            = duckdbrepo.EndpointRepository
	ExceptionStackTraceRepository = duckdbrepo.ExceptionStackTraceRepository
	FiredNotificationRepository   = duckdbrepo.FiredNotificationRepository
	LogRecordRepository           = duckdbrepo.LogRecordRepository
	MetricPointRepository         = duckdbrepo.MetricPointRepository
	ProfileRepository             = duckdbrepo.ProfileRepository
	SessionRecordingRepository    = duckdbrepo.SessionRecordingRepository
	SessionRepository             = duckdbrepo.SessionRepository
	SpanRepository                = duckdbrepo.SpanRepository
	TaskRepository                = duckdbrepo.TaskRepository
)

// StartWriters starts the DuckDB background write batchers for the hot
// telemetry tables. Invalid or unset env values fall back to the writer
// defaults (zero values in WriterOptions).
func StartWriters(ctx context.Context) {
	opts := duckdbrepo.WriterOptions{
		QueueRows: parsePositiveInt(config.Config.DuckDBWriteQueueRows),
		FlushRows: parsePositiveInt(config.Config.DuckDBWriteFlushRows),
		Writers:   parsePositiveInt(config.Config.DuckDBWriteWriters),
	}
	if ms := parsePositiveInt(config.Config.DuckDBWriteFlushIntervalMS); ms > 0 {
		opts.FlushInterval = time.Duration(ms) * time.Millisecond
	}
	if ms := parsePositiveInt(config.Config.DuckDBWriteQueueWaitMS); ms > 0 {
		opts.QueueWait = time.Duration(ms) * time.Millisecond
	}
	duckdbrepo.StartWriters(ctx, opts)
}

// FlushWriters blocks until everything enqueued before the call is flushed.
func FlushWriters(ctx context.Context) error {
	return duckdbrepo.FlushWriters(ctx)
}

func parsePositiveInt(v string) int {
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil || n <= 0 {
		return 0
	}
	return n
}
