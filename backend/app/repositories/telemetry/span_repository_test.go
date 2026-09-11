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

	found, err := SpanRepository.FindByTraceId(ctx, projectId, traceId, nil)
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

	found, err := SpanRepository.FindByTraceId(ctx, projectId, traceId, nil)
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

	found, err := SpanRepository.FindByTraceId(ctx, uuid.New(), uuid.New(), nil)
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

	found, err := SpanRepository.FindByTraceId(ctx, project1, traceId, nil)
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

func TestSpanRepository_FindByTraces(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	project1, project2 := uuid.New(), uuid.New()
	now := truncateMs(time.Now().UTC())
	later := now.Add(72 * time.Hour)
	var lookups []SpanLookup
	var spans []models.Span
	for i := 0; i < shared.SpanLookupBatchSize+5; i++ {
		traceID := uuid.New()
		lookups = append(lookups, SpanLookup{ProjectId: project1, TraceId: traceID, RecordedAt: &now})
		spans = append(spans, makeSpan(project1, traceID, fmt.Sprintf("span-%d", i), now.Add(-time.Duration(i)*time.Second), time.Millisecond))
	}
	parent := uuid.New()
	spans[0].ParentSpanId = &parent
	spans[0].Attributes = map[string]string{"db.system": "postgresql"}
	// The same span ID in another project or occurrence must remain distinct.
	otherProject := spans[0]
	otherProject.ProjectId = project2
	otherProject.Name = "other-project"
	otherOccurrence := spans[0]
	otherOccurrence.StartTime, otherOccurrence.RecordedAt = later, later
	otherOccurrence.Name = "later-occurrence"
	lookups = append(lookups,
		lookups[0],
		SpanLookup{ProjectId: project2, TraceId: spans[0].TraceId, RecordedAt: &now},
		SpanLookup{ProjectId: project1, TraceId: spans[0].TraceId, RecordedAt: &later},
	)
	spans = append(spans, otherProject, otherOccurrence)
	excluded := []models.Span{
		makeSpan(uuid.New(), spans[0].TraceId, "inaccessible-project", now, time.Millisecond),
		makeSpan(project1, uuid.New(), "unrequested-owner", now, time.Millisecond),
		makeSpan(project1, spans[0].TraceId, "outside-window", now.Add(36*time.Hour), time.Millisecond),
	}
	if err := SpanRepository.InsertAsync(ctx, append(spans, excluded...)); err != nil {
		t.Fatal(err)
	}
	found, err := SpanRepository.FindByTraces(ctx, lookups)
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != len(spans) {
		t.Fatalf("got %d spans, want %d", len(found), len(spans))
	}
	for i, span := range found {
		if i > 0 && span.StartTime.Before(found[i-1].StartTime) {
			t.Fatal("spans are not ordered across batches")
		}
		if span.Name == "span-0" && (span.ParentSpanId == nil || *span.ParentSpanId != parent || !reflect.DeepEqual(span.Attributes, spans[0].Attributes)) {
			t.Fatalf("span metadata did not round-trip: %+v", span)
		}
		for _, unwanted := range excluded {
			if span.Id == unwanted.Id {
				t.Fatalf("returned excluded span %s", span.Name)
			}
		}
	}
	empty, err := SpanRepository.FindByTraces(ctx, nil)
	if err != nil || len(empty) != 0 {
		t.Fatalf("empty lookup: got %v, %v", empty, err)
	}
}
