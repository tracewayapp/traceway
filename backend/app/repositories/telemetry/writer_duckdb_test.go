//go:build telemetry_duckdb

package telemetry

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
	duckdbrepo "github.com/tracewayapp/traceway/backend/app/repositories/telemetry/duckdb"
)

func startTestWriters(t *testing.T, opts duckdbrepo.WriterOptions) {
	t.Helper()
	duckdbrepo.StartWriters(context.Background(), opts)
	t.Cleanup(duckdbrepo.StopWriters)
}

func TestWriterBarrierReadYourWrites(t *testing.T) {
	setupTestDB(t)
	// Huge thresholds so only the barrier can flush.
	startTestWriters(t, duckdbrepo.WriterOptions{FlushRows: 1 << 20, FlushInterval: time.Hour})

	ctx := context.Background()
	projectId := uuid.New()
	traceId := uuid.New()
	now := truncateMs(time.Now().UTC())

	err := SpanRepository.InsertAsync(ctx, []models.Span{
		makeSpan(projectId, traceId, "db.query", now, 100*time.Millisecond),
		makeSpan(projectId, traceId, "http.request", now.Add(10*time.Millisecond), 200*time.Millisecond),
	})
	if err != nil {
		t.Fatalf("InsertAsync failed: %v", err)
	}

	if err := duckdbrepo.FlushWriters(ctx); err != nil {
		t.Fatalf("FlushWriters failed: %v", err)
	}

	found, err := SpanRepository.FindByTraceId(ctx, projectId, traceId, nil)
	if err != nil {
		t.Fatalf("FindByTraceId failed: %v", err)
	}
	if len(found) != 2 {
		t.Fatalf("expected 2 spans after barrier, got %d", len(found))
	}
}

func TestWriterAgeFlush(t *testing.T) {
	setupTestDB(t)
	startTestWriters(t, duckdbrepo.WriterOptions{FlushRows: 1 << 20, FlushInterval: 20 * time.Millisecond})

	ctx := context.Background()
	projectId := uuid.New()
	traceId := uuid.New()
	now := truncateMs(time.Now().UTC())

	err := SpanRepository.InsertAsync(ctx, []models.Span{
		makeSpan(projectId, traceId, "db.query", now, 100*time.Millisecond),
	})
	if err != nil {
		t.Fatalf("InsertAsync failed: %v", err)
	}

	deadline := time.Now().Add(5 * time.Second)
	for {
		found, err := SpanRepository.FindByTraceId(ctx, projectId, traceId, nil)
		if err != nil {
			t.Fatalf("FindByTraceId failed: %v", err)
		}
		if len(found) == 1 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("span not visible after 5s despite 20ms flush interval")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestWriterQueueFull(t *testing.T) {
	setupTestDB(t)
	// Negative QueueWait disables the bounded wait so the rejection is
	// deterministic and immediate.
	startTestWriters(t, duckdbrepo.WriterOptions{QueueRows: 10, FlushRows: 1 << 20, FlushInterval: time.Hour, QueueWait: -1})

	ctx := context.Background()
	projectId := uuid.New()
	traceId := uuid.New()
	now := truncateMs(time.Now().UTC())

	spans := func(n int) []models.Span {
		out := make([]models.Span, 0, n)
		for i := 0; i < n; i++ {
			out = append(out, makeSpan(projectId, traceId, "s", now.Add(time.Duration(i)*time.Millisecond), time.Millisecond))
		}
		return out
	}

	if err := SpanRepository.InsertAsync(ctx, spans(8)); err != nil {
		t.Fatalf("expected first batch to be admitted, got %v", err)
	}
	err := SpanRepository.InsertAsync(ctx, spans(5))
	if !errors.Is(err, db.ErrIngestQueueFull) {
		t.Fatalf("expected ErrIngestQueueFull, got %v", err)
	}

	// Draining the queue frees the budget; nothing admitted was lost.
	if err := duckdbrepo.FlushWriters(ctx); err != nil {
		t.Fatalf("FlushWriters failed: %v", err)
	}
	if err := SpanRepository.InsertAsync(ctx, spans(5)); err != nil {
		t.Fatalf("expected insert after drain to succeed, got %v", err)
	}
	if err := duckdbrepo.FlushWriters(ctx); err != nil {
		t.Fatalf("FlushWriters failed: %v", err)
	}

	found, err := SpanRepository.FindByTraceId(ctx, projectId, traceId, nil)
	if err != nil {
		t.Fatalf("FindByTraceId failed: %v", err)
	}
	if len(found) != 13 {
		t.Fatalf("expected 13 spans (8 admitted + 5 after drain), got %d", len(found))
	}
}

// Pins the graceful-shutdown guarantee: rows sitting in the queue when
// StopWriters runs are drained and flushed, not dropped — a clean restart
// must not lose acked rows.
func TestWriterStopDrainsQueue(t *testing.T) {
	setupTestDB(t)
	// Huge thresholds so neither size nor age can flush; only the shutdown
	// drain can persist the rows. Several batches so every shard has some.
	startTestWriters(t, duckdbrepo.WriterOptions{FlushRows: 1 << 20, FlushInterval: time.Hour})

	ctx := context.Background()
	projectId := uuid.New()
	traceId := uuid.New()
	now := truncateMs(time.Now().UTC())

	const batches = 20
	for i := 0; i < batches; i++ {
		err := SpanRepository.InsertAsync(ctx, []models.Span{
			makeSpan(projectId, traceId, "db.query", now.Add(time.Duration(i)*time.Millisecond), time.Millisecond),
		})
		if err != nil {
			t.Fatalf("InsertAsync failed: %v", err)
		}
	}

	duckdbrepo.StopWriters()

	found, err := SpanRepository.FindByTraceId(ctx, projectId, traceId, nil)
	if err != nil {
		t.Fatalf("FindByTraceId failed: %v", err)
	}
	if len(found) != batches {
		t.Fatalf("expected %d spans after StopWriters drain, got %d", batches, len(found))
	}
}

// Pins the synchronous fallback: with no writers started, InsertAsync must be
// immediately visible, which is what the rest of the test suite relies on.
func TestWriterFallbackWhenNotStarted(t *testing.T) {
	setupTestDB(t)

	ctx := context.Background()
	projectId := uuid.New()
	traceId := uuid.New()
	now := truncateMs(time.Now().UTC())

	err := SpanRepository.InsertAsync(ctx, []models.Span{
		makeSpan(projectId, traceId, "db.query", now, 100*time.Millisecond),
	})
	if err != nil {
		t.Fatalf("InsertAsync failed: %v", err)
	}

	found, err := SpanRepository.FindByTraceId(ctx, projectId, traceId, nil)
	if err != nil {
		t.Fatalf("FindByTraceId failed: %v", err)
	}
	if len(found) != 1 {
		t.Fatalf("expected immediate visibility without writers, got %d spans", len(found))
	}
}
