package agent

import (
	"database/sql"
	"sync"
	"time"

	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
)

// HealthStats is the agent block of GET /api/health/deep and the source of
// the traceway.agent.* metrics: the queue as it is, and counters since the
// process started.
type HealthStats struct {
	Queued          int               `json:"queued"`
	PendingApproval int               `json:"pendingApproval"`
	NeedsInput      int               `json:"needsInput"`
	Active          int               `json:"active"`
	ByStatus        map[string]int    `json:"byStatus"`
	RunnersOnline   int               `json:"runnersOnline"`
	LastReclaimAt   *time.Time        `json:"lastReclaimAt,omitempty"`
	ReclaimedTotal  uint64            `json:"reclaimedTotal"`
	StartedTotal    uint64            `json:"startedTotal"`
	FinishedTotal   map[string]uint64 `json:"finishedTotal"`
	CostUSDTotal    float64           `json:"costUsdTotal"`
}

// RunnerOnlineWindow is how recently a runner must have polled to count as
// online in operational health metrics.
const RunnerOnlineWindow = 2 * time.Minute

var (
	countersMu     sync.Mutex
	startedTotal   uint64
	finishedTotal  = map[string]uint64{}
	costTotal      float64
	reclaimedTotal uint64
	lastReclaimAt  *time.Time
)

func recordStarted() {
	countersMu.Lock()
	startedTotal++
	countersMu.Unlock()
}

func recordFinished(status string) {
	countersMu.Lock()
	finishedTotal[status]++
	countersMu.Unlock()
}

// RecordCost adds a finished run's cost to the process counter.
func RecordCost(usd float64) {
	countersMu.Lock()
	costTotal += usd
	countersMu.Unlock()
}

func recordReclaim(count int64, at time.Time) {
	countersMu.Lock()
	reclaimedTotal += uint64(count)
	lastReclaimAt = &at
	countersMu.Unlock()
}

// HealthSnapshot reads the queue and copies the counters.
func HealthSnapshot() (*HealthStats, error) {
	counts, err := db.ExecuteTransaction(func(tx *sql.Tx) ([]*models.AgentAttemptStatusCount, error) {
		return transactional.AgentAttemptRepository.CountByStatus(tx)
	})
	if err != nil {
		return nil, err
	}
	online, err := db.ExecuteTransaction(func(tx *sql.Tx) (int, error) {
		return transactional.AgentRunnerRepository.CountOnline(tx, time.Now().UTC().Add(-RunnerOnlineWindow))
	})
	if err != nil {
		return nil, err
	}
	stats := &HealthStats{ByStatus: map[string]int{}, FinishedTotal: map[string]uint64{}, RunnersOnline: online}
	for _, row := range counts {
		stats.ByStatus[row.Status] = row.Count
		switch row.Status {
		case models.AttemptQueued:
			stats.Queued = row.Count
		case models.AttemptPendingApproval:
			stats.PendingApproval = row.Count
		case models.AttemptNeedsInput:
			stats.NeedsInput = row.Count
		}
		if models.AttemptIsActive(row.Status) {
			stats.Active += row.Count
		}
	}
	countersMu.Lock()
	stats.StartedTotal = startedTotal
	for status, count := range finishedTotal {
		stats.FinishedTotal[status] = count
	}
	stats.CostUSDTotal = costTotal
	stats.ReclaimedTotal = reclaimedTotal
	if lastReclaimAt != nil {
		at := *lastReclaimAt
		stats.LastReclaimAt = &at
	}
	countersMu.Unlock()
	return stats, nil
}
