//go:build !telemetry_ch

package telemetry

import "context"

// The embedded stores keep tags as JSON with sorted keys, so their rows never split.
const MetricRollupRepairApplies = false

func RepairMetricRollups(ctx context.Context, guard func(step string) error) (MetricRollupRepairReport, error) {
	return MetricRollupRepairReport{}, nil
}
