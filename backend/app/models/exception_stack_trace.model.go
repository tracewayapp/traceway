package models

import (
	"time"

	"github.com/google/uuid"
)

type ExceptionStackTrace struct {
	Id              uuid.UUID         `json:"id" ch:"id"`
	ProjectId       uuid.UUID         `json:"projectId" ch:"project_id"`
	TraceId   *uuid.UUID        `json:"traceId" ch:"trace_id"`
	TraceType string            `json:"traceType" ch:"trace_type"` // "endpoint" or "task"
	ExceptionHash   string            `json:"exceptionHash" ch:"exception_hash"`
	StackTrace      string            `json:"stackTrace" ch:"stack_trace"`
	RecordedAt      time.Time         `json:"recordedAt" ch:"recorded_at"`
	Attributes      map[string]string `json:"attributes" ch:"attributes"`
	AppVersion      string            `json:"appVersion" ch:"app_version"`
	ServerName      string            `json:"serverName" ch:"server_name"`
	IsMessage       bool              `json:"isMessage" ch:"is_message"`
}

type ExceptionTrendPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Count     uint64    `json:"count"`
}

type ExceptionGroup struct {
	ExceptionHash string                `json:"exceptionHash" ch:"exception_hash"`
	StackTrace    string                `json:"stackTrace" ch:"stack_trace"`
	LastSeen      time.Time             `json:"lastSeen" ch:"last_seen"`
	FirstSeen     time.Time             `json:"firstSeen" ch:"first_seen"`
	Count         uint64                `json:"count" ch:"count"`
	HourlyTrend   []ExceptionTrendPoint `json:"hourlyTrend,omitempty"`
}
