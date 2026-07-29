//go:build telemetry_ch

package telemetry

import (
	"context"

	ch "github.com/tracewayapp/traceway/backend/app/repositories/telemetry/clickhouse"
)

var (
	AiTraceRepository             = ch.AiTraceRepository
	EndpointRepository            = ch.EndpointRepository
	ExceptionStackTraceRepository = ch.ExceptionStackTraceRepository
	FiredNotificationRepository   = ch.FiredNotificationRepository
	LogRecordRepository           = ch.LogRecordRepository
	MetricPointRepository         = ch.MetricPointRepository
	ProfileRepository             = ch.ProfileRepository
	SessionRecordingRepository    = ch.SessionRecordingRepository
	SessionRepository             = ch.SessionRepository
	SpanRepository                = ch.SpanRepository
	TaskRepository                = ch.TaskRepository
)

// StartWriters is a no-op: ClickHouse batch inserts are already efficient
// per request.
func StartWriters(ctx context.Context) {}

// FlushWriters is a no-op: ClickHouse inserts are visible when InsertAsync
// returns.
func FlushWriters(ctx context.Context) error { return nil }

// StopWriters is a no-op: there are no background writers.
func StopWriters() {}
