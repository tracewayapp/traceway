package slo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tracewayapp/traceway/backend/app/config"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry/shared"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
	traceway "go.tracewayapp.com"
)

const (
	lateDataWindow = 10 * time.Minute

	lateArrivalInterval = 15 * time.Minute
	lateExportSlack     = time.Hour
	lateCrashInterval   = 24 * time.Hour
	lateCrashHorizon    = 72 * time.Hour

	maxCatchUp = 24 * time.Hour

	backfillChunk       = 6 * time.Hour
	metricBackfillChunk = 24 * time.Hour
	backfillBudget      = 5 * time.Second
	maxBackfillChunks   = 48

	defaultComputeWorkers = 4
	maxComputeWorkers     = 64
	defaultComputeTimeout = 30
	computeGroupSize      = 200
	insertBatchRows       = 50_000

	bucketRetention = 120 * 24 * time.Hour
	pruneInterval   = 24 * time.Hour
)

var (
	wakeCh      = make(chan struct{}, 1)
	lastPruneAt time.Time
	tickCount   int
)

func Wake() {
	select {
	case wakeCh <- struct{}{}:
	default:
	}
}

func pollInterval() time.Duration {
	return config.PollSeconds(config.Config.SLOPollSeconds, 60)
}

func computeWorkers() int {
	return boundedSetting(config.Config.SLOComputeWorkers, defaultComputeWorkers, maxComputeWorkers)
}

func computeTimeout() time.Duration {
	return time.Duration(boundedSetting(config.Config.SLOComputeTimeoutSecs, defaultComputeTimeout, 3600)) * time.Second
}

func boundedSetting(value string, fallback, max int) int {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || parsed < 1 {
		return fallback
	}
	if parsed > max {
		return max
	}
	return parsed
}

func Start(ctx context.Context) {
	go func() {
		defer traceway.Recover()
		defer releaseShards()
		ticker := time.NewTicker(pollInterval())
		defer ticker.Stop()
		RunOnce(ctx, time.Now().UTC())
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			case <-wakeCh:
			}
			RunOnce(ctx, time.Now().UTC())
		}
	}()
}

func RunOnce(ctx context.Context, now time.Time) {
	shards, err := claimShards(ctx)
	if err != nil {
		traceway.CaptureException(fmt.Errorf("failed to lease SLO shards: %w", err))
		return
	}
	if shards.size() == 0 {
		pruneWindowCaches(nil)
		return
	}
	started := time.Now()
	tickStarted.Store(started.UnixNano())
	defer tickStarted.Store(0)
	enabled, err := db.ExecuteTransaction(func(tx *sql.Tx) ([]*models.SLO, error) {
		return transactional.SLORepository.FindEnabled(tx)
	})
	if err != nil {
		traceway.CaptureException(fmt.Errorf("failed to load enabled SLOs: %w", err))
		return
	}
	slos := make([]*models.SLO, 0, len(enabled))
	keep := make(map[int]bool, len(enabled))
	for _, slo := range enabled {
		if shards.owns(slo.Id) {
			slos = append(slos, slo)
			keep[slo.Id] = true
		}
	}
	pruneWindowCaches(keep)
	if len(slos) > 0 {
		offset := tickCount % len(slos)
		slos = append(slos[offset:], slos[:offset]...)
	}
	tickCount++
	stats := computeAll(ctx, slos, now)
	stats.At = now
	stats.DurationMs = millisSince(started)
	recordTick(stats)
	lastTickUnix.Store(now.Unix())
	if shards.owns(0) && now.Sub(lastPruneAt) >= pruneInterval {
		if err := telemetry.SLOBucketRepository.PruneBefore(ctx, now.Add(-bucketRetention)); err != nil {
			traceway.CaptureException(fmt.Errorf("failed to prune slo buckets: %w", err))
		} else {
			lastPruneAt = now
		}
	}
}

type tickResult struct {
	slo           *models.SLO
	buckets       []shared.SLOBucketPoint
	lastClosed    time.Time
	windowStart   time.Time
	historyFrom   time.Time
	gapFrom       *time.Time
	gapTo         *time.Time
	earliestWrite *time.Time
	backfillErr   error
	err           error
	elapsed       time.Duration
	timedOut      bool
	fullSnapshot  bool
}

func computeAll(ctx context.Context, slos []*models.SLO, now time.Time) *TickStats {
	stats := &TickStats{SLOs: len(slos)}
	deadline := time.Now().Add(backfillBudget)
	workers, timeout := computeWorkers(), computeTimeout()
	for start := 0; start < len(slos); start += computeGroupSize {
		end := min(start+computeGroupSize, len(slos))
		computeGroup(ctx, slos[start:end], now, deadline, workers, timeout, stats)
		if end < len(slos) {
			if err := renewShards(ctx); err != nil {
				traceway.CaptureException(fmt.Errorf("failed to renew SLO shard leases: %w", err))
			}
		}
	}
	return stats
}

