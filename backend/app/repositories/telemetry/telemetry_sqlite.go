//go:build !telemetry_ch && !telemetry_duckdb && !telemetry_firebolt

package telemetry

import sqliterepo "github.com/tracewayapp/traceway/backend/app/repositories/telemetry/sqlite"

var (
	AiTraceRepository             = sqliterepo.AiTraceRepository
	CheckResultRepository         = sqliterepo.CheckResultRepository
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
