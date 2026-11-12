package monitoring

import (
	"database/sql"

	"github.com/tracewayapp/traceway/backend/v2/app/db"
	"github.com/tracewayapp/traceway/backend/v2/app/traceway"
)

func reportDBPools(previous map[string]sql.DBStats) {
	reportDBPool(previous, "main", db.DB)
	reportDBPool(previous, "telemetry", db.TelemetryDB)
}

func reportDBPool(previous map[string]sql.DBStats, pool string, conn *sql.DB) {
	if conn == nil {
		return
	}
	stats := conn.Stats()
	tags := map[string]string{"pool": pool}
	traceway.CaptureMetricWithTags("traceway.db.pool.open", float64(stats.OpenConnections), tags)
	traceway.CaptureMetricWithTags("traceway.db.pool.in_use", float64(stats.InUse), tags)
	traceway.CaptureMetricWithTags("traceway.db.pool.idle", float64(stats.Idle), tags)
	traceway.CaptureMetricWithTags("traceway.db.pool.max_open", float64(stats.MaxOpenConnections), tags)
	if prev, ok := previous[pool]; ok {
		traceway.CaptureMetricWithTags("traceway.db.pool.wait_count.delta", float64(safeDeltaInt64(prev.WaitCount, stats.WaitCount)), tags)
		traceway.CaptureMetricWithTags("traceway.db.pool.wait_ms.delta", float64(safeDeltaInt64(int64(prev.WaitDuration), int64(stats.WaitDuration)))/1e6, tags)
	}
	previous[pool] = stats
}
