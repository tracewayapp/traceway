package otelcontrollers

import (
	"context"
	"encoding/hex"
	"testing"

	"github.com/google/uuid"
	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
)

func TestPromotedEntityLogCorrelation(t *testing.T) {
	const traceHex = "8462dd06157032df3c6c90ac5ff74ed7"
	traceBytes, _ := hex.DecodeString(traceHex)
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
		for _, encoding := range []string{"protobuf", "json"} {
			for _, override := range []bool{false, true} {
				t.Run(entity.name+"/"+encoding+"/override="+map[bool]string{false: "false", true: "true"}[override], func(t *testing.T) {
					attrs := append([]*commonpb.KeyValue{}, entity.attrs...)
					if override {
						attrs = append(attrs, strKV("traceway.distributed_trace_id", groupID.String()))
					}
					req := &coltracepb.ExportTraceServiceRequest{ResourceSpans: []*tracepb.ResourceSpans{{
						ScopeSpans: []*tracepb.ScopeSpans{{Spans: []*tracepb.Span{{
							TraceId: traceBytes, SpanId: spanBytes, ParentSpanId: []byte{1, 2, 3, 4, 5, 6, 7, 8},
							Name: "worker", Kind: entity.kind, Attributes: attrs,
							StartTimeUnixNano: 1_700_000_000_000_000_000, EndTimeUnixNano: 1_700_000_000_001_000_000,
						}}}},
					}}}
					converted := convertTraces(context.Background(), nil, testProjectId, req)
					endpoints, tasks, aiTraces := converted.Endpoints, converted.Tasks, converted.AiTraces
					if len(endpoints)+len(tasks)+len(aiTraces) != 1 {
						t.Fatal("expected one promoted entity")
					}
					var id uuid.UUID
					var traceId string
					switch entity.name {
					case "task":
						id, traceId = tasks[0].Id, tasks[0].TraceId
					case "endpoint":
						id, traceId = endpoints[0].Id, endpoints[0].TraceId
					case "ai":
						id, traceId = aiTraces[0].Id, aiTraces[0].TraceId
					}
					log := toLogRecord(testProjectId, decodeLogIDs(t, encoding, traceBytes, spanBytes), "worker", "", nil, "", "", "", nil)
					if traceId != traceHex || traceId != log.TraceId {
						t.Fatalf("entity trace ID %q does not correlate with log trace ID %q", traceId, log.TraceId)
					}
					if id != otelOccurrenceID(testProjectId, &tracepb.Span{TraceId: traceBytes, SpanId: spanBytes}) {
						t.Fatalf("occurrence ID changed: %s", id)
					}
				})
			}
		}
	}
}
