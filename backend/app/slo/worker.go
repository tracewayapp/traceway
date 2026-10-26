package slo

import (
	"context"
	"database/sql"
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

	// maxCatchUp bounds the recompute after downtime; older buckets of the gap
	// stay empty rather than one tick scanning days of telemetry.
	maxCatchUp = 24 * time.Hour

	backfillChunk       = 6 * time.Hour
	metricBackfillChunk = 24 * time.Hour // ClickHouse reads the 1m rollups past 6h, which outlive the 7-day raw TTL
	backfillBudget      = 5 * time.Second
	maxBackfillChunks   = 48

	bucketRetention = 120 * 24 * time.Hour
	pruneInterval   = 24 * time.Hour
)

var (
	wakeCh      = make(chan struct{}, 1)
	lastPruneAt time.Time
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
	slos, err := db.ExecuteTransaction(func(tx *sql.Tx) ([]*models.SLO, error) {
		if !db.IsSQLite() {
			if _, err := tx.Exec(fmt.Sprintf("SELECT pg_advisory_xact_lock(%d)", advisoryLockId)); err != nil {
				return nil, fmt.Errorf("failed to take slo worker lock: %w", err)
			}
		}
		return transactional.SLORepository.FindEnabled(tx)
	})
	if err != nil {
		traceway.CaptureException(fmt.Errorf("failed to load enabled SLOs: %w", err))
		return
	}
	for _, slo := range slos {
		if err := ComputeOne(ctx, slo, now); err != nil {
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

// ComputeOne refreshes the recent buckets of one SLO, backfills older history
// within a time budget, and writes the snapshot.
func ComputeOne(ctx context.Context, slo *models.SLO, now time.Time) error {
	bucket := slo.BucketDuration()
	lastClosed := now.Truncate(bucket)
	windowStart := lastClosed.Add(-slo.Window())

	from := lastClosed.Add(-maxCatchUp)
	if slo.LastBucketAt != nil && slo.LastBucketAt.Add(-lateDataWindow).After(from) {
		from = slo.LastBucketAt.Add(-lateDataWindow)
	}
	from = from.Truncate(bucket)
	for chunkFrom := from; chunkFrom.Before(lastClosed); chunkFrom = chunkFrom.Add(backfillChunk) {
		chunkTo := chunkFrom.Add(backfillChunk)
		if chunkTo.After(lastClosed) {
			chunkTo = lastClosed
		}
		if err := storeBuckets(ctx, slo, chunkFrom, chunkTo, now); err != nil {
			return err
		}
	}

	historyFrom := from
	if slo.HistoryFrom != nil && slo.HistoryFrom.Before(historyFrom) {
		historyFrom = slo.HistoryFrom.UTC()
	}
	chunk := backfillChunk
	if slo.SliKind == models.SLIKindMetric {
		chunk = metricBackfillChunk
	}
	deadline := time.Now().Add(backfillBudget)
	for chunks := 0; historyFrom.After(windowStart) && chunks < maxBackfillChunks && time.Now().Before(deadline); chunks++ {
		chunkFrom := historyFrom.Add(-chunk)
		if chunkFrom.Before(windowStart) {
			chunkFrom = windowStart
		}
		if err := storeBuckets(ctx, slo, chunkFrom, historyFrom, now); err != nil {
			return err
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
	_, err = db.ExecuteTransaction(func(tx *sql.Tx) (struct{}, error) {
		return struct{}{}, transactional.SLORepository.UpdateSnapshot(tx, slo.Id, snapshot)
	})
	return err
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
