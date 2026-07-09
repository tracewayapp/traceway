//go:build telemetry_duckdb

package telemetry

import duckdbrepo "github.com/tracewayapp/traceway/backend/app/repositories/telemetry/duckdb"

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
