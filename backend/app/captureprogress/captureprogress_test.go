package captureprogress

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

type fakeWriter struct {
	batches []map[key]time.Time
	fail    bool
}

func (w *fakeWriter) write(_ context.Context, batch map[key]time.Time) error {
	w.batches = append(w.batches, batch)
	if w.fail {
		return errors.New("database is locked")
	}
	return nil
}

// newTestRecorder never starts the worker goroutine, so each test drains by hand.
func newTestRecorder(w *fakeWriter) *recorder {
	r := newRecorder(w.write)
	r.start.Do(func() {})
	return r
}

func TestEachStageIsWrittenOnce(t *testing.T) {
	w := &fakeWriter{}
	r := newTestRecorder(w)
	project, capture := uuid.New(), uuid.New()

	for range 50 {
		r.mark(project, capture, Delivered)
	}
	r.mark(project, capture, Applied)
	r.mark(uuid.New(), capture, Delivered)
	r.drain(context.Background())

	if len(w.batches) != 1 || len(w.batches[0]) != 3 {
		t.Fatalf("want one batch of 3 stages, got %v", w.batches)
	}
	for range 50 {
		r.mark(project, capture, Delivered)
		r.mark(project, capture, Applied)
	}
	r.drain(context.Background())
	if len(w.batches) != 1 {
		t.Fatalf("a written stage was written again: %v", w.batches)
	}
}

func TestStageNotedDuringAWriteIsNotQueuedAgain(t *testing.T) {
	project, capture := uuid.New(), uuid.New()
	var r *recorder
	writes := 0
	r = newRecorder(func(context.Context, map[key]time.Time) error {
		writes++
		r.mark(project, capture, Received)
		return nil
	})
	r.start.Do(func() {})

	r.mark(project, capture, Received)
	r.drain(context.Background())
	r.drain(context.Background())
	if writes != 1 {
		t.Fatalf("want 1 write, got %d", writes)
	}
}

func TestFailedWriteIsRetriedByTheNextReport(t *testing.T) {
	w := &fakeWriter{fail: true}
	r := newTestRecorder(w)
	project, capture := uuid.New(), uuid.New()

	r.mark(project, capture, Received)
	r.drain(context.Background())
	r.drain(context.Background())
	if len(w.batches) != 1 {
		t.Fatalf("a failed batch must not stay queued: %d writes", len(w.batches))
	}

	w.fail = false
	r.mark(project, capture, Received)
	r.drain(context.Background())
	r.mark(project, capture, Received)
	r.drain(context.Background())
	if len(w.batches) != 2 {
		t.Fatalf("want the stage retried once after the failure, got %d writes", len(w.batches))
	}
}

func TestMarkIsStampedWithTheTimeItWasNoted(t *testing.T) {
	w := &fakeWriter{}
	r := newTestRecorder(w)
	noted := time.Date(2026, 11, 12, 9, 0, 0, 0, time.UTC)
	r.now = func() time.Time { return noted }
	project, capture := uuid.New(), uuid.New()

	r.mark(project, capture, Delivered)
	r.now = func() time.Time { return noted.Add(time.Minute) }
	r.drain(context.Background())
	if at := w.batches[0][key{project, capture, Delivered}]; !at.Equal(noted) {
		t.Fatalf("stamped %s, want %s", at, noted)
	}
}

func TestPendingIsBounded(t *testing.T) {
	w := &fakeWriter{}
	r := newTestRecorder(w)
	project := uuid.New()

	for range maxPending + 10 {
		r.mark(project, uuid.New(), Received)
	}
	r.drain(context.Background())
	if got := len(w.batches[0]); got != maxPending {
		t.Fatalf("want %d queued stages, got %d", maxPending, got)
	}
}

func TestRecordedStagesAgeOut(t *testing.T) {
	w := &fakeWriter{}
	r := newTestRecorder(w)
	start := time.Date(2026, 11, 12, 9, 0, 0, 0, time.UTC)
	r.now = func() time.Time { return start }
	project, capture := uuid.New(), uuid.New()

	r.mark(project, capture, Received)
	r.drain(context.Background())

	r.now = func() time.Time { return start.Add(recordedTTL + time.Hour) }
	r.mark(uuid.New(), uuid.New(), Received)
	r.drain(context.Background())
	if _, ok := r.recorded[key{project, capture, Received}]; ok {
		t.Fatalf("a stage older than %s is still remembered", recordedTTL)
	}

	for range maxRecorded + 1 {
		r.recorded[key{project, uuid.New(), Received}] = r.now()
	}
	r.mark(uuid.New(), uuid.New(), Received)
	r.drain(context.Background())
	if len(r.recorded) > maxRecorded {
		t.Fatalf("recorded grew to %d, cap is %d", len(r.recorded), maxRecorded)
	}
}
