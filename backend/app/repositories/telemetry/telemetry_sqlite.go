//go:build !telemetry_ch && !telemetry_duckdb

package telemetry

import (
	"context"

	sqliterepo "github.com/tracewayapp/traceway/backend/app/repositories/telemetry/sqlite"
)

var (
	AiTraceRepository             = sqliterepo.AiTraceRepository
	EndpointRepository            = sqliterepo.EndpointRepository
	ExceptionStackTraceRepository = sqliterepo.ExceptionStackTraceRepository
	FiredNotificationRepository   = sqliterepo.FiredNotificationRepository
	LogRecordRepository           = sqliterepo.LogRecordRepository
	MetricPointRepository         = sqliterepo.MetricPointRepository
	ProfileRepository             = sqliterepo.ProfileRepository
	SessionRecordingRepository    = sqliterepo.SessionRecordingRepository
	SessionRepository             = sqliterepo.SessionRepository
	SpanRepository                = sqliterepo.SpanRepository
	TaskRepository                = sqliterepo.TaskRepository
)

// StartWriters is a no-op: the SQLite backend inserts synchronously.
func StartWriters(ctx context.Context) {}

// FlushWriters is a no-op: SQLite inserts are visible when InsertAsync returns.
func FlushWriters(ctx context.Context) error { return nil }

// StopWriters is a no-op: there are no background writers.
func StopWriters() {}
