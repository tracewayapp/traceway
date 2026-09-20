package otelcontrollers

import (
	"encoding/hex"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry/shared"
	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
)

func otelOccurrenceID(projectId uuid.UUID, span *tracepb.Span) uuid.UUID {
	traceID := otelTraceIDToUUID(span.TraceId)
	spanID := otelSpanIDToUUID(span.SpanId)
	return uuid.NewSHA1(projectId, append(traceID[:], spanID[8:]...))
}

func otelSpanKey(span *tracepb.Span) string {
	return string(span.TraceId) + string(span.SpanId)
}

func convertCanonicalSpans(projectId uuid.UUID, req *coltracepb.ExportTraceServiceRequest) []models.OtelSpan {
	result := make([]models.OtelSpan, 0)
	for _, rs := range req.ResourceSpans {
		for _, ss := range rs.ScopeSpans {
			metadata := &tracepb.ResourceSpans{Resource: rs.Resource, SchemaUrl: rs.SchemaUrl,
				ScopeSpans: []*tracepb.ScopeSpans{{Scope: ss.Scope, SchemaUrl: ss.SchemaUrl}}}
			metadata.ProtoReflect().SetUnknown(rs.ProtoReflect().GetUnknown())
			metadata.ScopeSpans[0].ProtoReflect().SetUnknown(ss.ProtoReflect().GetUnknown())
			for _, source := range ss.Spans {
				if !validSourceSpanIDs(source) {
					continue
				}
				attributes := extractAttributes(source.Attributes)
				delete(attributes, "exception.stacktrace")
				start := nanoToTime(source.StartTimeUnixNano)
				result = append(result, models.OtelSpan{
					OTLP: source, Context: metadata,
					Span: models.Span{
						ProjectId: projectId,
						TraceId:   hex.EncodeToString(source.TraceId), SpanId: hex.EncodeToString(source.SpanId),
						ParentSpanId: hex.EncodeToString(source.ParentSpanId),
						Name:         source.Name, StartTime: start, RecordedAt: start, Duration: shared.OtelDuration(source.StartTimeUnixNano, source.EndTimeUnixNano),
						Attributes: attributes, SpanKind: int32(source.Kind), StatusCode: int32(source.GetStatus().GetCode()),
						ServiceName: getStringAttribute(rs.GetResource().GetAttributes(), "service.name"), ScopeName: ss.GetScope().GetName(),
					},
				})
			}
		}
	}
	return result
}

func countInvalidSpanIDs(req *coltracepb.ExportTraceServiceRequest) int64 {
	var count int64
	for _, resource := range req.ResourceSpans {
		for _, scope := range resource.ScopeSpans {
			for _, span := range scope.Spans {
				if !validSourceSpanIDs(span) {
					count++
				}
			}
		}
	}
	return count
}

func validSourceSpanIDs(span *tracepb.Span) bool {
	return span != nil && len(span.TraceId) == 16 && len(span.SpanId) == 8 &&
		(len(span.ParentSpanId) == 0 || len(span.ParentSpanId) == 8) &&
		otelTraceIDToUUID(span.TraceId) != uuid.Nil && otelSpanIDToUUID(span.SpanId) != uuid.Nil
}
