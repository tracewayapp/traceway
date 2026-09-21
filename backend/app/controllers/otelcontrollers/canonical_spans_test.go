package otelcontrollers

import (
	"context"
	"encoding/hex"
	"reflect"
	"testing"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/services"
	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
)

func spanRequest(spans ...*tracepb.Span) *coltracepb.ExportTraceServiceRequest {
	return &coltracepb.ExportTraceServiceRequest{ResourceSpans: []*tracepb.ResourceSpans{{ScopeSpans: []*tracepb.ScopeSpans{{Spans: spans}}}}}
}

func TestCanonicalSpansBatchIndependent(t *testing.T) {
	trace := uuid.New()
	root := &tracepb.Span{TraceId: trace[:], SpanId: []byte{1, 1, 1, 1, 1, 1, 1, 1}, Kind: tracepb.Span_SPAN_KIND_CONSUMER, Name: "worker"}
	child := &tracepb.Span{TraceId: trace[:], SpanId: []byte{2, 2, 2, 2, 2, 2, 2, 2}, ParentSpanId: root.SpanId, Kind: tracepb.Span_SPAN_KIND_INTERNAL, Attributes: []*commonpb.KeyValue{strKV("http.request.method", "GET")}}
	all := convertTraces(context.Background(), nil, testProjectId, spanRequest(root, child)).Spans
	split := append(convertTraces(context.Background(), nil, testProjectId, spanRequest(root)).Spans, convertTraces(context.Background(), nil, testProjectId, spanRequest(child)).Spans...)
	if !reflect.DeepEqual(all, split) {
		t.Fatal("batching changed canonical graph")
	}
	for _, req := range []*coltracepb.ExportTraceServiceRequest{spanRequest(root, child), spanRequest(child)} {
		endpoints := convertTraces(context.Background(), nil, testProjectId, req).Endpoints
		if len(endpoints) != 0 {
			t.Fatal("internal child classification depends on parent arrival")
		}
	}
	req := spanRequest(root)
	req.ResourceSpans = append(req.ResourceSpans, spanRequest(child).ResourceSpans...)
	if !reflect.DeepEqual(all, convertTraces(context.Background(), nil, testProjectId, req).Spans) {
		t.Fatal("resource split changed graph")
	}
}

// The entity id is the one derived value: it must differ across projects and traces and stay put across retries. The
// span keeps the ids it arrived with, and the browser's trace id never replaces them.
func TestCanonicalIdentityIncludesTraceAndProject(t *testing.T) {
	trace := uuid.MustParse("01234567-89ab-cdef-0123-456789abcdef")
	otherTrace, browser := uuid.New(), uuid.New()
	span := &tracepb.Span{TraceId: trace[:], SpanId: []byte{1, 2, 3, 4, 5, 6, 7, 8}}
	a := otelOccurrenceID(testProjectId, span)
	if a.String() != "211293e1-1332-5ff7-8097-c9b3a439f2fc" {
		t.Fatalf("historical occurrence ID changed: %s", a)
	}
	if a == otelOccurrenceID(uuid.New(), span) || a != otelOccurrenceID(testProjectId, span) {
		t.Fatal("entity id must depend on the project and be stable")
	}
	span.TraceId = otherTrace[:]
	if a == otelOccurrenceID(testProjectId, span) {
		t.Fatal("entity id collision across traces")
	}
	span.Kind = tracepb.Span_SPAN_KIND_SERVER
	span.Attributes = []*commonpb.KeyValue{strKV("http.request.method", "GET"), strKV("traceway.distributed_trace_id", browser.String())}
	converted := convertTraces(context.Background(), nil, testProjectId, spanRequest(span))
	stored := converted.Spans[0]
	if stored.TraceId != hex.EncodeToString(otherTrace[:]) || stored.SpanId != "0102030405060708" || stored.ParentSpanId != "" {
		t.Fatalf("a span keeps the ids it arrived with: %+v", stored.Span)
	}
	endpoints := converted.Endpoints
	if len(endpoints) != 1 || endpoints[0].TraceId != stored.TraceId || endpoints[0].SpanId != stored.SpanId || !endpoints[0].IsRoot {
		t.Fatalf("the endpoint must retain its own trace ID without custom linking: %+v", endpoints)
	}
	for _, ignored := range []string{"not-a-uuid", otherTrace.String(), ""} {
		span.Attributes = []*commonpb.KeyValue{strKV("http.request.method", "GET"), strKV("traceway.distributed_trace_id", ignored)}
		if endpoints := convertTraces(context.Background(), nil, testProjectId, spanRequest(span)).Endpoints; endpoints[0].TraceId != stored.TraceId {
			t.Fatalf("%q is no link: %+v", ignored, endpoints[0])
		}
	}
}

func TestCanonicalKeepsUnclassifiedRootsAndRejectsInvalidIDs(t *testing.T) {
	trace := uuid.New()
	root := &tracepb.Span{TraceId: trace[:], SpanId: []byte{1, 2, 3, 4, 5, 6, 7, 8}, Kind: tracepb.Span_SPAN_KIND_CLIENT}
	invalid := &tracepb.Span{TraceId: []byte{1}, SpanId: root.SpanId}
	zero := &tracepb.Span{TraceId: make([]byte, 16), SpanId: root.SpanId}
	badParent := &tracepb.Span{TraceId: trace[:], SpanId: root.SpanId, ParentSpanId: []byte{1}}
	converted := convertTraces(context.Background(), nil, testProjectId, spanRequest(root, invalid, zero, badParent, nil))
	found := converted.Spans
	if len(found) != 1 || found[0].ParentSpanId != "" {
		t.Fatalf("unexpected roots: %+v", found)
	}
	if converted.InvalidSpanIDs != 4 {
		t.Fatalf("invalid span count = %d, want 4", converted.InvalidSpanIDs)
	}
	// Frontend projects suppress product entities, but their source graph is retained.
	converted = convertTraces(context.Background(), &models.Project{Framework: "react"}, testProjectId, spanRequest(root))
	ep, tasks, ai := converted.Endpoints, converted.Tasks, converted.AiTraces
	if len(ep)+len(tasks)+len(ai) != 0 || len(converted.Spans) != 1 {
		t.Fatal("unexpected projection")
	}
}

func TestCanonicalHealthcheckFilterKeepsPromotedDescendants(t *testing.T) {
	trace := uuid.New()
	root := &tracepb.Span{TraceId: trace[:], SpanId: []byte{1, 1, 1, 1, 1, 1, 1, 1}}
	child := &tracepb.Span{TraceId: trace[:], SpanId: []byte{2, 2, 2, 2, 2, 2, 2, 2}, ParentSpanId: root.SpanId}
	promoted := &tracepb.Span{TraceId: trace[:], SpanId: []byte{3, 3, 3, 3, 3, 3, 3, 3}, ParentSpanId: root.SpanId}
	spans := convertTraces(context.Background(), nil, testProjectId, spanRequest(child, root, promoted)).Spans
	traceId := hex.EncodeToString(trace[:])
	kept := services.DropSpanSubtrees(spans, func(span models.OtelSpan) (string, string, string) {
		return span.TraceId, span.SpanId, span.ParentSpanId
	},
		map[string]bool{services.SpanKey(traceId, "0101010101010101"): true}, map[string]bool{services.SpanKey(traceId, "0303030303030303"): true})
	if len(kept) != 1 || kept[0].SpanId != "0303030303030303" {
		t.Fatalf("incorrect healthcheck subtree filtering: %+v", kept)
	}
}
