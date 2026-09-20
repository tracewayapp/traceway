package shared

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/models"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/protobuf/proto"
)

var testOtelCodec = OtelValueCodec{
	UUID:   func(id uuid.UUID) any { return id },
	Time:   func(t time.Time) any { return t },
	Nanos:  func(n uint64) any { return n },
	Uint32: func(n uint32) any { return n },
}

func stringAttribute(key, value string) *commonpb.KeyValue {
	return &commonpb.KeyValue{Key: key, Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: value}}}
}

func testOtelContext(attributes int) *tracepb.ResourceSpans {
	resource := &resourcepb.Resource{DroppedAttributesCount: 6}
	for i := 0; i < attributes; i++ {
		resource.Attributes = append(resource.Attributes, stringAttribute(fmt.Sprintf("k8s.resource.attribute.%d", i), fmt.Sprintf("value-%d-prod-eu-central-1", i)))
	}
	return &tracepb.ResourceSpans{Resource: resource, SchemaUrl: "resource-schema", ScopeSpans: []*tracepb.ScopeSpans{{
		Scope: &commonpb.InstrumentationScope{Name: "scope", Version: "v1", Attributes: []*commonpb.KeyValue{stringAttribute("scope.key", "value")}}, SchemaUrl: "scope-schema"}}}
}

func testOtelSpan(context *tracepb.ResourceSpans, spanID byte) models.OtelSpan {
	trace := uuid.MustParse("01020304-0506-0708-090a-0b0c0d0e0f10")
	start := time.Unix(1789747200, 123456789).UTC()
	source := &tracepb.Span{TraceId: trace[:], SpanId: []byte{spanID, 2, 3, 4, 5, 6, 7, 8}, Name: "GET /orders", Kind: tracepb.Span_SPAN_KIND_SERVER,
		StartTimeUnixNano: uint64(start.UnixNano()), EndTimeUnixNano: uint64(start.Add(time.Second).UnixNano()), TraceState: "vendor=value", Flags: 0x301,
		Status:     &tracepb.Status{Code: tracepb.Status_STATUS_CODE_ERROR, Message: "detail"},
		Attributes: []*commonpb.KeyValue{stringAttribute("http.request.method", "GET"), stringAttribute("http.route", "/orders")},
		Events:     []*tracepb.Span_Event{{Name: "exception", TimeUnixNano: uint64(start.UnixNano()) + 1}},
		Links:      []*tracepb.Span_Link{{TraceId: trace[:], SpanId: []byte{9, 9, 9, 9, 9, 9, 9, 9}}}}
	return models.OtelSpan{OTLP: source, Context: context, Span: models.Span{ProjectId: uuid.MustParse("00000000-0000-4000-8000-000000000001"),
		TraceId: strings.ReplaceAll(trace.String(), "-", ""), SpanId: fmt.Sprintf("%02x02030405060708", spanID), Name: source.Name, StartTime: start, Duration: time.Second}}
}

func TestOtelPayloadAssemblyMatchesMessageAssembly(t *testing.T) {
	unknown := testOtelContext(3)
	unknown.ProtoReflect().SetUnknown([]byte{0xa0, 0x06, 0x02})
	unknown.ScopeSpans[0].ProtoReflect().SetUnknown([]byte{0xa0, 0x06, 0x03})
	carriesSpans := testOtelContext(2)
	carriesSpans.ScopeSpans[0].Spans = []*tracepb.Span{{Name: "must not be stored with the scope"}}
	for name, context := range map[string]*tracepb.ResourceSpans{
		"populated": testOtelContext(29), "unknown fields": unknown, "no resource or scope": {ScopeSpans: []*tracepb.ScopeSpans{{}}},
		"missing context": nil, "scope already carries spans": carriesSpans,
	} {
		span := testOtelSpan(context, 1)
		span.OTLP.ProtoReflect().SetUnknown([]byte{0xa0, 0x06, 0x01})
		row, err := NewOtelSpanRow(span, OtelSpanGroups{})
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		expected := &tracepb.ResourceSpans{ScopeSpans: []*tracepb.ScopeSpans{{}}}
		if context != nil {
			expected = proto.Clone(context).(*tracepb.ResourceSpans)
		}
		expected.ScopeSpans[0].Spans = []*tracepb.Span{span.OTLP}
		var restored tracepb.ResourceSpans
		if err := proto.Unmarshal(row.Payload(), &restored); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !proto.Equal(expected, &restored) {
			t.Fatalf("%s: assembled payload differs from the source message\nwant %v\ngot  %v", name, expected, &restored)
		}
	}
}

