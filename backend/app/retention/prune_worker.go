package retention

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/tracewayapp/traceway/backend/app/config"
	"github.com/tracewayapp/traceway/backend/app/db"
	traceway "go.tracewayapp.com"
)

// startDBPruneWorker runs prune once at startup and then every interval until
// ctx is cancelled, inside a panic-guarded goroutine. It is the shared
// scaffolding behind the main-DB retention workers (oauth_sessions,
// auth_tokens, notification_outbox).
func startDBPruneWorker(ctx context.Context, name string, interval time.Duration, prune func(tx *sql.Tx) (int64, error)) {
	config.Logf("Starting %s prune worker (interval: %s)", name, interval)

	go func() {
		defer traceway.Recover()

		runDBPrune(ctx, name, prune)

		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				runDBPrune(ctx, name, prune)
			}
		}
	}()
}

// runDBPrune wraps a prune closure with the shared guard, transaction, and
// non-fatal error reporting.
func runDBPrune(ctx context.Context, name string, prune func(tx *sql.Tx) (int64, error)) {
	if ctx.Err() != nil || db.DB == nil {
		return
	}
	if _, err := db.ExecuteTransaction(prune); err != nil {
		traceway.CaptureException(fmt.Errorf("retention: prune %s failed: %w", name, err))
	}
}
