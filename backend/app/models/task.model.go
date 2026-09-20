package models

import (
	"time"

	"github.com/google/uuid"
)

type Task struct {
	Id         uuid.UUID         `json:"id" ch:"id"`
	ProjectId  uuid.UUID         `json:"projectId" ch:"project_id"`
	TaskName   string            `json:"taskName" ch:"task_name"`
	Duration   time.Duration     `json:"duration" ch:"duration"`
	RecordedAt time.Time         `json:"recordedAt" ch:"recorded_at"`
	ClientIP   string            `json:"clientIP" ch:"client_ip"`
	Attributes map[string]string `json:"attributes" ch:"attributes"`
	AppVersion string            `json:"appVersion" ch:"app_version"`
	ServerName string            `json:"serverName" ch:"server_name"`
	// Ids as they arrived, lowercase hex. LinkedTraceId is another trace this row belongs with, such as the browser's.
	TraceId       string `json:"traceId" ch:"trace_id"`
	SpanId        string `json:"spanId" ch:"span_id"`
	ParentSpanId  string `json:"parentSpanId,omitempty" ch:"parent_span_id"`
	LinkedTraceId string `json:"linkedTraceId,omitempty" ch:"linked_trace_id"`
	IsRoot        bool   `json:"isRoot" ch:"is_root"`
}

type TaskStats struct {
	TaskName    string        `json:"taskName"`
	Count       uint64        `json:"count"`
	P50Duration time.Duration `json:"p50Duration"`
	P95Duration time.Duration `json:"p95Duration"`
	AvgDuration time.Duration `json:"avgDuration"`
	LastSeen    time.Time     `json:"lastSeen"`
	HasRoot     bool          `json:"hasRoot"`
	HasNonRoot  bool          `json:"hasNonRoot"`
}

// TaskDetailStats contains detailed statistics for a specific task
type TaskDetailStats struct {
	Count          int64   `json:"count"`
	AvgDuration    float64 `json:"avgDuration"`    // in ms
	MedianDuration float64 `json:"medianDuration"` // in ms
	P95Duration    float64 `json:"p95Duration"`    // in ms
	P99Duration    float64 `json:"p99Duration"`    // in ms
	Throughput     float64 `json:"throughput"`     // tasks per minute
}
