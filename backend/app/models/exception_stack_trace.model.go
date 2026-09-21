package models

import (
	"time"

	"github.com/google/uuid"
)

type ExceptionStackTrace struct {
	Id        uuid.UUID `json:"id" ch:"id"`
	ProjectId uuid.UUID `json:"projectId" ch:"project_id"`
	// TraceId is the trace the exception happened in and SpanId the span it happened on, as lowercase hex.
	TraceId string `json:"traceId" ch:"trace_id"`
	SpanId  string `json:"spanId" ch:"span_id"`
	// TraceType is set when it is known without a lookup: native clients say it, and an exception on a promoted span has that span's kind.
	TraceType     string            `json:"traceType" ch:"trace_type"`
	ExceptionHash string            `json:"exceptionHash" ch:"exception_hash"`
	StackTrace    string            `json:"stackTrace" ch:"stack_trace"`
	RecordedAt    time.Time         `json:"recordedAt" ch:"recorded_at"`
	Attributes    map[string]string `json:"attributes" ch:"attributes"`
	AppVersion    string            `json:"appVersion" ch:"app_version"`
	ServerName    string            `json:"serverName" ch:"server_name"`
	IsMessage     bool              `json:"isMessage" ch:"is_message"`
	SessionId     *uuid.UUID        `json:"sessionId,omitempty" ch:"session_id"`
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
