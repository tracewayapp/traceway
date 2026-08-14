package synthetics

import (
	"database/sql"
	"sync/atomic"
	"time"

	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
)

type HealthStats struct {
	BrowserMode        string `json:"browserMode"`
	QueuedRuns         int    `json:"queuedRuns"`
	ClaimedRuns        int    `json:"claimedRuns"`
	OldestQueuedAgeSec int64  `json:"oldestQueuedAgeSec"`
	ChecksDown         int    `json:"checksDown"`
	RunnersOnline      int    `json:"runnersOnline"`
	ExecutedTotal      uint64 `json:"executedTotal"`
	MissedTotal        uint64 `json:"missedTotal"`
}

// runnerOnlineWindow: a runner counts as online while its last poll was
// within this window (polls arrive at least every ~25s while it runs).
const runnerOnlineWindow = 2 * time.Minute

// HealthSnapshot powers /api/health/deep. OldestQueuedAgeSec measures from
// scheduled_for, so a healthy queue shows near-zero even under load.
func HealthSnapshot() (*HealthStats, error) {
	type snapshot struct {
		Counts        *models.CheckRunHealthCounts
		Oldest        *models.CheckRun
		ChecksDown    int
		RunnersOnline int
	}
	loaded, err := db.ExecuteTransaction(func(tx *sql.Tx) (snapshot, error) {
		counts, err := transactional.CheckRunRepository.CountsForHealth(tx)
		if err != nil {
			return snapshot{}, err
		}
		oldest, err := transactional.CheckRunRepository.OldestQueued(tx)
		if err != nil {
			return snapshot{}, err
		}
		down, err := transactional.SyntheticCheckRepository.CountDownAll(tx)
		if err != nil {
			return snapshot{}, err
		}
		online, err := transactional.SyntheticRunnerRepository.CountOnline(tx, time.Now().Add(-runnerOnlineWindow))
		if err != nil {
			return snapshot{}, err
		}
		return snapshot{Counts: counts, Oldest: oldest, ChecksDown: down, RunnersOnline: online}, nil
	})
	if err != nil {
		return nil, err
	}
	stats := &HealthStats{
		BrowserMode:   BrowserMode(),
		ChecksDown:    loaded.ChecksDown,
		RunnersOnline: loaded.RunnersOnline,
		ExecutedTotal: atomic.LoadUint64(&executedTotal),
		MissedTotal:   atomic.LoadUint64(&missedTotal),
	}
	if loaded.Counts != nil {
		stats.QueuedRuns = loaded.Counts.QueuedCount
		stats.ClaimedRuns = loaded.Counts.ClaimedCount
	}
	if loaded.Oldest != nil {
		if age := time.Since(loaded.Oldest.ScheduledFor); age > 0 {
			stats.OldestQueuedAgeSec = int64(age.Seconds())
		}
	}
	return stats, nil
}
