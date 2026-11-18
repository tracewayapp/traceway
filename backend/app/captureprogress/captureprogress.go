// Package captureprogress records when a capture reached a client, was applied there and sent its first data.
//
// Each stage is a first-time fact, so it is written once per process and never on the request path: an OpAMP poll
// or an ingest batch only notes the stage in memory, and one goroutine writes what has been noted. On the
// single-connection SQLite main DB a write there would otherwise hold an admission slot while it waits for the
// connection, on every poll and batch of every client a broad capture matches.
package captureprogress

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/v2/app/db"
	"github.com/tracewayapp/traceway/backend/v2/app/repositories/transactional"
	"github.com/tracewayapp/traceway/backend/v2/app/traceway"
)

const (
	Delivered = "delivered"
	Applied   = "applied"
	Received  = "received"

	// maxPending bounds what waits for the writer; a stage noted while it is full is noted again by a later poll or batch.
	maxPending = 1024
	// maxRecorded bounds memory when clients send capture ids that do not exist; past it the set starts over, which
	// costs one more no-op update per stage still being reported.
	maxRecorded = 16384
	// recordedTTL outlives the longest capture (24 hours).
	recordedTTL       = 25 * time.Hour
	pruneInterval     = time.Hour
	errReportInterval = time.Minute
)

// errWriteAborted stands for a write that panicked; the panic itself is reported by the task.
var errWriteAborted = errors.New("capture progress write did not return")

type key struct {
	projectId uuid.UUID
	captureId uuid.UUID
	stage     string
}

type recorder struct {
	mu        sync.Mutex
	pending   map[key]time.Time
	recorded  map[key]time.Time
	lastPrune time.Time
	lastError time.Time

	wake  chan struct{}
	start sync.Once
	write func(ctx context.Context, batch map[key]time.Time) error
	now   func() time.Time
}

func newRecorder(write func(ctx context.Context, batch map[key]time.Time) error) *recorder {
	return &recorder{
		pending:  map[key]time.Time{},
		recorded: map[key]time.Time{},
		wake:     make(chan struct{}, 1),
		write:    write,
		now:      time.Now,
	}
}

var progress = newRecorder(writeBatch)

// Mark notes that a client reached stage of a capture and returns at once. The write happens in the background,
// stamped with the time of this call; a stage this process already wrote is ignored.
func Mark(projectId, captureId uuid.UUID, stage string) {
	progress.mark(projectId, captureId, stage)
}

func (r *recorder) mark(projectId, captureId uuid.UUID, stage string) {
	k := key{projectId: projectId, captureId: captureId, stage: stage}
	r.mu.Lock()
	_, done := r.recorded[k]
	_, queued := r.pending[k]
	if done || queued || len(r.pending) >= maxPending {
		r.mu.Unlock()
		return
	}
	r.pending[k] = r.now().UTC()
	r.mu.Unlock()

	r.start.Do(func() { go r.run() })
	select {
	case r.wake <- struct{}{}:
	default:
	}
}

// run must outlive every batch: start never launches a second writer, so a dead loop would drop every later mark.
func (r *recorder) run() {
	for range r.wake {
		r.record()
	}
}

// record writes one batch. A panic ends that batch only: RunTask reports it on the task and Recover stops it here.
func (r *recorder) record() {
	defer traceway.Recover()
	traceway.RunTask(context.Background(), "captureprogress.record", r.drain)
}

// drain writes everything pending in one transaction. Entries stay pending while they are written, so a stage
// noted again meanwhile is not queued twice; a failed batch, or one whose write panicked, is dropped and comes
// back with the next report.
func (r *recorder) drain(ctx context.Context) {
	r.mu.Lock()
	batch := make(map[key]time.Time, len(r.pending))
	for k, at := range r.pending {
		batch[k] = at
	}
	r.mu.Unlock()
	if len(batch) == 0 {
		traceway.SkipTask(ctx)
		return
	}

	err := errWriteAborted
	defer func() { r.settle(ctx, batch, err) }()
	err = r.write(ctx, batch)
}

func (r *recorder) settle(ctx context.Context, batch map[key]time.Time, err error) {
	now := r.now()
	r.mu.Lock()
	for k := range batch {
		delete(r.pending, k)
		if err == nil {
			r.recorded[k] = now
		}
	}
	if now.Sub(r.lastPrune) >= pruneInterval {
		for k, at := range r.recorded {
			if now.Sub(at) > recordedTTL {
				delete(r.recorded, k)
			}
		}
		r.lastPrune = now
	}
	if len(r.recorded) > maxRecorded {
		r.recorded = map[key]time.Time{}
	}
	report := err != nil && !errors.Is(err, errWriteAborted) && now.Sub(r.lastError) >= errReportInterval
	if report {
		r.lastError = now
	}
	r.mu.Unlock()

	if report {
		traceway.CaptureExceptionContext(ctx, fmt.Errorf("recording progress of %d capture stages: %w", len(batch), err))
	}
}

// writeBatch relies on MarkCaptureProgress only setting a stage that is still empty: an update that finds it set,
// or finds no such capture, changes nothing, and either way the stage needs no further write.
func writeBatch(ctx context.Context, batch map[key]time.Time) error {
	_, err := db.ExecuteTransactionContext(ctx, func(tx *sql.Tx) (struct{}, error) {
		for k, at := range batch {
			if err := transactional.SdkConfigRepository.MarkCaptureProgress(tx, k.projectId, k.captureId, k.stage, at); err != nil {
				return struct{}{}, err
			}
		}
		return struct{}{}, nil
	})
	return err
}
