//go:build telemetry_firebolt

package telemetry

import fireboltrepo "github.com/tracewayapp/traceway/backend/app/repositories/telemetry/firebolt"

var (
	AiTraceRepository             = fireboltrepo.AiTraceRepository
	CheckResultRepository         = fireboltrepo.CheckResultRepository
	EndpointRepository            = fireboltrepo.EndpointRepository
	ExceptionStackTraceRepository = fireboltrepo.ExceptionStackTraceRepository
	FiredNotificationRepository   = fireboltrepo.FiredNotificationRepository
	LogRecordRepository           = fireboltrepo.LogRecordRepository
	MetricPointRepository         = fireboltrepo.MetricPointRepository
	ProfileRepository             = fireboltrepo.ProfileRepository
	SessionRecordingRepository    = fireboltrepo.SessionRecordingRepository
	SessionRepository             = fireboltrepo.SessionRepository
	SpanRepository                = fireboltrepo.SpanRepository
	TaskRepository                = fireboltrepo.TaskRepository
)
