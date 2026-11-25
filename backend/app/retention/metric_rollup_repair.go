package retention

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/tracewayapp/traceway/backend/v2/app/config"
	"github.com/tracewayapp/traceway/backend/v2/app/db"
	"github.com/tracewayapp/traceway/backend/v2/app/repositories/telemetry"
	"github.com/tracewayapp/traceway/backend/v2/app/traceway"
)

const (
	metricRollupRepairInterval = time.Hour
	metricRollupRepairLockId   = 824737009
)

// Opt in: the repair rewrites parts of the rollup tables, which competes with ingest
// for telemetry I/O. It ends for good after the first pass that leaves nothing behind.
func startMetricRollupRepair(ctx context.Context) {
	enabled, err := strconv.ParseBool(strings.TrimSpace(config.Config.MetricRollupRepair))
	if err != nil || !enabled {
		return
	}
	if !telemetry.MetricRollupRepairApplies {
		config.Logf("metric rollup repair: METRIC_ROLLUP_REPAIR only applies to ClickHouse telemetry, nothing to do")
		return
	}
	go func() {
		defer traceway.Recover()
		for ctx.Err() == nil {
			if runMetricRollupRepair(ctx) {
				return
			}
			timer := time.NewTimer(metricRollupRepairInterval)
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
		}
	}()
}

func runMetricRollupRepair(ctx context.Context) (finished bool) {
	traceway.RunTask(ctx, "retention.metric_rollup_repair", func(ctx context.Context) {
		guard, release, err := lockMetricRollupRepair(ctx)
		if err != nil {
			config.LogfContext(ctx, "WARNING: metric rollup repair: could not take the lock, retrying in %s: %v", metricRollupRepairInterval, err)
			traceway.CaptureExceptionContext(ctx, fmt.Errorf("metric rollup repair: taking the lock: %w", err))
			return
		}
		if guard == nil {
			config.LogfContext(ctx, "metric rollup repair: another instance is running it, checking again in %s", metricRollupRepairInterval)
			traceway.SkipTask(ctx)
			return
		}
		defer release()

		report, err := telemetry.RepairMetricRollups(ctx, guard)
		switch {
		case errors.Is(err, context.Canceled):
		case err != nil:
			config.LogfContext(ctx, "WARNING: metric rollup repair: stopped, retrying in %s: %v", metricRollupRepairInterval, err)
			traceway.CaptureExceptionContext(ctx, fmt.Errorf("metric rollup repair stopped: %w", err))
		case len(report.Skipped) > 0:
			config.LogfContext(ctx, "WARNING: metric rollup repair: %d partitions were skipped, retrying in %s: %s", len(report.Skipped), metricRollupRepairInterval, strings.Join(report.Skipped, "; "))
		default:
			finished = true
			config.LogfContext(ctx, "metric rollup repair: complete, METRIC_ROLLUP_REPAIR can be unset")
		}
	})
	return finished
}

// lockMetricRollupRepair keeps a second instance out with a session advisory lock held
// on one connection for the whole pass. A nil guard means another instance holds it.
// The guard fails once that connection is gone, and the lock with it.
func lockMetricRollupRepair(ctx context.Context) (guard func(step string) error, release func(), err error) {
	if db.IsSQLite() {
		return func(string) error { return nil }, func() {}, nil
	}
	conn, err := db.DB.Conn(ctx)
	if err != nil {
		return nil, nil, err
	}
	var held bool
	if err := conn.QueryRowContext(ctx, "SELECT pg_try_advisory_lock($1)", metricRollupRepairLockId).Scan(&held); err != nil || !held {
		_ = conn.Close()
		return nil, nil, err
	}
	guard = func(step string) error {
		if err := conn.PingContext(ctx); err != nil {
			return fmt.Errorf("lost the repair lock before %s: %w", step, err)
		}
		return nil
	}
	release = func() {
		_, _ = conn.ExecContext(context.WithoutCancel(ctx), "SELECT pg_advisory_unlock($1)", metricRollupRepairLockId)
		_ = conn.Close()
	}
	return guard, release, nil
}