func computeGroup(ctx context.Context, group []*models.SLO, now, deadline time.Time, workers int, timeout time.Duration, stats *TickStats) {
	results := make([]*tickResult, len(group))
	phase := time.Now()
	forEach(len(group), workers, func(i int) {
		began := time.Now()
		sloCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		result := computeBuckets(sloCtx, group[i], now, deadline, recomputeHorizon(group[i], now))
		result.timedOut = timedOut(sloCtx, result.err) || timedOut(sloCtx, result.backfillErr)
		result.elapsed = time.Since(began)
		results[i] = result
	})
	stats.ComputeMs += millisSince(phase)

	phase = time.Now()
	if err := storeBuckets(ctx, results); err != nil {
		for _, result := range results {
			if result != nil && result.err == nil {
				result.err = err
			}
		}
	}
	stats.StoreMs += millisSince(phase)

	phase = time.Now()
	forEach(len(results), workers, func(i int) {
		result := results[i]
		if result == nil || result.err != nil {
			return
		}
		began := time.Now()
		sloCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		result.err = writeSnapshot(sloCtx, result, now)
		result.timedOut = result.timedOut || timedOut(sloCtx, result.err)
		result.elapsed += time.Since(began)
	})
	stats.SnapshotMs += millisSince(phase)

	for i, result := range results {
		if result == nil {
			atomic.AddUint64(&errorsTotal, 1)
			stats.Errors++
			traceway.CaptureException(fmt.Errorf("failed to compute SLO %d (%s): panicked", group[i].Id, group[i].SliKind))
			continue
		}
		stats.Buckets += len(result.buckets)
		if result.fullSnapshot {
			stats.FullSnapshots++
		}
		if stats.Slowest == nil || durationMs(result.elapsed) > stats.Slowest.DurationMs {
			stats.Slowest = &SlowestSLO{Id: result.slo.Id, ProjectId: result.slo.ProjectId, Kind: result.slo.SliKind, DurationMs: durationMs(result.elapsed)}
		}
		if result.timedOut {
			atomic.AddUint64(&timeoutsTotal, 1)
			stats.Timeouts++
		}
		if result.err != nil {
			atomic.AddUint64(&errorsTotal, 1)
			stats.Errors++
			if result.timedOut {
				traceway.CaptureException(fmt.Errorf("SLO %d (%s) of project %s timed out after %s: %w", result.slo.Id, result.slo.SliKind, result.slo.ProjectId, timeout, result.err))
			} else {
				traceway.CaptureException(fmt.Errorf("failed to compute SLO %d (%s): %w", result.slo.Id, result.slo.SliKind, result.err))
			}
			continue
		}
		atomic.AddUint64(&computedTotal, 1)
	}
}

func timedOut(ctx context.Context, err error) bool {
	return err != nil && (errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded))
}

func durationMs(d time.Duration) float64 {
	return float64(d) / float64(time.Millisecond)
}

func millisSince(t time.Time) float64 {
	return durationMs(time.Since(t))
}

func forEach(n, workers int, fn func(i int)) {
	var wg sync.WaitGroup
	slots := make(chan struct{}, workers)
	for i := 0; i < n; i++ {
		wg.Add(1)
		slots <- struct{}{}
		go func(i int) {
			defer wg.Done()
			defer func() { <-slots }()
			defer traceway.Recover()
			fn(i)
		}(i)
	}
	wg.Wait()
}

func lateArrivalHorizon(kind string) time.Duration {
	switch kind {
	case models.SLIKindSession:
		return shared.SessionExceptionSlack + lateArrivalInterval + lateDataWindow
	case models.SLIKindTask:
		return shared.TaskExceptionSlack + lateArrivalInterval + lateDataWindow
	}
	return lateExportSlack + lateArrivalInterval
}

