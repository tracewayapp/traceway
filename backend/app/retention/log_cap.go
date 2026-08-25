package retention

import (
	"context"
	"fmt"
	"time"

	"github.com/tracewayapp/traceway/backend/app/config"
	"github.com/tracewayapp/traceway/backend/app/db"
	traceway "go.tracewayapp.com"
)

const logCapTickInterval = time.Minute

func startLogRecordsCap(ctx context.Context, maxRows int) {
	if db.TelemetryDB == nil || maxRows <= 0 {
		return
	}

	config.Logf("Starting log_records row cap worker (max rows: %d, interval: %s)", maxRows, logCapTickInterval)

	go func() {
		defer traceway.Recover()

		runLogRecordsCap(ctx, maxRows)

		ticker := time.NewTicker(logCapTickInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				runLogRecordsCap(ctx, maxRows)
			}
		}
	}()
}

func runLogRecordsCap(ctx context.Context, maxRows int) {
	if db.TelemetryDB == nil || ctx.Err() != nil {
		return
	}

	// Deletes everything strictly older than the Nth-newest row; a no-op (NULL
	// boundary) while the table holds maxRows rows or fewer.
	query := fmt.Sprintf(
		"DELETE FROM log_records WHERE timestamp < (SELECT timestamp FROM log_records ORDER BY timestamp DESC LIMIT 1 OFFSET %d)",
		maxRows-1,
	)
	if _, err := db.TelemetryDB.ExecContext(ctx, query); err != nil {
		traceway.CaptureException(fmt.Errorf("retention: log_records row cap delete failed: %w", err))
	}
}
