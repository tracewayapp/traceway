//go:build telemetry_ch

package repositories

import ch "github.com/tracewayapp/traceway/backend/app/repositories/telemetry/clickhouse"

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
