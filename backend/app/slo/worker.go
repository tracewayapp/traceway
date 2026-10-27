package slo

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
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
	advisoryLockId = 824737005

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

	computeWorkers  = 4
	insertBatchRows = 50_000

	bucketRetention = 120 * 24 * time.Hour
	pruneInterval   = 24 * time.Hour
)

var (
	wakeCh            = make(chan struct{}, 1)
	lastPruneAt       time.Time
	tickCount         int
	lastLateArrivalAt = map[int]time.Time{}
	lastLateCrashAt   = map[int]time.Time{}
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

func Start(ctx context.Context) {
	go func() {
		defer traceway.Recover()
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
	release, ok := acquireWorkerLock(ctx)
	if !ok {
		return
	}
	defer release()
	slos, err := db.ExecuteTransaction(func(tx *sql.Tx) ([]*models.SLO, error) {
		return transactional.SLORepository.FindEnabled(tx)
	})
	if err != nil {
		traceway.CaptureException(fmt.Errorf("failed to load enabled SLOs: %w", err))
		return
	}
	if len(slos) > 0 {
		offset := tickCount % len(slos)
		slos = append(slos[offset:], slos[:offset]...)
	}
	tickCount++
	computeAll(ctx, slos, now)
	lastTickUnix.Store(now.Unix())
	if now.Sub(lastPruneAt) >= pruneInterval {
		if err := telemetry.SLOBucketRepository.PruneBefore(ctx, now.Add(-bucketRetention)); err != nil {
			traceway.CaptureException(fmt.Errorf("failed to prune slo buckets: %w", err))
		} else {
			lastPruneAt = now
		}
	}
}

func acquireWorkerLock(ctx context.Context) (func(), bool) {
	if db.IsSQLite() {
		return func() {}, true
	}
	conn, err := db.DB.Conn(ctx)
	if err != nil {
		traceway.CaptureException(fmt.Errorf("failed to open slo worker lock connection: %w", err))
		return nil, false
	}
	var locked bool
	if err := conn.QueryRowContext(ctx, "SELECT pg_try_advisory_lock($1)", int64(advisoryLockId)).Scan(&locked); err != nil || !locked {
		if err != nil {
			traceway.CaptureException(fmt.Errorf("failed to take slo worker lock: %w", err))
		}
		conn.Close()
		return nil, false
	}
	return func() {
		if _, err := conn.ExecContext(context.Background(), "SELECT pg_advisory_unlock($1)", int64(advisoryLockId)); err != nil {
			traceway.CaptureException(fmt.Errorf("failed to release slo worker lock: %w", err))
			_ = conn.Raw(func(any) error { return driver.ErrBadConn })
		}
		conn.Close()
	}, true
}

type tickResult struct {
	slo         *models.SLO
	buckets     []shared.SLOBucketPoint
	lastClosed  time.Time
	windowStart time.Time
	historyFrom time.Time
	gapFrom     *time.Time
	gapTo       *time.Time
	backfillErr error
	err         error
}

func computeAll(ctx context.Context, slos []*models.SLO, now time.Time) {
	deadline := time.Now().Add(backfillBudget)
	horizons := recomputeHorizons(slos, now)
	results := make([]*tickResult, len(slos))
	forEach(len(slos), func(i int) {
		results[i] = computeBuckets(ctx, slos[i], now, deadline, horizons[slos[i].Id])
	})
	if err := storeBuckets(ctx, results); err != nil {
		for _, result := range results {
			if result != nil && result.err == nil {
				result.err = err
			}
		}
	}
	forEach(len(results), func(i int) {
		if results[i] != nil && results[i].err == nil {
			results[i].err = writeSnapshot(ctx, results[i], now)
		}
	})
	for i, result := range results {
		err := fmt.Errorf("panicked")
		if result != nil {
			err = result.err
		}
		if err != nil {
			atomic.AddUint64(&errorsTotal, 1)
			traceway.CaptureException(fmt.Errorf("failed to compute SLO %d (%s): %w", slos[i].Id, slos[i].SliKind, err))
			continue
		}
		atomic.AddUint64(&computedTotal, 1)
	}
}

func forEach(n int, fn func(i int)) {
	var wg sync.WaitGroup
	slots := make(chan struct{}, computeWorkers)
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

func recomputeHorizons(slos []*models.SLO, now time.Time) map[int]time.Duration {
	horizons := make(map[int]time.Duration, len(slos))
	seen := make(map[int]time.Time, len(slos))
	crashSeen := make(map[int]time.Time)
	for _, slo := range slos {
		horizons[slo.Id] = lateDataWindow
		if slo.SliKind == models.SLIKindSession {
			last, ok := lastLateCrashAt[slo.Id]
			if !ok || now.Sub(last) >= lateCrashInterval {
				horizons[slo.Id] = lateCrashHorizon
				crashSeen[slo.Id] = now
				seen[slo.Id] = now
				continue
			}
			crashSeen[slo.Id] = last
		}
		long := lateArrivalHorizon(slo.SliKind)
		if long == lateDataWindow {
			continue
		}
		last, ok := lastLateArrivalAt[slo.Id]
		if !ok || now.Sub(last) >= lateArrivalInterval {
			horizons[slo.Id] = long
			last = now
		}
		seen[slo.Id] = last
	}
	lastLateArrivalAt = seen
	lastLateCrashAt = crashSeen
	return horizons
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
	sums, err := telemetry.SLOBucketRepository.SnapshotSums(ctx, slo.ProjectId, slo.Id, result.windowStart, result.lastClosed, result.lastClosed.Add(-6*time.Hour), result.lastClosed.Add(-time.Hour))
	if err != nil {
		return err
	}
	snapshot := Snapshot(window(sums.Window), window(sums.LastHour), window(sums.LastSixHours), slo.Target(), now, &result.historyFrom, &result.lastClosed)
	snapshot.GapFrom, snapshot.GapTo = result.gapFrom, result.gapTo
	applied, err := db.ExecuteTransaction(func(tx *sql.Tx) (bool, error) {
		return transactional.SLORepository.UpdateSnapshot(tx, slo.Id, slo.Revision, snapshot)
	})
	if err != nil {
		return err
	}
	if !applied {
		return discardStale(ctx, slo)
	}
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
