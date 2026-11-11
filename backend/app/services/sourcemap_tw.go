package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/tracewayapp/traceway/backend/app/storage"
	"github.com/tracewayapp/traceway/backend/app/traceway"

	"github.com/google/uuid"
)

const (
	twWorkers    = 2
	twQueueDepth = 64
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
	twDropped     atomic.Uint64
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
		twDropped.Add(1)
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
	ctx := context.Background()

	mapKey := SourceMapStorageKey(job.projectId, job.base+".map")
	bundleKey := SourceMapStorageKey(job.projectId, job.base)
	_, done, err := sharedCache.Get(ctx, twKeyFor(mapKey), loadSourceMapBlob(mapKey, bundleKey))
	if err != nil && !errors.Is(err, storage.ErrNotFound) {
		traceway.CaptureException(fmt.Errorf("tw generation: failed to warm resolver (key=%s): %w", mapKey, err))
	}
	done()
}