func computeBuckets(ctx context.Context, slo *models.SLO, now, deadline time.Time, horizon time.Duration) *tickResult {
	result := &tickResult{slo: slo}
	if slo.LastBucketAt == nil {
		if err := telemetry.SLOBucketRepository.DeleteBySLO(ctx, slo.ProjectId, slo.Id); err != nil {
			result.err = err
			return result
		}
	}
	collect := func(from, to time.Time) error {
		points, err := ComputeBuckets(ctx, slo, from, to)
		result.buckets = append(result.buckets, points...)
		return err
	}
	bucket := slo.BucketDuration()
	lastClosed := now.Truncate(bucket)
	windowStart := lastClosed.Add(-slo.Window())
	chunk := backfillChunk
	if slo.SliKind == models.SLIKindMetric {
		chunk = metricBackfillChunk
	}

	from := lastClosed.Add(-maxCatchUp).Truncate(bucket)
	gapFrom, gapTo := copyTime(slo.GapFrom), copyTime(slo.GapTo)
	if slo.LastBucketAt != nil {
		resume := slo.LastBucketAt.UTC().Add(-horizon).Truncate(bucket)
		switch {
		case resume.After(from):
			from = resume
		case resume.Before(from):
			if gapFrom == nil || resume.Before(*gapFrom) {
				gapFrom = &resume
			}
			gapTo = copyTime(&from)
		}
	}
	for chunkFrom := from; chunkFrom.Before(lastClosed); chunkFrom = chunkFrom.Add(backfillChunk) {
		chunkTo := chunkFrom.Add(backfillChunk)
		if chunkTo.After(lastClosed) {
			chunkTo = lastClosed
		}
		if err := collect(chunkFrom, chunkTo); err != nil {
			result.err = err
			return result
		}
	}

	if gapFrom != nil && gapFrom.Before(windowStart) {
		gapFrom = copyTime(&windowStart)
	}
	for chunks := 0; gapFrom != nil && gapTo != nil && gapTo.After(*gapFrom) && chunks < maxBackfillChunks && time.Now().Before(deadline); chunks++ {
		chunkFrom := gapTo.Add(-chunk)
		if chunkFrom.Before(*gapFrom) {
			chunkFrom = *gapFrom
		}
		if result.backfillErr = collect(chunkFrom, *gapTo); result.backfillErr != nil {
			break
		}
		gapTo = &chunkFrom
	}
	if gapFrom == nil || gapTo == nil || !gapTo.After(*gapFrom) {
		gapFrom, gapTo = nil, nil
	}

	historyFrom := from
	if slo.HistoryFrom != nil && slo.HistoryFrom.Before(historyFrom) {
		historyFrom = slo.HistoryFrom.UTC()
	}
	for chunks := 0; result.backfillErr == nil && historyFrom.After(windowStart) && chunks < maxBackfillChunks && time.Now().Before(deadline); chunks++ {
		chunkFrom := historyFrom.Add(-chunk)
		if chunkFrom.Before(windowStart) {
			chunkFrom = windowStart
		}
		if result.backfillErr = collect(chunkFrom, historyFrom); result.backfillErr != nil {
			break
		}
		historyFrom = chunkFrom
	}

	result.lastClosed, result.windowStart, result.historyFrom = lastClosed, windowStart, historyFrom
	result.gapFrom, result.gapTo = gapFrom, gapTo
	return result
}

func storeBuckets(ctx context.Context, results []*tickResult) error {
	batch := make([]shared.SLOBucket, 0, insertBatchRows)
	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		stamp := time.Now().UTC()
		for i := range batch {
			batch[i].ComputedAt = stamp
		}
		err := telemetry.SLOBucketRepository.Upsert(ctx, batch)
		batch = batch[:0]
		return err
	}
	for _, result := range results {
		if result == nil || result.err != nil {
			continue
		}
		for _, point := range result.buckets {
			if result.earliestWrite == nil || point.Bucket.Before(*result.earliestWrite) {
				at := point.Bucket
				result.earliestWrite = &at
			}
			batch = append(batch, shared.SLOBucket{ProjectId: result.slo.ProjectId, SLOId: result.slo.Id, BucketStart: point.Bucket, Good: point.Good, Total: point.Total})
			if len(batch) == insertBatchRows {
				if err := flush(); err != nil {
					return err
				}
			}
		}
	}
	return flush()
}

func writeSnapshot(ctx context.Context, result *tickResult, now time.Time) error {
	slo := result.slo
	sums, err := readWindowSums(ctx, result, now)
	if err != nil {
		storeWindowCache(slo.Id, nil)
		return err
	}
	result.fullSnapshot = sums.full
	snapshot := Snapshot(sums.window, sums.lastHour, sums.lastSixHours, slo.Target(), now, &result.historyFrom, &result.lastClosed)
	snapshot.GapFrom, snapshot.GapTo = result.gapFrom, result.gapTo
	applied, err := db.ExecuteTransaction(func(tx *sql.Tx) (bool, error) {
		return transactional.SLORepository.UpdateSnapshot(tx, slo.Id, slo.Revision, snapshot)
	})
	if err != nil {
		storeWindowCache(slo.Id, nil)
		return err
	}
	if !applied {
		storeWindowCache(slo.Id, nil)
		return discardStale(ctx, slo)
	}
	storeWindowCache(slo.Id, sums.cache)
	return result.backfillErr
}

func window(count shared.SLOCount) Window {
	return Window{Good: count.Good, Total: count.Total}
}

func discardStale(ctx context.Context, stale *models.SLO) error {
	current, err := db.ExecuteTransaction(func(tx *sql.Tx) (*models.SLO, error) {
		return transactional.SLORepository.FindById(tx, stale.Id)
	})
	if err != nil || current != nil {
		return err
	}
	return telemetry.SLOBucketRepository.DeleteBySLO(ctx, stale.ProjectId, stale.Id)
}

func copyTime(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	value := t.UTC()
	return &value
}

func sumRange(ctx context.Context, slo *models.SLO, from, to time.Time) (Window, error) {
	good, total, err := telemetry.SLOBucketRepository.SumRange(ctx, slo.ProjectId, slo.Id, from, to)
	return Window{Good: good, Total: total}, err
}

func WindowSums(ctx context.Context, slo *models.SLO, length time.Duration, now time.Time) (Window, error) {
	to := now.Truncate(slo.BucketDuration())
	return sumRange(ctx, slo, to.Add(-length), to)
}
