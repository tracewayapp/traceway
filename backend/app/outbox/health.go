package outbox

import (
	"database/sql"
	"sync/atomic"
	"time"

	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
)

type HealthStats struct {
	Pending               int    `json:"pending"`
	Sending               int    `json:"sending"`
	OldestPendingAgeSec   int64  `json:"oldestPendingAgeSec"`
	FailedRows            int    `json:"failedRows"`
	SentTotal             uint64 `json:"sentTotal"`
	TerminalFailuresTotal uint64 `json:"terminalFailuresTotal"`
}

// HealthSnapshot powers /api/health/deep. OldestPendingAgeSec measures due-age
// (from next_attempt_at), so a scheduled future delivery does not look stuck.
func HealthSnapshot() (*HealthStats, error) {
	type snapshot struct {
		Counts *models.OutboxHealthCounts
		Oldest *models.OutboxDelivery
	}
	loaded, err := db.ExecuteTransaction(func(tx *sql.Tx) (snapshot, error) {
		counts, err := transactional.OutboxRepository.CountsForHealth(tx)
		if err != nil {
			return snapshot{}, err
		}
		oldest, err := transactional.OutboxRepository.OldestPending(tx)
		if err != nil {
			return snapshot{}, err
		}
		return snapshot{Counts: counts, Oldest: oldest}, nil
	})
	if err != nil {
		return nil, err
	}
	stats := &HealthStats{
		SentTotal:             atomic.LoadUint64(&sentTotal),
		TerminalFailuresTotal: atomic.LoadUint64(&terminalFailuresTotal),
	}
	if loaded.Counts != nil {
		stats.Pending = loaded.Counts.PendingCount
		stats.Sending = loaded.Counts.SendingCount
		stats.FailedRows = loaded.Counts.FailedCount
	}
	if loaded.Oldest != nil {
		if age := time.Since(loaded.Oldest.NextAttemptAt); age > 0 {
			stats.OldestPendingAgeSec = int64(age.Seconds())
		}
	}
	return stats, nil
}
