package monitoring

import (
	"context"
	"fmt"
	"time"

	traceway "go.tracewayapp.com"

	"github.com/tracewayapp/traceway/backend/app/outbox"
)

const outboxReportInterval = 10 * time.Second

type outboxBaselines struct {
	sent             uint64
	terminalFailures uint64
	first            bool
}

func StartOutboxReporter(ctx context.Context) {
	go func() {
		defer traceway.Recover()

		ticker := time.NewTicker(outboxReportInterval)
		defer ticker.Stop()

		baselines := &outboxBaselines{first: true}
		reportOutboxOnce(baselines)

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				reportOutboxOnce(baselines)
			}
		}
	}()
}

func reportOutboxOnce(baselines *outboxBaselines) {
	stats, err := outbox.HealthSnapshot()
	if err != nil {
		traceway.CaptureException(fmt.Errorf("failed to snapshot outbox health for metrics: %w", err))
		return
	}
	traceway.CaptureMetric("traceway.outbox.pending", float64(stats.Pending))
	traceway.CaptureMetric("traceway.outbox.sending", float64(stats.Sending))
	traceway.CaptureMetric("traceway.outbox.oldest_pending_sec", float64(stats.OldestPendingAgeSec))
	traceway.CaptureMetric("traceway.outbox.failed_rows", float64(stats.FailedRows))
	if !baselines.first {
		traceway.CaptureMetric("traceway.outbox.sent.delta", float64(stats.SentTotal-baselines.sent))
		traceway.CaptureMetric("traceway.outbox.terminal_failures.delta", float64(stats.TerminalFailuresTotal-baselines.terminalFailures))
	}
	baselines.sent = stats.SentTotal
	baselines.terminalFailures = stats.TerminalFailuresTotal
	baselines.first = false
}