func TestOtelSpanGroupsEncodeSharedContextOnce(t *testing.T) {
	context, groups := testOtelContext(29), OtelSpanGroups{}
	first, err := NewOtelSpanRow(testOtelSpan(context, 1), groups)
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewOtelSpanRow(testOtelSpan(context, 2), groups)
	if err != nil {
		t.Fatal(err)
	}
	if first.Group != second.Group || len(groups) != 1 {
		t.Fatal("spans sharing a context must share one encoded group")
	}
	builds := 0
	for range 3 {
		if _, err := first.Group.Memo("key", func() (string, error) { builds++; return "value", nil }); err != nil {
			t.Fatal(err)
		}
	}
	if builds != 1 {
		t.Fatalf("group encoding ran %d times", builds)
	}
	other, err := NewOtelSpanRow(testOtelSpan(testOtelContext(29), 3), OtelSpanGroups{})
	if err != nil {
		t.Fatal(err)
	}
	if string(other.Group.ResourcePB) != string(first.Group.ResourcePB) || string(other.Group.ScopePB) != string(first.Group.ScopePB) {
		t.Fatal("equal resources from separate requests must encode to identical bytes so dictionary columns deduplicate them")
	}
	if len(first.Group.ResourcePB) == 0 || strings.Contains(string(first.SpanPB), "k8s.resource.attribute") {
		t.Fatal("the span payload must not repeat the resource")
	}
}

func TestOtelSpanRowValuesMatchColumns(t *testing.T) {
	row, err := NewOtelSpanRow(testOtelSpan(testOtelContext(2), 1), OtelSpanGroups{})
	if err != nil {
		t.Fatal(err)
	}
	scalars, err := row.ScalarValues(testOtelCodec)
	if err != nil {
		t.Fatal(err)
	}
	text, err := row.TextValues()
	if err != nil {
		t.Fatal(err)
	}
	if want := len(strings.Split(OtelScalarColumns, ",")); len(scalars) != want {
		t.Fatalf("scalar values %d, columns %d", len(scalars), want)
	}
	if want := len(strings.Split(OtelTextStorageColumns, ",")); len(scalars)+len(text) != want || strings.Count(OtelTextInsertSQL, "?") != want {
		t.Fatalf("text storage values %d, columns %d", len(scalars)+len(text), want)
	}
	multiple := testOtelContext(1)
	multiple.ScopeSpans = append(multiple.ScopeSpans, &tracepb.ScopeSpans{})
	if _, err := NewOtelSpanRow(testOtelSpan(multiple, 1), OtelSpanGroups{}); err == nil {
		t.Fatal("a stored span must belong to exactly one scope")
	}
}

func benchmarkOtelSpans(count int) []models.OtelSpan {
	context := testOtelContext(29)
	spans := make([]models.OtelSpan, count)
	for i := range spans {
		spans[i] = testOtelSpan(context, byte(i%250+1))
	}
	return spans
}

func BenchmarkOtelSpanRowScalars(b *testing.B) {
	spans := benchmarkOtelSpans(512)
	b.ReportAllocs()
	for b.Loop() {
		groups := OtelSpanGroups{}
		for _, span := range spans {
			row, err := NewOtelSpanRow(span, groups)
			if err != nil {
				b.Fatal(err)
			}
			if _, err := row.ScalarValues(testOtelCodec); err != nil {
				b.Fatal(err)
			}
		}
	}
	b.ReportMetric(float64(b.Elapsed().Microseconds())/float64(b.N*len(spans)), "us/span")
}

func BenchmarkOtelSpanRowTextStorage(b *testing.B) {
	spans := benchmarkOtelSpans(512)
	b.ReportAllocs()
	for b.Loop() {
		groups := OtelSpanGroups{}
		for _, span := range spans {
			row, err := NewOtelSpanRow(span, groups)
			if err != nil {
				b.Fatal(err)
			}
			if _, err := row.ScalarValues(testOtelCodec); err != nil {
				b.Fatal(err)
			}
			if _, err := row.TextValues(); err != nil {
				b.Fatal(err)
			}
		}
	}
	b.ReportMetric(float64(b.Elapsed().Microseconds())/float64(b.N*len(spans)), "us/span")
}
