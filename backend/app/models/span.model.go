package models

import (
	"time"

	"github.com/google/uuid"
)

type Span struct {
	Id           uuid.UUID         `json:"id" ch:"id"`
	TraceId      uuid.UUID         `json:"traceId" ch:"trace_id"`
	ProjectId    uuid.UUID         `json:"projectId" ch:"project_id"`
	Name         string            `json:"name" ch:"name"`
	StartTime    time.Time         `json:"startTime" ch:"start_time"`
	Duration     time.Duration     `json:"duration" ch:"duration"`
	RecordedAt   time.Time         `json:"recordedAt" ch:"recorded_at"`
	ParentSpanId *uuid.UUID        `json:"parentSpanId,omitempty" ch:"parent_span_id"`
	Attributes   map[string]string `json:"attributes,omitempty" ch:"attributes"`
}

// TraceRef names one entity's span tree. Spans are stored under their owning
// endpoint, task or AI-trace id as trace_id, scoped to the project, and
// RecordedAt anchors the lookup window.
type TraceRef struct {
	ProjectId  uuid.UUID
	TraceId    uuid.UUID
	RecordedAt time.Time
}
