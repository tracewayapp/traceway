//go:build telemetry_ch

package clickhouse

import (
	"context"
	"encoding/hex"
	"fmt"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/protobuf/proto"
	"math"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	ch "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/chdb"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry/shared"
)

func TestOtelSpanWireRoundTrip(t *testing.T) {
	server := os.Getenv("TEST_CLICKHOUSE_SERVER")
	if server == "" {
		t.Skip("TEST_CLICKHOUSE_SERVER not set")
	}
	ctx := context.Background()
	auth := ch.Auth{Database: "default", Username: os.Getenv("TEST_CLICKHOUSE_USERNAME"), Password: os.Getenv("TEST_CLICKHOUSE_PASSWORD")}
	if auth.Username == "" {
		auth.Username = "default"
	}
	admin, err := ch.Open(&ch.Options{Addr: []string{server}, Auth: auth})
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	database := "span_graph_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if err := admin.Exec(ctx, "CREATE DATABASE "+database); err != nil {
		t.Fatal(err)
	}
	defer admin.Exec(ctx, "DROP DATABASE "+database)
	auth.Database = database
	conn, err := ch.Open(&ch.Options{Addr: []string{server}, Auth: auth})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	previous := chdb.Conn
	chdb.Conn = conn
	defer func() { chdb.Conn = previous }()
	createSpansTable(t, ctx, conn)
	project, start := uuid.New(), time.Now().UTC().Truncate(time.Microsecond)
	span := models.OtelSpan{Span: models.Span{ProjectId: project, TraceId: "0123456789abcdef0123456789abcdef", SpanId: "abcdef1234567890",
		ParentSpanId: "fedcba9876543210", Name: "query", StartTime: start, RecordedAt: start, Duration: 123456789,
		Attributes: map[string]string{"db.system": "postgresql"}, SpanKind: 3, StatusCode: 2, ServiceName: "api", ScopeName: "test"}}
	span.OTLP = &tracepb.Span{Name: "original name", Flags: 0xffffffff, Status: &tracepb.Status{Message: "original status"}, Links: []*tracepb.Span_Link{{Flags: 0x100}}}
	span.OTLP.Attributes = []*commonpb.KeyValue{{Key: "db.system", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "postgresql"}}}}
	span.OTLP.StartTimeUnixNano = uint64(span.StartTime.UnixNano())
	span.Context = &tracepb.ResourceSpans{SchemaUrl: "resource-schema", ScopeSpans: []*tracepb.ScopeSpans{{SchemaUrl: "scope-schema"}}}
	span.OTLP.ProtoReflect().SetUnknown([]byte{0xa0, 0x06, 0x01})
	span.Context.ProtoReflect().SetUnknown([]byte{0xa0, 0x06, 0x02})
	span.Context.ScopeSpans[0].ProtoReflect().SetUnknown([]byte{0xa0, 0x06, 0x03})
	if _, err := OtelSpanRepository.InsertAsync(ctx, []models.OtelSpan{span}); err != nil {
		t.Fatal(err)
	}
	var storedJSON string
	if err := conn.QueryRow(ctx, "SELECT toJSONString(span_attributes) FROM spans_v2").Scan(&storedJSON); err != nil {
		t.Fatal(err)
	}
	t.Log("stored attributes", storedJSON)
	found, err := OtelSpanRepository.FindTraceTopology(ctx, []shared.SpanLookup{{ProjectId: project, TraceId: span.TraceId, SpanId: span.SpanId}}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 1 {
		t.Fatalf("expected one span, got %d", len(found))
	}
	attributes, err := OtelSpanRepository.FindSpanAttributes(ctx, shared.SpanLookup{ProjectId: project, TraceId: span.TraceId}, []string{span.SpanId}, shared.OtelAttributeLimits{PerSpanBytes: shared.MaxOtelSpanAttributeBytes, BudgetBytes: shared.MaxOtelGraphBytes})
	if err != nil {
		t.Fatal(err)
	}
	found[0].Attributes = attributes[span.SpanId].Attributes
	if len(found) != 1 || !reflect.DeepEqual(found[0].Span, span.Span) {
		t.Fatalf("source IDs/timestamps changed: want %+v; got %+v", span.Span, found)
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
	expected.ScopeSpans[0].Spans = []*tracepb.Span{span.OTLP}
	if !proto.Equal(expected, &restored) {
		t.Fatal("OTLP fidelity changed")
	}
	other, err := OtelSpanRepository.FindTraceTopology(ctx, []shared.SpanLookup{{ProjectId: uuid.New(), TraceId: span.TraceId, SpanId: span.SpanId}}, 0)
	if err != nil || len(other) != 0 {
		t.Fatalf("project isolation failed: %+v %v", other, err)
	}
	stringValue := func(value string) *commonpb.AnyValue {
		return &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: value}}
	}
	attrs := []*commonpb.KeyValue{
		{Key: "large", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_IntValue{IntValue: 9223372036854775807}}},
		{Key: "enabled", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_BoolValue{BoolValue: true}}},
		{Key: "http.method", Value: stringValue("POST")},
		{Key: "http%2Emethod", Value: stringValue("literal-percent")},
		{Key: "http", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_KvlistValue{KvlistValue: &commonpb.KeyValueList{Values: []*commonpb.KeyValue{{Key: "method", Value: stringValue("nested")}}}}}},
		{Key: "bytes", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_BytesValue{BytesValue: []byte{0, 255}}}},
		{Key: "nonfinite", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_DoubleValue{DoubleValue: math.Inf(1)}}},
		{Key: "array", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_ArrayValue{ArrayValue: &commonpb.ArrayValue{Values: []*commonpb.AnyValue{stringValue("first"), {Value: &commonpb.AnyValue_IntValue{IntValue: 123}}}}}}},
	}
	for i, mixed := range []*commonpb.AnyValue{stringValue("text"), {Value: &commonpb.AnyValue_IntValue{IntValue: 123}}, {Value: &commonpb.AnyValue_BoolValue{BoolValue: true}}} {
		sample := span
		sample.SpanId = fmt.Sprintf("%016x", i+1)
		sample.OTLP = proto.Clone(span.OTLP).(*tracepb.Span)
		sample.OTLP.SpanId, _ = hex.DecodeString(sample.SpanId)
		sample.OTLP.Attributes = append(append([]*commonpb.KeyValue{}, attrs...), &commonpb.KeyValue{Key: "mixed", Value: mixed})
		sample.OTLP.Events = []*tracepb.Span_Event{{Name: "event", TimeUnixNano: sample.OTLP.StartTimeUnixNano + 1, Attributes: attrs, DroppedAttributesCount: 4}}
		sample.OTLP.Links = []*tracepb.Span_Link{{TraceId: []byte{1, 2, 3}, SpanId: []byte{4, 5}, TraceState: "vendor=state", Flags: 0xffffffff, Attributes: attrs, DroppedAttributesCount: 5}}
		sample.Context = &tracepb.ResourceSpans{Resource: &resourcepb.Resource{Attributes: []*commonpb.KeyValue{{Key: "service.name", Value: stringValue("api")}}}, ScopeSpans: []*tracepb.ScopeSpans{{Scope: &commonpb.InstrumentationScope{Name: "scope", Attributes: attrs}}}}
		if _, err := OtelSpanRepository.InsertAsync(ctx, []models.OtelSpan{sample}); err != nil {
			t.Fatal(err)
		}
		payload, err := OtelSpanRepository.FindOTLP(ctx, project, sample.TraceId, sample.SpanId, sample.StartTime)
		if err != nil {
			t.Fatal(err)
		}
		var decoded tracepb.ResourceSpans
		if err := proto.Unmarshal(payload, &decoded); err != nil {
			t.Fatal(err)
		}
		if !proto.Equal(decoded.ScopeSpans[0].Spans[0], sample.OTLP) || !proto.Equal(decoded.Resource, sample.Context.Resource) {
			t.Fatal("the attribute projection changed lossless span export")
		}
	}
	var large int64
	var enabled, dotted, nested, percent, array, service, eventName, linkedTrace string
	var eventTime, linkFlags uint64
	query := "SELECT toInt64(span_attributes['large']), span_attributes['enabled'], span_attributes['http.method'], span_attributes['http'], span_attributes['http%2Emethod'], span_attributes['array'], JSONExtractString(resource, 'attributes', 'service.name'), JSONExtractString(events, 1, 'name'), JSONExtractUInt(events, 1, 'time_unix_nano'), JSONExtractString(links, 1, 'trace_id'), JSONExtractUInt(links, 1, 'flags') FROM spans_v2 WHERE span_id = '0000000000000001'"
	if err := conn.QueryRow(ctx, query).Scan(&large, &enabled, &dotted, &nested, &percent, &array, &service, &eventName, &eventTime, &linkedTrace, &linkFlags); err != nil {
		t.Fatal(err)
	}
	if large != 9223372036854775807 || enabled != "true" || dotted != "POST" || nested != `{"method":"nested"}` || percent != "literal-percent" || array != `["first",123]` || service != "api" || eventName != "event" || eventTime != span.OTLP.StartTimeUnixNano+1 || linkedTrace != "010203" || linkFlags != 0xffffffff {
		t.Fatalf("queryable values changed: %d %s %s %s %s %s %s %s %d %s %d", large, enabled, dotted, nested, percent, array, service, eventName, eventTime, linkedTrace, linkFlags)
	}
	var literalKey string
	if err := conn.QueryRow(ctx, "SELECT JSONExtractString(events, 1, 'attributes', 'http%2Emethod') FROM spans_v2 WHERE span_id = '0000000000000001'").Scan(&literalKey); err != nil || literalKey != "literal-percent" {
		t.Fatalf("text columns must keep attribute keys verbatim: %q %v", literalKey, err)
	}
	var distinctResources, distinctEnvelopes, matched uint64
	if err := conn.QueryRow(ctx, "SELECT uniqExact(resource), uniqExact(resource_pb), countIf(resource LIKE '%\"service.name\":\"api\"%') FROM spans_v2 WHERE span_id IN ('0000000000000001','0000000000000002','0000000000000003')").Scan(&distinctResources, &distinctEnvelopes, &matched); err != nil {
		t.Fatal(err)
	}
	if distinctResources != 1 || distinctEnvelopes != 1 || matched != 3 {
		t.Fatalf("equal resources must share one dictionary entry and match a LIKE predicate: %d %d %d", distinctResources, distinctEnvelopes, matched)
	}
	var types uint64
	if err := conn.QueryRow(ctx, "SELECT uniqExact(span_attributes['mixed']) FROM spans_v2 WHERE span_id IN ('0000000000000001','0000000000000002','0000000000000003') AND span_attributes['mixed'] IN ('text', '123', 'true')").Scan(&types); err != nil || types != 3 {
		t.Fatalf("one key must hold values of any source type: %d %v", types, err)
	}
}
