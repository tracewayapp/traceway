package retention

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/tracewayapp/traceway/backend/app/agent"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
	"github.com/tracewayapp/traceway/backend/app/storage"
	traceway "go.tracewayapp.com"
)

const (
	defaultAgentAttemptRetentionDays = 90
	agentAttemptPruneInterval        = 24 * time.Hour
	agentAttemptPruneBatch           = 200
)

// Terminal attempts older than AGENT_ATTEMPT_RETENTION_DAYS lose their event
// stream and their storage blobs; the attempt rows and the conversation stay,
// they are the history on the issue page. report_key is cleared once the
// blobs are gone, which is also what keeps a pruned attempt out of the next
// batch.
func startAgentAttemptPrune(ctx context.Context, days int) {
	if days <= 0 {
		return
	}
	startDBPruneWorker(ctx, "agent_attempts", agentAttemptPruneInterval, func(tx *sql.Tx) (int64, error) {
		return pruneAgentAttempts(ctx, tx, time.Now().UTC().AddDate(0, 0, -days))
	})
}

func pruneAgentAttempts(ctx context.Context, tx *sql.Tx, cutoff time.Time) (int64, error) {
	attempts, err := transactional.AgentAttemptRepository.FindTerminalFinishedBefore(tx, cutoff, agentAttemptPruneBatch)
	if err != nil {
		return 0, err
	}
	var pruned int64
	for _, attempt := range attempts {
		if err := deleteAttemptBlobs(ctx, attempt); err != nil {
			traceway.CaptureException(fmt.Errorf("retention: agent attempt %s blobs: %w", attempt.Id, err))
			continue
		}
		if _, err := transactional.AgentAttemptEventRepository.DeleteByAttempt(tx, attempt.Id); err != nil {
			return pruned, err
		}
		if err := transactional.AgentAttemptRepository.ClearReportKey(tx, attempt.Id, time.Now().UTC()); err != nil {
			return pruned, err
		}
		pruned++
	}
	return pruned, nil
}

func deleteAttemptBlobs(ctx context.Context, attempt *models.AgentAttempt) error {
	if storage.Store == nil {
		return nil
	}
	for _, key := range agent.AttemptBlobKeys(attempt.Id) {
		if err := storage.Store.Delete(ctx, key); err != nil {
			return fmt.Errorf("delete %s: %w", key, err)
		}
	}
	return nil
}
