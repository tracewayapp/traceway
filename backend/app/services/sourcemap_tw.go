package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tracewayapp/traceway/backend/v2/app/storage"
	"github.com/tracewayapp/traceway/backend/v2/app/traceway"

	"github.com/google/uuid"
)

// The queue holds keys, not file contents, so its depth costs nothing: what
// bounds the work is twWorkers. It has to fit whole uploads, since one build
// of a code-split app is a few hundred jobs (a job per bundle plus one per
// debug id alias) and they are all enqueued at once.
const (
	twWorkers    = 2
	twQueueDepth = 2048
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
	twGenerating  atomic.Int32
	twDropped     atomic.Uint64
	twRun         = warmTWArtifact

	// Guarded by twPendingMu.
	twDropReportedAt time.Time
)

const twDropReportInterval = time.Minute

// SourceMapWarmupBacklog is the number of warm-ups not finished yet: uploads
// still being enqueued, queued jobs and running jobs.
func SourceMapWarmupBacklog() int {
	twPendingMu.Lock()
	queued := len(twPending)
	twPendingMu.Unlock()
	return int(twGenerating.Load()) + queued + int(twRunning.Load())
}

func GenerateTWArtifactsAsync(ctx context.Context, projectId uuid.UUID, fileNames []string) {
	ctx = context.WithoutCancel(ctx)
	twGenerating.Add(1)
	go func() {
		defer twGenerating.Add(-1)
		defer traceway.Recover()
		GenerateTWArtifacts(ctx, projectId, fileNames)
	}()
}

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
		reportTWDrop(job)
		return false
	}
}

func reportTWDrop(job twJob) {
	now := time.Now()
	if now.Sub(twDropReportedAt) < twDropReportInterval {
		return
	}
	twDropReportedAt = now
	traceway.CaptureException(errors.New(twDropMessage(job)))
}

// No file name in the message: it is hashed into the issue, and a name per
// drop would open a new issue (and a new-error notification) every time.
func twDropMessage(job twJob) string {
	return fmt.Sprintf("tw generation: warm-up queue full (%d jobs), dropped warm-ups of project %s; the artifacts will be built on first lookup", twQueueDepth, job.projectId)
}

func startTWWorkers() {
	for i := 0; i < twWorkers; i++ {
		go func() {
			for job := range twQueue {
				// Counted as running before it leaves twPending, so
				// SourceMapWarmupBacklog never reads zero in between.
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
