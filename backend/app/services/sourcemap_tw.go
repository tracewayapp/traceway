package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tracewayapp/traceway/backend/app/storage"

	"github.com/google/uuid"
	traceway "go.tracewayapp.com"
)

const (
	twWorkers    = 2
	twQueueDepth = 64
	twJobTimeout = 2 * time.Minute
)

type twJob struct {
	projectId uuid.UUID
	base      string
}

var (
	twQueue       = make(chan twJob, twQueueDepth)
	twWorkersOnce sync.Once
	twPendingMu   sync.Mutex
	twPending     = map[twJob]struct{}{}
	twRunning     atomic.Int32
	twDroppedAt   time.Time
	twRun         = warmTWArtifact
)

func GenerateTWArtifacts(ctx context.Context, projectId uuid.UUID, fileNames []string) {
	ctx = context.WithoutCancel(ctx)

	bases := make(map[string]bool)
	for _, name := range fileNames {
		bases[strings.TrimSuffix(name, ".map")] = true
	}

	for base := range bases {
		twKey := twKeyFor(SourceMapStorageKey(projectId, base+".map"))
		if err := storage.Store.Delete(ctx, twKey); err != nil {
			traceway.CaptureException(fmt.Errorf("tw generation: failed to delete stale tw artifact (key=%s): %w", twKey, err))
		}
		InvalidateSourceMap(projectId, base)

		enqueueTWWarmup(twJob{projectId: projectId, base: base})
	}
}

func enqueueTWWarmup(job twJob) bool {
	twWorkersOnce.Do(startTWWorkers)

	twPendingMu.Lock()
	defer twPendingMu.Unlock()
	if _, pending := twPending[job]; pending {
		return false
	}
	select {
	case twQueue <- job:
		twPending[job] = struct{}{}
		return true
	default:
		reportTWDrop(job)
		return false
	}
}

func startTWWorkers() {
	for i := 0; i < twWorkers; i++ {
		go func() {
			for job := range twQueue {
				twRunning.Add(1)
				twPendingMu.Lock()
				delete(twPending, job)
				twPendingMu.Unlock()
				func() {
					defer twRunning.Add(-1)
					defer traceway.Recover()
					twRun(job)
				}()
			}
		}()
	}
}

func warmTWArtifact(job twJob) {
	ctx, cancel := context.WithTimeout(context.Background(), twJobTimeout)
	defer cancel()

	mapKey := SourceMapStorageKey(job.projectId, job.base+".map")
	bundleKey := SourceMapStorageKey(job.projectId, job.base)
	_, done, err := sharedCache.Get(ctx, twKeyFor(mapKey), loadSourceMapBlob(mapKey, bundleKey))
	if err != nil && !errors.Is(err, storage.ErrNotFound) {
		traceway.CaptureException(fmt.Errorf("tw generation: failed to warm resolver (key=%s): %w", mapKey, err))
	}
	done()
}

func reportTWDrop(job twJob) {
	now := time.Now()
	if now.Sub(twDroppedAt) < time.Minute {
		return
	}
	twDroppedAt = now
	traceway.CaptureException(fmt.Errorf("tw generation: warm-up queue full (%d jobs), dropped %s/%s; the artifact will be built on first lookup", twQueueDepth, job.projectId, job.base))
}
