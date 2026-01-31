package models

import (
	"time"

	"github.com/google/uuid"
)

type Span struct {
	Id            uuid.UUID     `json:"id" ch:"id"`
	TraceId       uuid.UUID     `json:"traceId" ch:"trace_id"`
	ProjectId     uuid.UUID     `json:"projectId" ch:"project_id"`
	Name          string        `json:"name" ch:"name"`
	StartTime     time.Time     `json:"startTime" ch:"start_time"`
	Duration      time.Duration `json:"duration" ch:"duration"`
	RecordedAt    time.Time     `json:"recordedAt" ch:"recorded_at"`
}
