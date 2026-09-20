//go:build !telemetry_ch && !telemetry_duckdb && !transactional_pg

package otelcontrollers

import (
	"bytes"
	"context"
	"encoding/hex"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/dbtest"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry"
	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/protobuf/proto"
)

func TestOTLPFullSpanRoundTrip(t *testing.T) {
	dbtest.SetupSQLite(t)
	trace := uuid.New()
	attrs := []*commonpb.KeyValue{
		strKV("service.name", "api"), strKV("exception.stacktrace", "keep original stack"),
		{Key: "int", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_IntValue{IntValue: 9223372036854775807}}},
		{Key: "double", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_DoubleValue{DoubleValue: 1.234}}},
		{Key: "bool", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_BoolValue{BoolValue: true}}},
		{Key: "bytes", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_BytesValue{BytesValue: []byte{0, 255, 128}}}},
		{Key: "array", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_ArrayValue{ArrayValue: &commonpb.ArrayValue{Values: []*commonpb.AnyValue{{Value: &commonpb.AnyValue_StringValue{StringValue: "nested"}}}}}}},
		{Key: "map", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_KvlistValue{KvlistValue: &commonpb.KeyValueList{Values: []*commonpb.KeyValue{strKV("traceId", "this is an attribute")}}}}},
	}
	now := uint64(time.Now().UnixNano())
	source := &tracepb.Span{TraceId: trace[:], SpanId: []byte{128, 2, 3, 4, 5, 6, 7, 255}, ParentSpanId: []byte{8, 7, 6, 5, 4, 3, 2, 1},
		Name: "original name", Kind: tracepb.Span_SPAN_KIND_INTERNAL, TraceState: "vendor=opaque", Flags: 0xabcdef01,
		StartTimeUnixNano: now, EndTimeUnixNano: now + 123456789, Attributes: attrs, DroppedAttributesCount: 3, DroppedEventsCount: 4, DroppedLinksCount: 5,
		Status: &tracepb.Status{Code: tracepb.Status_STATUS_CODE_ERROR, Message: "full status detail"},
		Events: []*tracepb.Span_Event{{TimeUnixNano: now + 1, Name: "event", Attributes: attrs, DroppedAttributesCount: 6}},
		Links:  []*tracepb.Span_Link{{TraceId: trace[:], SpanId: []byte{1, 2, 3, 4, 5, 6, 7, 8}, TraceState: "other=state", Flags: 0xff000301, Attributes: attrs, DroppedAttributesCount: 7}},
	}
	resource := &tracepb.ResourceSpans{SchemaUrl: "https://resource/schema", Resource: &resourcepb.Resource{Attributes: attrs, DroppedAttributesCount: 8},
		ScopeSpans: []*tracepb.ScopeSpans{{SchemaUrl: "https://scope/schema", Scope: &commonpb.InstrumentationScope{Name: "scope", Version: "1.2.3", Attributes: attrs, DroppedAttributesCount: 9}, Spans: []*tracepb.Span{source}}}}
	source.ProtoReflect().SetUnknown([]byte{0xa0, 0x06, 0x01})
	resource.ProtoReflect().SetUnknown([]byte{0xa0, 0x06, 0x02})
	resource.ScopeSpans[0].ProtoReflect().SetUnknown([]byte{0xa0, 0x06, 0x03})
	req := &coltracepb.ExportTraceServiceRequest{ResourceSpans: []*tracepb.ResourceSpans{resource}}
	spans := convertTraces(context.Background(), nil, testProjectId, req).Spans
	// Retries retain each span's full inline context.
	for range 2 {
		if _, err := telemetry.OtelSpanRepository.InsertAsync(context.Background(), append(spans, spans...)); err != nil {
			t.Fatal(err)
		}
	}
	payload, err := telemetry.OtelSpanRepository.FindOTLP(context.Background(), testProjectId, hex.EncodeToString(trace[:]), hex.EncodeToString(source.SpanId), spans[0].StartTime)
	if err != nil {
		t.Fatal(err)
	}
	var got tracepb.ResourceSpans
	if err := proto.Unmarshal(payload, &got); err != nil {
		t.Fatal(err)
	}
	if !proto.Equal(resource, &got) {
		t.Fatalf("OTLP fields changed:\nwant %v\ngot %v", resource, &got)
	}
	var count int
	if err := db.TelemetryDB.QueryRow("SELECT count(*) FROM spans_v2 WHERE resource_schema_url = 'https://resource/schema'").Scan(&count); err != nil || count != 4 {
		t.Fatalf("complete span rows: %d %v", count, err)
	}
	missing, err := telemetry.OtelSpanRepository.FindOTLP(context.Background(), uuid.New(), spans[0].TraceId, spans[0].SpanId, spans[0].StartTime)
	if err != nil || len(missing) != 0 {
		t.Fatalf("cross-project payload leaked: %v", err)
	}
}

func TestOTLPJSONHexAndUnknownFields(t *testing.T) {
	body := []byte(`{"resourceSpans":[{"futureResourceField":true,"scopeSpans":[{"spans":[{"traceId":"0123456789ABCDEF0123456789ABCDEF","spanId":"ABCDEF0123456789","parentSpanId":"FEDCBA9876543210","flags":4294967295,"links":[{"traceId":"1234567890abcdef1234567890abcdef","spanId":"1234567890abcdef"}],"attributes":[{"key":"traceId","value":{"stringValue":"leave this untouched"}}],"futureSpanField":123}]}]}],"futureTopLevelField":{}}`)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/api/otel/v1/traces", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	req, n, err := decodeTraceRequest(c)
	if err != nil {
		t.Fatal(err)
	}
	span := req.ResourceSpans[0].ScopeSpans[0].Spans[0]
	if n != len(body) || len(span.TraceId) != 16 || len(span.SpanId) != 8 || len(span.ParentSpanId) != 8 || len(span.Links[0].TraceId) != 16 || len(span.Links[0].SpanId) != 8 || span.Flags != 0xffffffff || getStringAttribute(span.Attributes, "traceId") != "leave this untouched" {
		t.Fatalf("incorrect OTLP JSON decoding: %v", span)
	}
}
