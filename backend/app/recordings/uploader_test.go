package recordings

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/models"
)

type stubStorage struct {
	delay   time.Duration
	written atomic.Uint64
}

func (s *stubStorage) Delete(_ context.Context, _ string) error { return nil }

func (s *stubStorage) Write(ctx context.Context, key string, data []byte) error {
	if s.delay > 0 {
		select {
		case <-time.After(s.delay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	s.written.Add(1)
	return nil
}

func makeJob() Job {
	return Job{
		Id:           uuid.New(),
		ProjectId:    uuid.New(),
		Key:          "recordings/test/" + uuid.NewString() + ".json",
		Body:         []byte("{}"),
		RecordedAt:   time.Now().UTC(),
		SegmentIndex: 0,
	}
}

// TestEnqueue_NeverBlocks_AndDropsAccountedFor exercises the backpressure
// path: the channel must never block enqueue, and uploaded+dropped+failed
// must always equal the number of submitted jobs. The batcher is wired up
// to the real repository singleton so this test only runs as far as the
// successful storage write — it discards the inserts channel via cancel.
func TestEnqueue_NeverBlocks_AndDropsAccountedFor(t *testing.T) {
	storage := &stubStorage{delay: 50 * time.Millisecond}

	const workers = 2
	const queue = 4
	const total = 100

	p := &pool{
		workers: workers,
		jobs:    make(chan Job, queue),
		inserts: make(chan models.SessionRecording, insertBatchSize),
		storage: storage,
	}
	ctx, cancel := context.WithCancel(context.Background())
	for i := 0; i < workers; i++ {
		go p.worker(ctx)
	}
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-p.inserts:
			}
		}
	}()
	defer cancel()

	start := time.Now()
	for i := 0; i < total; i++ {
		p.enqueue(makeJob())
	}
	enqueueElapsed := time.Since(start)

	if enqueueElapsed > 50*time.Millisecond {
		t.Fatalf("Enqueue blocked: %d calls took %v (expected < 50ms)", total, enqueueElapsed)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if p.uploaded.Load()+p.dropped.Load() >= total {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	uploaded := p.uploaded.Load()
	dropped := p.dropped.Load()
	failed := p.failed.Load()

	if uploaded+dropped+failed != total {
		t.Fatalf("expected uploaded+dropped+failed == %d, got uploaded=%d dropped=%d failed=%d",
			total, uploaded, dropped, failed)
	}
	if dropped == 0 {
		t.Fatalf("expected drops with workers=%d queue=%d under slow storage, got dropped=0", workers, queue)
	}
	if uploaded == 0 {
		t.Fatalf("expected at least some uploads to succeed, got uploaded=0")
	}

	if p.inFlight.Load() != 0 {
		deadline = time.Now().Add(time.Second)
		for time.Now().Before(deadline) {
			if p.inFlight.Load() == 0 {
				break
			}
			time.Sleep(20 * time.Millisecond)
		}
	}
	if p.inFlight.Load() != 0 {
		t.Errorf("expected inFlight=0 after drain, got %d", p.inFlight.Load())
	}
}

func TestEnqueue_NoSingleton_NoOps(t *testing.T) {
	saved := singleton
	singleton = nil
	defer func() { singleton = saved }()

	Enqueue(makeJob())
}

