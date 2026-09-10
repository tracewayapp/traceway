package monitoring

import (
	"context"
	"fmt"
	"time"

	traceway "go.tracewayapp.com"

	"github.com/tracewayapp/traceway/backend/app/agent"
)

const agentReportInterval = 10 * time.Second

type agentBaselines struct {
	started   uint64
	finished  map[string]uint64
	reclaimed uint64
	cost      float64
	first     bool
}

// StartAgentReporter emits the traceway.agent.* metrics: the queue as
// gauges, and started/finished/reclaimed/cost as deltas per interval.
func StartAgentReporter(ctx context.Context) {
	go func() {
		defer traceway.Recover()

		ticker := time.NewTicker(agentReportInterval)
		defer ticker.Stop()

		baselines := &agentBaselines{finished: map[string]uint64{}, first: true}
		reportAgentOnce(baselines)

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				reportAgentOnce(baselines)
			}
		}
	}()
}

func reportAgentOnce(baselines *agentBaselines) {
	stats, err := agent.HealthSnapshot()
	if err != nil {
		traceway.CaptureException(fmt.Errorf("failed to snapshot agent health for metrics: %w", err))
		return
	}
	traceway.CaptureMetric("traceway.agent.queued", float64(stats.Queued))
	traceway.CaptureMetric("traceway.agent.pending_approval", float64(stats.PendingApproval))
	traceway.CaptureMetric("traceway.agent.needs_input", float64(stats.NeedsInput))
	traceway.CaptureMetric("traceway.agent.active", float64(stats.Active))
	traceway.CaptureMetric("traceway.agent.runners_online", float64(stats.RunnersOnline))
	if !baselines.first {
		traceway.CaptureMetric("traceway.agent.started.delta", float64(stats.StartedTotal-baselines.started))
		traceway.CaptureMetric("traceway.agent.reclaimed.delta", float64(stats.ReclaimedTotal-baselines.reclaimed))
		traceway.CaptureMetric("traceway.agent.cost_usd.delta", stats.CostUSDTotal-baselines.cost)
		var finished uint64
		for status, count := range stats.FinishedTotal {
			delta := count - baselines.finished[status]
			finished += delta
			traceway.CaptureMetric("traceway.agent.finished."+status+".delta", float64(delta))
		}
		traceway.CaptureMetric("traceway.agent.finished.delta", float64(finished))
	}
	baselines.started = stats.StartedTotal
	baselines.reclaimed = stats.ReclaimedTotal
	baselines.cost = stats.CostUSDTotal
	baselines.finished = map[string]uint64{}
	for status, count := range stats.FinishedTotal {
		baselines.finished[status] = count
	}
	baselines.first = false
}
