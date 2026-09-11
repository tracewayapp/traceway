package otelcontrollers

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"testing"

	"github.com/google/uuid"
	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
)

func TestPromotedEntityLogCorrelation(t *testing.T) {
	const traceHex = "8462dd06157032df3c6c90ac5ff74ed7"
	traceBytes, _ := hex.DecodeString(traceHex)
	jsonTraceBytes, _ := base64.StdEncoding.DecodeString(traceHex)
	spanBytes, _ := hex.DecodeString("1985a7abed0024db")
	groupID := uuid.MustParse("a5100000-0000-4000-8000-000000000001")

	for _, entity := range []struct {
		name  string
		kind  tracepb.Span_SpanKind
		attrs []*commonpb.KeyValue
	}{
		{"task", tracepb.Span_SPAN_KIND_CONSUMER, nil},
		{"endpoint", tracepb.Span_SPAN_KIND_SERVER, []*commonpb.KeyValue{strKV("http.request.method", "GET")}},
		{"ai", tracepb.Span_SPAN_KIND_CLIENT, []*commonpb.KeyValue{strKV("gen_ai.system", "openai")}},
	} {
		for _, encoding := range []struct {
			name string
			id   []byte
		}{
			{"protobuf", traceBytes},
			{"json", jsonTraceBytes},
		} {
			for _, override := range []bool{false, true} {
				t.Run(entity.name+"/"+encoding.name+"/override="+map[bool]string{false: "false", true: "true"}[override], func(t *testing.T) {
					attrs := append([]*commonpb.KeyValue{}, entity.attrs...)
					if override {
						attrs = append(attrs, strKV("traceway.distributed_trace_id", groupID.String()))
					}
					req := &coltracepb.ExportTraceServiceRequest{ResourceSpans: []*tracepb.ResourceSpans{{
						ScopeSpans: []*tracepb.ScopeSpans{{Spans: []*tracepb.Span{{
							TraceId: encoding.id, SpanId: spanBytes, ParentSpanId: []byte{1, 2, 3, 4, 5, 6, 7, 8},
							Name: "worker", Kind: entity.kind, Attributes: attrs,
							StartTimeUnixNano: 1_700_000_000_000_000_000, EndTimeUnixNano: 1_700_000_000_001_000_000,
						}}}},
					}}}
					endpoints, tasks, _, _, aiTraces, _ := convertTraces(context.Background(), nil, testProjectId, req)
					if len(endpoints)+len(tasks)+len(aiTraces) != 1 {
						t.Fatal("expected one promoted entity")
					}
					var id uuid.UUID
					var distributedID *uuid.UUID
					var attributes map[string]string
					switch entity.name {
					case "task":
						id, distributedID, attributes = tasks[0].Id, tasks[0].DistributedTraceId, tasks[0].Attributes
					case "endpoint":
						id, distributedID, attributes = endpoints[0].Id, endpoints[0].DistributedTraceId, endpoints[0].Attributes
					case "ai":
						id, distributedID, attributes = aiTraces[0].Id, aiTraces[0].DistributedTraceId, aiTraces[0].Attributes
					}
					log := toLogRecord(testProjectId, &logspb.LogRecord{TraceId: encoding.id, SpanId: spanBytes}, "worker", "", nil, "", "", "", nil)
					if attributes["traceway.otel.trace_id"] != traceHex || attributes["traceway.otel.trace_id"] != log.TraceId {
						t.Fatalf("entity trace ID %q does not correlate with log trace ID %q", attributes["traceway.otel.trace_id"], log.TraceId)
					}
					if id != otelSpanIDToUUID(spanBytes) {
						t.Fatalf("occurrence ID changed: %s", id)
					}
					wantGroup := otelTraceIDToUUID(traceBytes)
					if override {
						wantGroup = groupID
					}
					if distributedID == nil || *distributedID != wantGroup {
						t.Fatalf("distributed grouping changed: %v", distributedID)
					}
				})
			}
		}
	}
}
