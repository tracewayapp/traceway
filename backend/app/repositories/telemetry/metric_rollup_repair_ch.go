//go:build telemetry_ch

package telemetry

import (
	"context"

	ch "github.com/tracewayapp/traceway/backend/v2/app/repositories/telemetry/clickhouse"
)

const MetricRollupRepairApplies = true

func RepairMetricRollups(ctx context.Context, guard func(step string) error) (MetricRollupRepairReport, error) {
	return ch.RepairMetricRollups(ctx, guard)
}
