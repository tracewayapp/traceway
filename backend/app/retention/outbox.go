package retention

import (
	"context"
	"database/sql"
	"time"

	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
)

const outboxPruneInterval = 24 * time.Hour

// Terminal outbox rows are debugging artifacts: the durable audit lives in
// fired_notifications and page_notifications. Sent/cancelled rows keep a week;
// failed rows keep a month so operators can still find why an alert never
// arrived. Pending/sending rows are never pruned.
func startOutboxPrune(ctx context.Context) {
	startDBPruneWorker(ctx, "notification_outbox", outboxPruneInterval, func(tx *sql.Tx) (int64, error) {
		now := time.Now().UTC()
		return transactional.OutboxRepository.PruneTerminal(tx, now.AddDate(0, 0, -7), now.AddDate(0, 0, -30))
	})
}
