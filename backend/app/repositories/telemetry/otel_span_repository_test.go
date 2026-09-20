package telemetry

import (
	"context"
	"encoding/hex"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry/shared"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/protobuf/proto"
)

// canonicalSpan takes its ids as UUIDs for the fixtures' convenience: the trace id is all 16 bytes, a span id the last 8.
func canonicalSpan(project, trace, id uuid.UUID, parent *uuid.UUID) models.OtelSpan {
	span := models.OtelSpan{Span: models.Span{
		ProjectId: project, TraceId: hex.EncodeToString(trace[:]), SpanId: hex.EncodeToString(id[8:]),
		Name: "operation", StartTime: time.Now().UTC(), RecordedAt: time.Now().UTC(), Duration: time.Second,
	}}
	if parent != nil {
		span.ParentSpanId = hex.EncodeToString(parent[8:])
	}
	return span
}

func TestOtelGraphLateParentsAndIsolation(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	project, trace := uuid.New(), uuid.New()
	rootID := uuid.MustParse("00000000-0000-0000-0101-010101010101")
	childID := uuid.MustParse("00000000-0000-0000-0202-020202020202")
	grandchildID := uuid.MustParse("00000000-0000-0000-0303-030303030303")
	root := canonicalSpan(project, trace, rootID, nil)
	child := canonicalSpan(project, trace, childID, &rootID)
	grandchild := canonicalSpan(project, trace, grandchildID, &childID)
	// Still inside the 24 hours either side of the root that every read is held to.
	grandchild.StartTime = root.StartTime.Add(23 * time.Hour)
	grandchild.RecordedAt = grandchild.StartTime
	reusedTrace := canonicalSpan(project, uuid.New(), childID, &rootID)
	reusedProject := canonicalSpan(uuid.New(), trace, childID, &rootID)
	// Child exports first; unrelated reused span IDs must not attach to this root.
	if _, err := OtelSpanRepository.InsertAsync(ctx, []models.OtelSpan{grandchild, reusedTrace, reusedProject}); err != nil {
		t.Fatal(err)
	}
	if _, err := OtelSpanRepository.InsertAsync(ctx, []models.OtelSpan{root}); err != nil {
		t.Fatal(err)
	}
	// Its parent has not arrived, so the trace's root entity holds it until the child does.
	found, err := findSpans(ctx, root, &root.RecordedAt)
	if err != nil || len(found) != 1 || found[0].SpanId != grandchild.SpanId {
		t.Fatalf("orphan under the root entity: %v, %v", found, err)
	}
	if _, err := OtelSpanRepository.InsertAsync(ctx, []models.OtelSpan{child, grandchild}); err != nil {
		t.Fatal(err)
	}
	found, err = findSpans(ctx, root, &root.RecordedAt)
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 2 {
		t.Fatalf("want child and long-running grandchild, got %d", len(found))
	}
	if found[0].SpanId != child.SpanId || found[1].SpanId != grandchild.SpanId {
		t.Fatalf("wrong tree: %+v", found)
	}
	for _, span := range found {
		if span.ProjectId != project || span.TraceId != root.TraceId || span.ParentSpanId == "" {
			t.Fatalf("lost identity: %+v", span)
		}
	}
	found, err = findSpans(ctx, child, &child.RecordedAt)
	if err != nil || len(found) != 1 || found[0].SpanId != grandchild.SpanId {
		t.Fatalf("nested promoted subtree: %v %v", found, err)
	}
}

func TestOtelGraphCyclesAndDuplicateExports(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	project, trace := uuid.New(), uuid.New()
	aID := uuid.MustParse("00000000-0000-0000-0101-010101010101")
	bID := uuid.MustParse("00000000-0000-0000-0202-020202020202")
	a, b := canonicalSpan(project, trace, aID, &bID), canonicalSpan(project, trace, bID, &aID)
	duplicate := b
	duplicate.Duration = 2 * time.Second
	if _, err := OtelSpanRepository.InsertAsync(ctx, []models.OtelSpan{a, b, duplicate, a, b}); err != nil {
		t.Fatal(err)
	}
	found, err := findSpans(ctx, a, &a.RecordedAt)
	if err != nil || len(found) != 1 || found[0].Duration != 2*time.Second {
		t.Fatalf("cycle or duplicate: %+v %v", found, err)
	}
}

func TestOtelPayloadContextRoundTrip(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	span := canonicalSpan(uuid.New(), uuid.New(), uuid.New(), nil)
	span.OTLP = &tracepb.Span{Name: "original", Flags: 0xffffffff, StartTimeUnixNano: 1789747200123456789,
		Events: []*tracepb.Span_Event{{Name: "event", TimeUnixNano: 1789747200123456790}},
		Status: &tracepb.Status{Code: tracepb.Status_STATUS_CODE_ERROR, Message: "status detail"}}
	span.Context = &tracepb.ResourceSpans{SchemaUrl: "resource-schema", ScopeSpans: []*tracepb.ScopeSpans{{SchemaUrl: "scope-schema"}}}
	for range 2 {
		if _, err := OtelSpanRepository.InsertAsync(ctx, []models.OtelSpan{span, span}); err != nil {
			t.Fatal(err)
		}
	}
	// The row is filed under the OTLP start, not under the wall clock the fixture was built at.
	payload, err := OtelSpanRepository.FindOTLP(ctx, span.ProjectId, span.TraceId, span.SpanId, shared.OtelNanosToTime(span.OTLP.StartTimeUnixNano))
	if err != nil {
		t.Fatal(err)
	}
	var resource tracepb.ResourceSpans
	if err := proto.Unmarshal(payload, &resource); err != nil {
		t.Fatal(err)
	}
	if resource.SchemaUrl != span.Context.SchemaUrl || !proto.Equal(resource.ScopeSpans[0].Spans[0], span.OTLP) {
		t.Fatalf("OTLP data changed: %v", &resource)
	}
}

// findSpans reads the spans under one span the way the detail pages do, through the graph facade.
func findSpans(ctx context.Context, under models.OtelSpan, recordedAt *time.Time) ([]models.Span, error) {
	graph, err := SpanRepository.FindGraph(ctx, shared.SpanLookup{ProjectId: under.ProjectId, TraceId: under.TraceId, SpanId: under.SpanId, RecordedAt: recordedAt})
	if err != nil {
		return nil, err
	}
	return graph.Spans, nil
}
