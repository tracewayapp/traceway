package slo

import (
	"context"
	"database/sql"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
)

const staleAfter = 5 * time.Minute

var (
	computedTotal uint64
	errorsTotal   uint64
	timeoutsTotal uint64
	lastTickUnix  atomic.Int64
	tickStarted   atomic.Int64

	lastTickMu sync.Mutex
	lastTick   *TickStats
)

type SlowestSLO struct {
	Id         int       `json:"id"`
	ProjectId  uuid.UUID `json:"projectId"`
	Kind       string    `json:"kind"`
	DurationMs float64   `json:"durationMs"`
}

type TickStats struct {
	At            time.Time   `json:"at"`
	DurationMs    float64     `json:"durationMs"`
	ComputeMs     float64     `json:"computeMs"`
	StoreMs       float64     `json:"storeMs"`
	SnapshotMs    float64     `json:"snapshotMs"`
	SLOs          int         `json:"slos"`
	Errors        int         `json:"errors"`
	Timeouts      int         `json:"timeouts"`
	FullSnapshots int         `json:"fullSnapshots"`
	Buckets       int         `json:"buckets"`
	Slowest       *SlowestSLO `json:"slowest"`
}

type Freshness struct {
	Enabled         int     `json:"enabled"`
	Stale           int     `json:"stale"`
	MaxStalenessSec float64 `json:"maxStalenessSec"`
	BackfillPending int     `json:"backfillPending"`
}

type HealthStats struct {
	ComputedTotal uint64     `json:"computedTotal"`
	ErrorsTotal   uint64     `json:"errorsTotal"`
	TimeoutsTotal uint64     `json:"timeoutsTotal"`
	LastTickAt    *time.Time `json:"lastTickAt"`
	RunningForSec float64    `json:"runningForSec"`
	OwnedShards   int        `json:"ownedShards"`
	Instances     int        `json:"instances"`
	LastTick      *TickStats `json:"lastTick"`
	Freshness     *Freshness `json:"freshness"`
}

func HealthSnapshot(ctx context.Context) (*HealthStats, error) {
	stats := &HealthStats{
		ComputedTotal: atomic.LoadUint64(&computedTotal),
		ErrorsTotal:   atomic.LoadUint64(&errorsTotal),
		TimeoutsTotal: atomic.LoadUint64(&timeoutsTotal),
		OwnedShards:   int(ownedShards.Load()),
		Instances:     int(knownInstances.Load()),
	}
	if unix := lastTickUnix.Load(); unix > 0 {
		at := time.Unix(unix, 0).UTC()
		stats.LastTickAt = &at
	}
	if started := tickStarted.Load(); started > 0 {
		stats.RunningForSec = time.Since(time.Unix(0, started)).Seconds()
	}
	lastTickMu.Lock()
	if lastTick != nil {
		copied := *lastTick
		stats.LastTick = &copied
	}
	lastTickMu.Unlock()
	freshness, err := ReadFreshness(ctx, time.Now().UTC())
	if err != nil {
		return stats, err
	}
	stats.Freshness = freshness
	return stats, nil
}

// ReadFreshness reads every enabled SLO's last computation from the main
// database, so it covers all instances and also catches a worker that stopped.
func ReadFreshness(ctx context.Context, now time.Time) (*Freshness, error) {
	rows, err := db.ExecuteTransaction(func(tx *sql.Tx) ([]*models.SLOFreshness, error) {
		return transactional.SLORepository.FindEnabledFreshness(tx)
	})
	if err != nil {
		return nil, err
	}
	freshness := &Freshness{Enabled: len(rows)}
	for _, row := range rows {
		last := row.CreatedAt
		if row.ComputedAt != nil {
			last = *row.ComputedAt
		}
		staleness := now.Sub(last).Seconds()
		if staleness < 0 {
			staleness = 0
		}
		if staleness > freshness.MaxStalenessSec {
			freshness.MaxStalenessSec = staleness
		}
		if row.ComputedAt == nil || now.Sub(*row.ComputedAt) > staleAfter {
			freshness.Stale++
		}
		if backfillPending(row) {
			freshness.BackfillPending++
		}
	}
	return freshness, nil
}

func backfillPending(row *models.SLOFreshness) bool {
	if row.GapFrom != nil || row.HistoryFrom == nil || row.LastBucketAt == nil {
		return true
	}
	return row.HistoryFrom.After(row.LastBucketAt.Add(-time.Duration(row.WindowDays) * 24 * time.Hour))
}

func recordTick(stats *TickStats) {
	lastTickMu.Lock()
	lastTick = stats
	lastTickMu.Unlock()
}
