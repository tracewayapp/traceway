package monitoring

import (
	"context"
	"fmt"
	"time"

	traceway "go.tracewayapp.com"

	"github.com/tracewayapp/traceway/backend/app/slo"
)

const sloReportInterval = 30 * time.Second

type sloBaselines struct {
	computed uint64
	errors   uint64
	timeouts uint64
	lastTick time.Time
	first    bool
}

func StartSLOReporter(ctx context.Context) {
	go func() {
		defer traceway.Recover()

		ticker := time.NewTicker(sloReportInterval)
		defer ticker.Stop()

		baselines := &sloBaselines{first: true}
		reportSLOOnce(ctx, baselines)

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				reportSLOOnce(ctx, baselines)
			}
		}
	}()
}

func reportSLOOnce(ctx context.Context, baselines *sloBaselines) {
	stats, err := slo.HealthSnapshot(ctx)
	if err != nil {
		traceway.CaptureException(fmt.Errorf("failed to snapshot slo health for metrics: %w", err))
	}
	if stats == nil {
		return
	}
	if stats.Freshness != nil {
		traceway.CaptureMetric("traceway.slo.enabled", float64(stats.Freshness.Enabled))
		traceway.CaptureMetric("traceway.slo.stale", float64(stats.Freshness.Stale))
		traceway.CaptureMetric("traceway.slo.max_staleness_sec", stats.Freshness.MaxStalenessSec)
		traceway.CaptureMetric("traceway.slo.backfill_pending", float64(stats.Freshness.BackfillPending))
	}
	traceway.CaptureMetric("traceway.slo.tick.running_sec", stats.RunningForSec)
	traceway.CaptureMetric("traceway.slo.shards_owned", float64(stats.OwnedShards))
	traceway.CaptureMetric("traceway.slo.instances", float64(stats.Instances))
	if tick := stats.LastTick; tick != nil && !tick.At.Equal(baselines.lastTick) {
		traceway.CaptureMetric("traceway.slo.tick.duration_ms", tick.DurationMs)
		traceway.CaptureMetric("traceway.slo.tick.compute_ms", tick.ComputeMs)
		traceway.CaptureMetric("traceway.slo.tick.store_ms", tick.StoreMs)
		traceway.CaptureMetric("traceway.slo.tick.snapshot_ms", tick.SnapshotMs)
		traceway.CaptureMetric("traceway.slo.tick.slos", float64(tick.SLOs))
		traceway.CaptureMetric("traceway.slo.tick.buckets", float64(tick.Buckets))
		traceway.CaptureMetric("traceway.slo.tick.full_snapshots", float64(tick.FullSnapshots))
		if tick.Slowest != nil {
			traceway.CaptureMetricWithTags("traceway.slo.tick.slowest_ms", tick.Slowest.DurationMs, map[string]string{"kind": tick.Slowest.Kind})
		}
		baselines.lastTick = tick.At
	}
	if !baselines.first {
		traceway.CaptureMetric("traceway.slo.computed.delta", float64(stats.ComputedTotal-baselines.computed))
		traceway.CaptureMetric("traceway.slo.errors.delta", float64(stats.ErrorsTotal-baselines.errors))
		traceway.CaptureMetric("traceway.slo.timeouts.delta", float64(stats.TimeoutsTotal-baselines.timeouts))
	}
	baselines.computed = stats.ComputedTotal
	baselines.errors = stats.ErrorsTotal
	baselines.timeouts = stats.TimeoutsTotal
	baselines.first = false
}
