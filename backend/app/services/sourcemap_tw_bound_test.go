package services

import (
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/v2/app/traceway"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func awaitTWWarmups(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		twPendingMu.Lock()
		pending := len(twPending)
		twPendingMu.Unlock()
		if pending == 0 && len(twQueue) == 0 && twRunning.Load() == 0 {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("source-map warm-up queue did not drain")
}

func TestTWWarmupQueueIsBoundedAndCoalesced(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	t.Cleanup(traceway.Install(traceway.Providers{
		TracerProvider: sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder)),
	}))
	twPendingMu.Lock()
	twDropReportedAt = time.Time{}
	twPendingMu.Unlock()
	droppedBefore := twDropped.Load()

	var live, peak int64
	gate := make(chan struct{})
	started := make(chan twJob, twWorkers+twQueueDepth+8)

	prev := twRun
	twRun = func(job twJob) {
		n := atomic.AddInt64(&live, 1)
		for {
			old := atomic.LoadInt64(&peak)
			if n <= old || atomic.CompareAndSwapInt64(&peak, old, n) {
				break
			}
		}
		started <- job
		<-gate
		atomic.AddInt64(&live, -1)
	}
	t.Cleanup(func() { twRun = prev })
	defer func() {
		close(gate)
		awaitTWWarmups(t)
	}()

	project := uuid.New()
	job := func(i int) twJob { return twJob{projectId: project, base: fmt.Sprintf("bundle-%d", i)} }

	for i := 0; i < twWorkers; i++ {
		if !enqueueTWWarmup(job(i)) {
			t.Fatalf("job %d was not accepted on an idle pool", i)
		}
	}
	for i := 0; i < twWorkers; i++ {
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("workers did not start")
		}
	}
	for i := twWorkers; i < twWorkers+twQueueDepth; i++ {
		if !enqueueTWWarmup(job(i)) {
			t.Fatalf("job %d was dropped with room left in the queue", i)
		}
	}

	done := make(chan bool, 1)
	go func() { done <- enqueueTWWarmup(job(10_000)) }()
	select {
	case queued := <-done:
		if queued {
			t.Fatal("a job past the queue depth was accepted")
		}
	case <-time.After(time.Second):
		t.Fatal("enqueue blocked on a full queue")
	}
	if enqueueTWWarmup(job(10_001)) {
		t.Fatal("a job past the queue depth was accepted")
	}
	if dropped := twDropped.Load() - droppedBefore; dropped != 2 {
		t.Fatalf("counted %d dropped warm-ups, want 2", dropped)
	}
	if reports := recorder.Ended(); len(reports) != 1 || !strings.Contains(reports[0].Status().Description, "warm-up queue full") {
		t.Fatalf("a burst of drops must be reported once, got %d reports", len(reports))
	}

	if enqueueTWWarmup(job(twWorkers + 1)) {
		t.Fatal("a pending job was queued a second time")
	}

	if peak > int64(twWorkers) {
		t.Fatalf("peak concurrent warm-ups %d exceeded the %d worker bound", peak, twWorkers)
	}
}
