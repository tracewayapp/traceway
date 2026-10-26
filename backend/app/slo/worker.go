package slo

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
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
	advisoryLockId = 824737005 // synthetics: 824737004, outbox: 824737003, escalator: 824737002, backfill: 824737001

	// lateDataWindow is how far behind the newest computed bucket every tick
	// recomputes, so a span or exception that arrives a few minutes late still
	// lands in its bucket.
	lateDataWindow = 10 * time.Minute

	// maxCatchUp bounds the recompute a tick does before anything else. Older
	// buckets of a longer outage are recorded as a gap and filled within the
	// backfill budget.
	maxCatchUp = 24 * time.Hour

	backfillChunk       = 6 * time.Hour
	metricBackfillChunk = 24 * time.Hour // ClickHouse reads the 1m rollups past 6h, which outlive the 7-day raw TTL
	// backfillBudget is shared by all SLOs of one tick, so a burst of new SLOs
	// cannot hold back the recompute and the snapshot of every other one.
	backfillBudget    = 5 * time.Second
	maxBackfillChunks = 48

	bucketRetention = 120 * 24 * time.Hour
	pruneInterval   = 24 * time.Hour
)

var (
	wakeCh      = make(chan struct{}, 1)
	lastPruneAt time.Time
	tickCount   int
)

// Wake computes the SLOs on the next loop iteration instead of waiting for
// the tick. Call after the transaction that created or changed an SLO commits.
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

// RunOnce is one worker tick. The main-DB transaction only loads the SLOs:
// the telemetry reads run with no transaction held, because the SQLite main
// DB has a single connection. Exported so tests can drive the worker.
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
	deadline := time.Now().Add(backfillBudget)
	offset := 0
	if len(slos) > 0 {
		offset = tickCount % len(slos)
	}
	tickCount++
	for i := range slos {
		slo := slos[(offset+i)%len(slos)]
		if err := computeOne(ctx, slo, now, deadline); err != nil {
			atomic.AddUint64(&errorsTotal, 1)
			traceway.CaptureException(fmt.Errorf("failed to compute SLO %d (%s): %w", slo.Id, slo.SliKind, err))
			continue
		}
		atomic.AddUint64(&computedTotal, 1)
	}
	lastTickUnix.Store(now.Unix())
	if now.Sub(lastPruneAt) >= pruneInterval {
		if err := telemetry.SLOBucketRepository.PruneBefore(ctx, now.Add(-bucketRetention)); err != nil {
			traceway.CaptureException(fmt.Errorf("failed to prune slo buckets: %w", err))
		} else {
			lastPruneAt = now
		}
	}
}

// acquireWorkerLock lets one Postgres instance compute at a time. It is a
// session lock on a dedicated connection held for the whole tick: a
// transaction lock would be released as soon as the SLOs are loaded.
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
			// A pooled connection would keep holding the lock; drop it instead.
			_ = conn.Raw(func(any) error { return driver.ErrBadConn })
		}
		conn.Close()
	}, true
}

// ComputeOne refreshes the recent buckets of one SLO, fills gaps and older
// history within a time budget, and writes the snapshot.
func ComputeOne(ctx context.Context, slo *models.SLO, now time.Time) error {
	return computeOne(ctx, slo, now, time.Now().Add(backfillBudget))
}

