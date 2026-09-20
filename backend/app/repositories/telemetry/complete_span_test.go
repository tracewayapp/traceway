//go:build !telemetry_ch

package telemetry

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry/shared"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/protobuf/proto"
)

func TestCompleteSpanFieldsAndNativeSpans(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	project, trace := uuid.New(), uuid.New()
	sourceID := uuid.New()
	span := canonicalSpan(project, trace, sourceID, nil)
	span.StartTime = time.Unix(1789747200, 123456789).UTC()
	attrs := []*commonpb.KeyValue{
		{Key: "large", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_IntValue{IntValue: 9223372036854775807}}},
		{Key: "enabled", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_BoolValue{BoolValue: true}}},
		{Key: "bytes", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_BytesValue{BytesValue: []byte{0, 255}}}},
		{Key: "nested", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_KvlistValue{KvlistValue: &commonpb.KeyValueList{Values: []*commonpb.KeyValue{{Key: "array", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_ArrayValue{ArrayValue: &commonpb.ArrayValue{Values: []*commonpb.AnyValue{{Value: &commonpb.AnyValue_DoubleValue{DoubleValue: 1.25}}}}}}}}}}}},
	}
	source := &tracepb.Span{TraceId: trace[:], SpanId: sourceID[8:], Name: span.Name, StartTimeUnixNano: uint64(span.StartTime.UnixNano()), EndTimeUnixNano: uint64(span.StartTime.Add(span.Duration).UnixNano()), TraceState: "vendor=value", Flags: 0xffffffff, Attributes: attrs, Status: &tracepb.Status{Code: tracepb.Status_STATUS_CODE_ERROR, Message: "detail"}, DroppedAttributesCount: 1, DroppedEventsCount: 2, DroppedLinksCount: 3,
		Events: []*tracepb.Span_Event{{Name: "exception", TimeUnixNano: uint64(span.StartTime.UnixNano() + 1), Attributes: attrs, DroppedAttributesCount: 4}},
		Links:  []*tracepb.Span_Link{{TraceId: trace[:], SpanId: sourceID[8:], TraceState: "other=value", Flags: 0xffffffff, Attributes: attrs, DroppedAttributesCount: 5}}}
	span.OTLP = source
	span.Context = &tracepb.ResourceSpans{Resource: &resourcepb.Resource{Attributes: attrs, DroppedAttributesCount: 6}, SchemaUrl: "resource-schema", ScopeSpans: []*tracepb.ScopeSpans{{Scope: &commonpb.InstrumentationScope{Name: "scope", Version: "v1", Attributes: attrs, DroppedAttributesCount: 7}, SchemaUrl: "scope-schema"}}}
	// Unknown protobuf fields must survive even though the query columns only expose known fields.
	source.ProtoReflect().SetUnknown([]byte{0xa0, 0x06, 0x01})
	span.Context.ProtoReflect().SetUnknown([]byte{0xa0, 0x06, 0x02})
	span.Context.ScopeSpans[0].ProtoReflect().SetUnknown([]byte{0xa0, 0x06, 0x03})
	if _, err := OtelSpanRepository.InsertAsync(ctx, []models.OtelSpan{span}); err != nil {
		t.Fatal(err)
	}
	var attrJSON, eventsJSON, linksJSON, resourceJSON, scopeJSON, traceState, message, resourceSchema, scopeSchema, scopeVersion string
	var flags, attrDrops, eventDrops, linkDrops int64
	if err := db.TelemetryDB.QueryRow(`SELECT span_attributes,events,links,resource,scope,trace_state,flags,status_message,resource_schema_url,scope_schema_url,scope_version,dropped_attributes_count,dropped_events_count,dropped_links_count FROM spans_v2`).Scan(&attrJSON, &eventsJSON, &linksJSON, &resourceJSON, &scopeJSON, &traceState, &flags, &message, &resourceSchema, &scopeSchema, &scopeVersion, &attrDrops, &eventDrops, &linkDrops); err != nil {
		t.Fatal(err)
	}
	// The SQL-visible columns are the readable form every backend stores. Exact types live in the payload, checked below.
	var attributes map[string]string
	if err := json.Unmarshal([]byte(attrJSON), &attributes); err != nil {
		t.Fatal(err)
	}
	if want := map[string]string{"large": "9223372036854775807", "enabled": "true", "bytes": "AP8=", "nested": `{"array":[1.25]}`}; !reflect.DeepEqual(attributes, want) {
		t.Fatalf("span_attributes:\nwant %v\ngot  %v", want, attributes)
	}
	var events []struct {
		Name       string         `json:"name"`
		Time       uint64         `json:"time_unix_nano"`
		Attributes map[string]any `json:"attributes"`
		Dropped    uint32         `json:"dropped_attributes_count"`
	}
	var links []struct {
		TraceId    string         `json:"trace_id"`
		SpanId     string         `json:"span_id"`
		TraceState string         `json:"trace_state"`
		Flags      uint32         `json:"flags"`
		Attributes map[string]any `json:"attributes"`
		Dropped    uint32         `json:"dropped_attributes_count"`
	}
	if err := json.Unmarshal([]byte(eventsJSON), &events); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(linksJSON), &links); err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Name != "exception" || events[0].Time != uint64(span.StartTime.UnixNano()+1) || events[0].Dropped != 4 || events[0].Attributes["enabled"] != true {
		t.Fatalf("events column changed: %s", eventsJSON)
	}
	if len(links) != 1 || links[0].TraceId != hex.EncodeToString(trace[:]) || links[0].SpanId != hex.EncodeToString(sourceID[8:]) || links[0].TraceState != "other=value" || links[0].Flags != 0xffffffff || links[0].Dropped != 5 {
		t.Fatalf("links column changed: %s", linksJSON)
	}
	var resource, scope struct {
		Name       string         `json:"name"`
		Version    string         `json:"version"`
		Attributes map[string]any `json:"attributes"`
		Dropped    uint32         `json:"droppedAttributesCount"`
	}
	if err := json.Unmarshal([]byte(resourceJSON), &resource); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(scopeJSON), &scope); err != nil {
		t.Fatal(err)
	}
	if len(resource.Attributes) != 4 || resource.Dropped != 6 || scope.Name != "scope" || scope.Version != "v1" || len(scope.Attributes) != 4 || scope.Dropped != 7 {
		t.Fatalf("resource/scope columns changed:\n%s\n%s", resourceJSON, scopeJSON)
	}
	if traceState != "vendor=value" || flags != 0xffffffff || message != "detail" || resourceSchema != "resource-schema" || scopeSchema != "scope-schema" || scopeVersion != "v1" || attrDrops != 1 || eventDrops != 2 || linkDrops != 3 {
		t.Fatal("scalar OTLP fields changed")
	}
	payload, err := OtelSpanRepository.FindOTLP(ctx, project, span.TraceId, span.SpanId, span.StartTime)
	if err != nil {
		t.Fatal(err)
	}
	var restored tracepb.ResourceSpans
	if err := proto.Unmarshal(payload, &restored); err != nil {
		t.Fatal(err)
	}
	expected := proto.Clone(span.Context).(*tracepb.ResourceSpans)
	expected.ScopeSpans[0].Spans = []*tracepb.Span{source}
	if !proto.Equal(expected, &restored) {
		t.Fatal("inline OTLP round trip changed source data")
	}
	// A span of the native protocol has no OTLP source. It is stored in the same table and read the same way.
	run := uuid.New()
	native := makeSpan(project, run, "native", span.StartTime, time.Second)
	native.SpanKind, native.StatusCode = 3, 2
	if err := SpanRepository.InsertAsync(ctx, []models.Span{native}); err != nil {
		t.Fatal(err)
	}
	stored, err := findRunSpans(ctx, project, run, &span.StartTime)
	if err != nil || len(stored) != 1 || stored[0].SpanId != native.SpanId || stored[0].SpanKind != 3 || stored[0].StatusCode != 2 {
		t.Fatalf("native span: %+v %v", stored, err)
	}
	synthesized, err := OtelSpanRepository.FindOTLP(ctx, project, native.TraceId, native.SpanId, span.StartTime)
	if err != nil {
		t.Fatal(err)
	}
	var nativePayload tracepb.ResourceSpans
	if err := proto.Unmarshal(synthesized, &nativePayload); err != nil || nativePayload.ScopeSpans[0].Spans[0].Name != "native" {
		t.Fatalf("a native span downloads as OTLP too: %v %v", &nativePayload, err)
	}
	// No Endpoint, Task, or AI row exists: source lookup still returns the complete trace.
	found, err := OtelSpanRepository.FindTraceTopology(ctx, []shared.SpanLookup{{ProjectId: project, TraceId: hex.EncodeToString(trace[:]), RecordedAt: &span.StartTime}}, 0)
	if err != nil || len(found) != 1 || !found[0].StartTime.Equal(span.StartTime) {
		t.Fatalf("source lookup or nanosecond precision failed: %+v %v", found, err)
	}
}
