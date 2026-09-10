package agentrunner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/agentrunner/agents"
	traceway "go.tracewayapp.com"
)

// recorder batches the agent's events into the attempt's stream: a flush
// every two seconds or fifty events, and every flush renews the lease, so
// the heartbeat is the event stream itself. A lost claim cancels the run.
type recorder struct {
	attemptId uuid.UUID
	reporter  Reporter
	cancel    context.CancelFunc
	mu        sync.Mutex
	pending   []agents.Event
	lastFlush time.Time
}

func newRecorder(attemptId uuid.UUID, reporter Reporter, cancel context.CancelFunc) *recorder {
	return &recorder{attemptId: attemptId, reporter: reporter, cancel: cancel, lastFlush: time.Now()}
}

func (r *recorder) add(events ...agents.Event) {
	r.mu.Lock()
	r.pending = append(r.pending, events...)
	due := len(r.pending) >= eventFlushSize || time.Since(r.lastFlush) >= eventFlushInterval
	r.mu.Unlock()
	if due {
		r.flush()
	}
}

func (r *recorder) flush() {
	r.mu.Lock()
	batch := r.pending
	r.pending = nil
	r.lastFlush = time.Now()
	r.mu.Unlock()
	if len(batch) == 0 {
		return
	}
	err := r.reporter.Events(context.Background(), r.attemptId, batch)
	if errors.Is(err, ErrClaimLost) {
		r.cancel()
		return
	}
	if err != nil {
		traceway.CaptureException(fmt.Errorf("agentrunner: flush %d events for %s: %w", len(batch), r.attemptId, err))
	}
}

func eventPayload(event agents.Event) (map[string]any, error) {
	encoded, err := json.Marshal(event)
	if err != nil {
		return nil, err
	}
	var payload map[string]any
	if err := json.Unmarshal(encoded, &payload); err != nil {
		return nil, err
	}
	delete(payload, "schemaVersion")
	delete(payload, "kind")
	return payload, nil
}