func computeOne(ctx context.Context, slo *models.SLO, now time.Time, deadline time.Time) error {
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
		resume := slo.LastBucketAt.UTC().Add(-lateDataWindow).Truncate(bucket)
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
		if err := storeBuckets(ctx, slo, chunkFrom, chunkTo, now); err != nil {
			return err
		}
	}

	// Backfill failures do not hold back the snapshot of what was computed:
	// they are reported after it, and the next tick resumes where this one stopped.
	var backfillErr error
	if gapFrom != nil && gapFrom.Before(windowStart) {
		gapFrom = copyTime(&windowStart)
	}
	for chunks := 0; gapFrom != nil && gapTo != nil && gapTo.After(*gapFrom) && chunks < maxBackfillChunks && time.Now().Before(deadline); chunks++ {
		chunkFrom := gapTo.Add(-chunk)
		if chunkFrom.Before(*gapFrom) {
			chunkFrom = *gapFrom
		}
		if backfillErr = storeBuckets(ctx, slo, chunkFrom, *gapTo, now); backfillErr != nil {
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
	for chunks := 0; backfillErr == nil && historyFrom.After(windowStart) && chunks < maxBackfillChunks && time.Now().Before(deadline); chunks++ {
		chunkFrom := historyFrom.Add(-chunk)
		if chunkFrom.Before(windowStart) {
			chunkFrom = windowStart
		}
		if backfillErr = storeBuckets(ctx, slo, chunkFrom, historyFrom, now); backfillErr != nil {
			break
		}
		historyFrom = chunkFrom
	}

	window, err := sumRange(ctx, slo, windowStart, lastClosed)
	if err != nil {
		return err
	}
	lastHour, err := sumRange(ctx, slo, lastClosed.Add(-time.Hour), lastClosed)
	if err != nil {
		return err
	}
	lastSixHours, err := sumRange(ctx, slo, lastClosed.Add(-6*time.Hour), lastClosed)
	if err != nil {
		return err
	}
	snapshot := Snapshot(window, lastHour, lastSixHours, slo.Target(), now, &historyFrom, &lastClosed)
	snapshot.GapFrom, snapshot.GapTo = gapFrom, gapTo
	applied, err := db.ExecuteTransaction(func(tx *sql.Tx) (bool, error) {
		return transactional.SLORepository.UpdateSnapshot(tx, slo.Id, slo.Revision, snapshot)
	})
	if err != nil {
		return err
	}
	if !applied {
		return discardStale(ctx, slo)
	}
	return backfillErr
}

// discardStale runs when the SLO was edited or deleted while this tick
// computed it. The buckets written from the loaded definition are wrong only
// when the definition itself changed, and the edit's own bucket delete may
// have run before some of them, so they are deleted again. The edit already
// reset the history, and the worker lock keeps any other instance from having
// computed the new definition since.
func discardStale(ctx context.Context, stale *models.SLO) error {
	current, err := db.ExecuteTransaction(func(tx *sql.Tx) (*models.SLO, error) {
		return transactional.SLORepository.FindById(tx, stale.Id)
	})
	if err != nil {
		return err
	}
	if current != nil && current.SliKind == stale.SliKind && current.BucketMinutes == stale.BucketMinutes && string(current.SliConfig) == string(stale.SliConfig) {
		return nil
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

func storeBuckets(ctx context.Context, slo *models.SLO, from, to time.Time, now time.Time) error {
	points, err := ComputeBuckets(ctx, slo, from, to)
	if err != nil {
		return err
	}
	buckets := make([]shared.SLOBucket, 0, len(points))
	for _, point := range points {
		buckets = append(buckets, shared.SLOBucket{
			ProjectId:   slo.ProjectId,
			SLOId:       slo.Id,
			BucketStart: point.Bucket,
			Good:        point.Good,
			Total:       point.Total,
			ComputedAt:  now,
		})
	}
	return telemetry.SLOBucketRepository.Upsert(ctx, buckets)
}

func sumRange(ctx context.Context, slo *models.SLO, from, to time.Time) (Window, error) {
	good, total, err := telemetry.SLOBucketRepository.SumRange(ctx, slo.ProjectId, slo.Id, from, to)
	return Window{Good: good, Total: total}, err
}

// WindowSums returns the good/total counts of an SLO over [to - length, to),
// read from the stored buckets. The notification rules evaluate from it.
func WindowSums(ctx context.Context, slo *models.SLO, length time.Duration, now time.Time) (Window, error) {
	to := now.Truncate(slo.BucketDuration())
	return sumRange(ctx, slo, to.Add(-length), to)
}
