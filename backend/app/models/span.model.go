package models

import (
	"time"

	"github.com/google/uuid"
)

// Span is one stored span. TraceId, SpanId and ParentSpanId are the ids it arrived with, as lowercase hex: 32
// characters for a trace, 16 for an OTel span, 32 for a span from the native protocol.
type Span struct {
	ProjectId    uuid.UUID `json:"projectId"`
	TraceId      string    `json:"traceId"`
	SpanId       string    `json:"spanId"`
	ParentSpanId string    `json:"parentSpanId,omitempty"`

	Name        string        `json:"name"`
	StartTime   time.Time     `json:"startTime"`
	Duration    time.Duration `json:"duration"`
	RecordedAt  time.Time     `json:"recordedAt"`
	SpanKind    int32         `json:"spanKind,omitempty"`
	StatusCode  int32         `json:"statusCode,omitempty"`
	ServiceName string        `json:"serviceName,omitempty"`
	ScopeName   string        `json:"scopeName,omitempty"`

	Attributes        map[string]string `json:"attributes,omitempty"`
	AttributesOmitted bool              `json:"attributesOmitted,omitempty"`
	// DbStatement is a short preview of the database statement, sent when a read leaves the attributes out.
	DbStatement string `json:"dbStatement,omitempty"`
}
