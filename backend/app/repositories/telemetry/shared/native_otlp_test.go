package shared

import (
	"bytes"
	"encoding/hex"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/models"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/protobuf/proto"
)

func TestRegressionNativeOTLPConformance(t *testing.T) {
	s := models.OtelSpan{Span: models.Span{ProjectId: uuid.New(), TraceId: "0123456789abcdef0123456789abcdef", SpanId: "abcdef0123456789abcdef0123456789", Name: "GET /probe", StartTime: time.Now(), Duration: time.Second, ServiceName: "native-service", ScopeName: "native-scope", ParentSpanId: "fedcba9876543210fedcba9876543210", Attributes: map[string]string{"http.request.method": "GET"}}}
	r, err := NewOtelSpanRow(s, OtelSpanGroups{})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Source.SpanId) != 8 {
		t.Errorf("native export span ID has %d bytes, OTLP requires 8", len(r.Source.SpanId))
	}
	if len(r.Source.Attributes) == 0 {
		t.Error("native span attributes missing from exported protobuf")
	}
	if len(r.Group.Resource.GetAttributes()) == 0 {
		t.Error("native service resource missing from exported protobuf")
	}
}

func TestNativeOTLPIdentityMappingAndMetadata(t *testing.T) {
	traceID := "0123456789abcdef0123456789abcdef"
	parent := models.OtelSpan{Span: models.Span{TraceId: traceID, SpanId: "fedcba9876543210fedcba9876543210", ServiceName: "parent-service", ScopeName: "parent-scope", StartTime: time.Now(), Duration: time.Second}}
	child := parent
	child.SpanId, child.ParentSpanId = "abcdef0123456789abcdef0123456789", parent.SpanId
	child.ServiceName, child.ScopeName = "child-service", "child-scope"
	child.Attributes = map[string]string{"z": "last", "a": "first", "unicode": "界"}
	groups := OtelSpanGroups{}
	parentRow, err := NewOtelSpanRow(parent, groups)
	if err != nil {
		t.Fatal(err)
	}
	childRow, err := NewOtelSpanRow(child, groups)
	if err != nil {
		t.Fatal(err)
	}
	repeated, err := NewOtelSpanRow(child, groups)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(childRow.Source.ParentSpanId, parentRow.Source.SpanId) || !bytes.Equal(childRow.Payload(), repeated.Payload()) {
		t.Fatal("native mapping or encoding is not deterministic")
	}
	var envelope tracepb.ResourceSpans
	if err := proto.Unmarshal(childRow.Payload(), &envelope); err != nil {
		t.Fatal(err)
	}
	scope := envelope.ScopeSpans[0]
	exported := scope.Spans[0]
	if hex.EncodeToString(exported.TraceId) != traceID || scope.Scope.Name != "child-scope" || StringAttributes(envelope.Resource.Attributes)["service.name"] != "child-service" {
		t.Fatal("metadata leaked between native spans sharing a batch")
	}
	attributes := StringAttributes(exported.Attributes)
	for key, value := range child.Attributes {
		if attributes[key] != value {
			t.Fatalf("attribute %s was lost", key)
		}
	}
	if attributes["traceway.native.span_id"] != child.SpanId || attributes["traceway.native.parent_span_id"] != parent.SpanId {
		t.Fatal("native identity mapping was not exported")
	}
	for _, id := range []string{"", "00", "0000000000000000", "00000000000000000000000000000000", "invalid"} {
		if _, err := nativeOTLPSpanID(id); err == nil {
			t.Errorf("accepted invalid span ID %q", id)
		}
	}
	existing := "0123456789abcdef"
	raw, err := nativeOTLPSpanID(existing)
	if err != nil || hex.EncodeToString(raw) != existing {
		t.Fatal("an existing eight-byte ID must be unchanged")
	}
}
