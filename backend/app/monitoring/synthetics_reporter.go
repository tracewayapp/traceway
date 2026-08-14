package monitoring

import (
	"context"
	"fmt"
	"time"

	traceway "go.tracewayapp.com"

	"github.com/tracewayapp/traceway/backend/app/synthetics"
)

const syntheticsReportInterval = 10 * time.Second

type syntheticsBaselines struct {
	executed uint64
	missed   uint64
	first    bool
}

func StartSyntheticsReporter(ctx context.Context) {
	go func() {
		defer traceway.Recover()

		ticker := time.NewTicker(syntheticsReportInterval)
		defer ticker.Stop()

		baselines := &syntheticsBaselines{first: true}
		reportSyntheticsOnce(baselines)

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				reportSyntheticsOnce(baselines)
			}
		}
	}()
}

func reportSyntheticsOnce(baselines *syntheticsBaselines) {
	stats, err := synthetics.HealthSnapshot()
	if err != nil {
		traceway.CaptureException(fmt.Errorf("failed to snapshot synthetics health for metrics: %w", err))
		return
	}
	traceway.CaptureMetric("traceway.synthetics.queued", float64(stats.QueuedRuns))
	traceway.CaptureMetric("traceway.synthetics.claimed", float64(stats.ClaimedRuns))
	traceway.CaptureMetric("traceway.synthetics.oldest_queued_sec", float64(stats.OldestQueuedAgeSec))
	traceway.CaptureMetric("traceway.synthetics.checks_down", float64(stats.ChecksDown))
	traceway.CaptureMetric("traceway.synthetics.runners_online", float64(stats.RunnersOnline))
	if !baselines.first {
		traceway.CaptureMetric("traceway.synthetics.executed.delta", float64(stats.ExecutedTotal-baselines.executed))
		traceway.CaptureMetric("traceway.synthetics.missed.delta", float64(stats.MissedTotal-baselines.missed))
	}
	baselines.executed = stats.ExecutedTotal
	baselines.missed = stats.MissedTotal
	baselines.first = false
}
