package shared

import (
	"reflect"
	"strings"
	"testing"

	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
)

func TestOtelResourceTextIsDeterministic(t *testing.T) {
	values := map[string]string{"service.name": "api", "k8s.pod.name": "cart-1", "note": `a<b & "q" 100%_done`}
	build := func(order ...string) string {
		resource := &resourcepb.Resource{DroppedAttributesCount: 3}
		for _, key := range order {
			resource.Attributes = append(resource.Attributes, &commonpb.KeyValue{Key: key, Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: values[key]}}})
		}
		text, err := MetadataJSON(resource, resource.Attributes)
		if err != nil {
			t.Fatal(err)
		}
		return text
	}
	want := `{"attributes":{"k8s.pod.name":"cart-1","note":"a\u003cb \u0026 \"q\" 100%_done","service.name":"api"},"droppedAttributesCount":3}`
	for range 50 {
		if got := build("note", "k8s.pod.name", "service.name"); got != want || got != build("service.name", "note", "k8s.pod.name") {
			t.Fatalf("dictionary columns and LIKE filters need stable resource text:\nwant %s\ngot  %s", want, got)
		}
	}
}

func TestOtelTextColumnsKeepAttributeKeysVerbatim(t *testing.T) {
	attributes := []*commonpb.KeyValue{{Key: "rate%", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "high"}}}}
	events, err := EventsJSON([]*tracepb.Span_Event{{Name: "event", TimeUnixNano: 18446744073709551615, Attributes: attributes, DroppedAttributesCount: 2}})
	if err != nil {
		t.Fatal(err)
	}
	links, err := LinksJSON([]*tracepb.Span_Link{{TraceId: []byte{1, 2}, SpanId: []byte{3}, TraceState: "vendor=state", Flags: 0xffffffff, Attributes: attributes}})
	if err != nil {
		t.Fatal(err)
	}
	if events != `[{"time_unix_nano":18446744073709551615,"name":"event","attributes":{"rate%":"high"},"dropped_attributes_count":2}]` ||
		links != `[{"trace_id":"0102","span_id":"03","trace_state":"vendor=state","flags":4294967295,"attributes":{"rate%":"high"},"dropped_attributes_count":0}]` {
		t.Fatalf("text columns changed:\n%s\n%s", events, links)
	}
	if stored := StringAttributes(attributes); stored["rate%"] != "high" || strings.Contains(events, "%25") {
		t.Fatal("attribute keys must be stored verbatim everywhere")
	}
	if empty, err := EventsJSON(nil); err != nil || empty != "[]" {
		t.Fatalf("empty events: %q %v", empty, err)
	}
}

func TestOtelSpanAttributesMapFollowsTheCollectorConvention(t *testing.T) {
	text := func(v string) *commonpb.AnyValue {
		return &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: v}}
	}
	stored := StringAttributes([]*commonpb.KeyValue{
		{Key: "http.method", Value: text("POST")},
		{Key: "http%2Emethod", Value: text("literal")},
		{Key: "large", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_IntValue{IntValue: 9223372036854775807}}},
		{Key: "enabled", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_BoolValue{BoolValue: true}}},
		{Key: "ratio", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_DoubleValue{DoubleValue: 1.25}}},
		{Key: "list", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_ArrayValue{ArrayValue: &commonpb.ArrayValue{Values: []*commonpb.AnyValue{text("a"), {Value: &commonpb.AnyValue_IntValue{IntValue: 2}}}}}}},
		{Key: "nested", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_KvlistValue{KvlistValue: &commonpb.KeyValueList{Values: []*commonpb.KeyValue{{Key: "k", Value: text("v")}}}}}},
		{Key: "unset"},
		{Key: "exception.stacktrace", Value: text("Error: boom\n  at f")},
	})
	want := map[string]string{"http.method": "POST", "http%2Emethod": "literal", "large": "9223372036854775807", "enabled": "true", "ratio": "1.25", "list": `["a",2]`, "nested": `{"k":"v"}`}
	if !reflect.DeepEqual(stored, want) {
		t.Fatalf("span_attributes map:\nwant %v\ngot  %v", want, stored)
	}
}
