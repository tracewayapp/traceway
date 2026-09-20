package otelcontrollers

import (
	"github.com/google/uuid"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
)

func otelOccurrenceID(projectId uuid.UUID, span *tracepb.Span) uuid.UUID {
	var identity [24]byte
	copy(identity[:16], span.TraceId)
	copy(identity[16:], span.SpanId)
	return uuid.NewSHA1(projectId, identity[:])
}

func otelSpanKey(span *tracepb.Span) string {
	return string(span.TraceId) + string(span.SpanId)
}

func validSourceSpanIDs(span *tracepb.Span) bool {
	return span != nil && validOtelID(span.TraceId, 16) && validOtelID(span.SpanId, 8) &&
		(len(span.ParentSpanId) == 0 || len(span.ParentSpanId) == 8)
}
