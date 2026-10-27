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
	advisoryLockId = 824737005 // synthetics: 824737004, outbox: 824737003, escalator: 824737002, backfill: 824737001

	// lateDataWindow is how far behind the newest computed bucket every tick
	// recomputes, so a span or exception that arrives a few minutes late still
	// lands in its bucket.
	lateDataWindow = 10 * time.Minute

	// lateArrivalInterval is how often the longer horizon of lateArrivalHorizon
	// is recomputed; doing it every tick would multiply the reads of those kinds.
	lateArrivalInterval = 15 * time.Minute

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

	computeWorkers = 4
	// insertBatchRows caps one bucket insert. A tick writes all its SLOs in as
	// few inserts as this allows: ClickHouse makes a part of every insert.
	insertBatchRows = 50_000

	bucketRetention = 120 * 24 * time.Hour
	pruneInterval   = 24 * time.Hour
)

var (
	wakeCh            = make(chan struct{}, 1)
	lastPruneAt       time.Time
	tickCount         int
	lastLateArrivalAt = map[int]time.Time{}
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

// computeAll runs one tick over slos in three phases: read every SLO's
// buckets from telemetry, write them all in as few inserts as possible, then
// write each snapshot. The order of slos decides who gets the backfill budget first.
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

// lateArrivalHorizon is how long the buckets of a kind keep changing. A
// session or a task is counted at its start and turns bad when its exception
// lands, up to the slack its SLI query allows; a recompute every
// lateArrivalInterval must still cover the end of that slack.
func lateArrivalHorizon(kind string) time.Duration {
	switch kind {
	case models.SLIKindSession:
		return shared.SessionExceptionSlack + lateArrivalInterval + lateDataWindow
	case models.SLIKindTask:
		return shared.TaskExceptionSlack + lateArrivalInterval + lateDataWindow
	}
	return lateDataWindow
}

// recomputeHorizons picks how far back each SLO recomputes this tick: the
// late-arrival horizon of its kind once every lateArrivalInterval, otherwise
// lateDataWindow.
func recomputeHorizons(slos []*models.SLO, now time.Time) map[int]time.Duration {
	horizons := make(map[int]time.Duration, len(slos))
	seen := make(map[int]time.Time, len(slos))
	for _, slo := range slos {
		horizons[slo.Id] = lateDataWindow
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
	return horizons
}

// computeBuckets reads the buckets this tick refreshes for one SLO: the
// recent ones, a gap left by an outage and older history within the budget.
func computeBuckets(ctx context.Context, slo *models.SLO, now, deadline time.Time, horizon time.Duration) *tickResult {
	result := &tickResult{slo: slo}
	if slo.LastBucketAt == nil {
		// A new SLO, or one whose definition changed: the old buckets are
		// deleted before any new one is written, so a delete can never land
		// after the buckets of the new definition.
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

	// Backfill failures do not hold back the snapshot of what was computed:
	// they are reported after it, and the next tick resumes where this one stopped.
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

// storeBuckets writes the buckets of every computed SLO. Rows are stamped at
// write time, after any delete of the same tick, so the newest computation
// always wins on ClickHouse.
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

// discardStale runs when the SLO was edited or deleted while this tick
// computed it. An edit that changed the definition reset the history, so the
// next tick deletes these buckets before it writes new ones; an edit of the
// target or the window leaves them valid. Only a deleted SLO is cleaned up
// here, since its own delete may have run before this tick wrote.
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

// WindowSums returns the good/total counts of an SLO over [to - length, to),
// read from the stored buckets. The notification rules evaluate from it.
func WindowSums(ctx context.Context, slo *models.SLO, length time.Duration, now time.Time) (Window, error) {
	to := now.Truncate(slo.BucketDuration())
	return sumRange(ctx, slo, to.Add(-length), to)
}
