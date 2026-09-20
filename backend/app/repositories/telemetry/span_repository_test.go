package telemetry

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry/shared"
)

func TestSpanRepository_InsertAndFindByTraceId(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	traceId := uuid.New()
	now := truncateMs(time.Now().UTC())

	s1 := makeSpan(projectId, traceId, "db.query", now, 100*time.Millisecond)
	s2 := makeSpan(projectId, traceId, "http.request", now.Add(10*time.Millisecond), 200*time.Millisecond)
	s3 := makeSpan(projectId, traceId, "cache.get", now.Add(20*time.Millisecond), 50*time.Millisecond)

	err := SpanRepository.InsertAsync(ctx, []models.Span{s1, s2, s3})
	if err != nil {
		t.Fatalf("InsertAsync failed: %v", err)
	}

	found, err := findRunSpans(ctx, projectId, traceId, nil)
	if err != nil {
		t.Fatalf("FindByTraceId failed: %v", err)
	}

	if len(found) != 3 {
		t.Fatalf("expected 3 spans, got %d", len(found))
	}

	// Ordered by start_time ASC
	if found[0].Name != "db.query" {
		t.Errorf("expected first span name 'db.query', got %q", found[0].Name)
	}
	if found[1].Name != "http.request" {
		t.Errorf("expected second span name 'http.request', got %q", found[1].Name)
	}
	if found[2].Name != "cache.get" {
		t.Errorf("expected third span name 'cache.get', got %q", found[2].Name)
	}

	if found[0].Duration != 100*time.Millisecond {
		t.Errorf("expected duration 100ms, got %v", found[0].Duration)
	}
}

func TestSpanRepository_AttributesRoundTrip(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	traceId := uuid.New()
	now := truncateMs(time.Now().UTC())

	withAttrs := makeSpan(projectId, traceId, "db.query", now, 100*time.Millisecond)
	withAttrs.Attributes = map[string]string{
		"db.system":     "postgresql",
		"db.query.text": "SELECT * FROM users",
	}
	withoutAttrs := makeSpan(projectId, traceId, "cache.get", now.Add(10*time.Millisecond), 50*time.Millisecond)

	if err := SpanRepository.InsertAsync(ctx, []models.Span{withAttrs, withoutAttrs}); err != nil {
		t.Fatalf("InsertAsync failed: %v", err)
	}

	found, err := findRunSpans(ctx, projectId, traceId, nil)
	if err != nil {
		t.Fatalf("FindByTraceId failed: %v", err)
	}
	if len(found) != 2 {
		t.Fatalf("expected 2 spans, got %d", len(found))
	}

	if found[0].Attributes["db.system"] != "postgresql" {
		t.Errorf("expected db.system 'postgresql', got %q", found[0].Attributes["db.system"])
	}
	if found[0].Attributes["db.query.text"] != "SELECT * FROM users" {
		t.Errorf("expected db.query.text 'SELECT * FROM users', got %q", found[0].Attributes["db.query.text"])
	}
	if len(found[1].Attributes) != 0 {
		t.Errorf("expected no attributes, got %v", found[1].Attributes)
	}
}

func TestSpanRepository_FindByTraceId_Empty(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()

	found, err := findRunSpans(ctx, uuid.New(), uuid.New(), nil)
	if err != nil {
		t.Fatalf("FindByTraceId failed: %v", err)
	}

	if len(found) != 0 {
		t.Errorf("expected 0 spans for unknown trace, got %d", len(found))
	}
}

func TestSpanRepository_InsertEmpty(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()

	err := SpanRepository.InsertAsync(ctx, []models.Span{})
	if err != nil {
		t.Fatalf("InsertAsync with empty slice should not error: %v", err)
	}
}

func TestSpanRepository_ProjectIsolation(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	project1 := uuid.New()
	project2 := uuid.New()
	traceId := uuid.New()
	now := truncateMs(time.Now().UTC())

	s1 := makeSpan(project1, traceId, "span-p1", now, 100*time.Millisecond)
	s2 := makeSpan(project2, traceId, "span-p2", now, 200*time.Millisecond)

	if err := SpanRepository.InsertAsync(ctx, []models.Span{s1, s2}); err != nil {
		t.Fatalf("InsertAsync failed: %v", err)
	}

	found, err := findRunSpans(ctx, project1, traceId, nil)
	if err != nil {
		t.Fatalf("FindByTraceId failed: %v", err)
	}
	if len(found) != 1 {
		t.Fatalf("expected 1 span for project1, got %d", len(found))
	}
	if found[0].Name != "span-p1" {
		t.Errorf("expected span name 'span-p1', got %q", found[0].Name)
	}
}

// One read serves many lookups, in batches. The same run id in another project, a run nobody asked for and a span
// outside the 24 hour window must all stay out.
func TestSpanRepository_FindGraphsBatchesAndIsolates(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	project1, project2 := uuid.New(), uuid.New()
	now := truncateMs(time.Now().UTC())
	var lookups []shared.SpanLookup
	var spans []models.Span
	runs := make([]uuid.UUID, shared.SpanLookupBatchSize+5)
	for i := range runs {
		runs[i] = uuid.New()
		lookups = append(lookups, shared.SpanLookup{ProjectId: project1, TraceId: hexId(runs[i]), SpanId: hexId(runs[i]), RecordedAt: &now})
		spans = append(spans, makeSpan(project1, runs[i], fmt.Sprintf("span-%d", i), now.Add(-time.Duration(i)*time.Second), time.Millisecond))
	}
	spans[0].Attributes = map[string]string{"db.system": "postgresql"}
	otherProject := makeSpan(project2, runs[0], "other-project", now, time.Millisecond)
	lookups = append(lookups, lookups[0], shared.SpanLookup{ProjectId: project2, TraceId: hexId(runs[0]), SpanId: hexId(runs[0]), RecordedAt: &now})
	excluded := []models.Span{
		makeSpan(uuid.New(), runs[0], "inaccessible-project", now, time.Millisecond),
		makeSpan(project1, uuid.New(), "unrequested-run", now, time.Millisecond),
		makeSpan(project1, runs[0], "outside-window", now.Add(36*time.Hour), time.Millisecond),
	}
	if err := SpanRepository.InsertAsync(ctx, append(append(spans, otherProject), excluded...)); err != nil {
		t.Fatal(err)
	}
	found, err := SpanRepository.FindGraphs(ctx, lookups)
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != len(runs)+1 {
		t.Fatalf("got %d graphs, want one per distinct lookup (%d)", len(found), len(runs)+1)
	}
	for i, lookup := range lookups[:len(runs)] {
		graph := found[lookup.Owner()]
		if graph == nil || len(graph.Spans) != 1 || graph.Spans[0].Name != fmt.Sprintf("span-%d", i) {
			t.Fatalf("run %d: %+v", i, graph)
		}
	}
	if first := found[lookups[0].Owner()].Spans[0]; first.ParentSpanId != hexId(runs[0]) || !reflect.DeepEqual(first.Attributes, spans[0].Attributes) {
		t.Fatalf("span metadata did not round-trip: %+v", first)
	}
	if other := found[lookups[len(lookups)-1].Owner()]; len(other.Spans) != 1 || other.Spans[0].Name != "other-project" {
		t.Fatalf("the same run id in another project is its own graph: %+v", other)
	}
	empty, err := SpanRepository.FindGraphs(ctx, nil)
	if err != nil || len(empty) != 0 {
		t.Fatalf("empty lookup: got %v, %v", empty, err)
	}
}
