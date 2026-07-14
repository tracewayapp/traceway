package monitoring

import (
	"context"
	"time"

	traceway "go.tracewayapp.com"

	"github.com/tracewayapp/traceway/backend/app/db"
)

const telemetryDBReportInterval = 10 * time.Second

type telemetryDBBaselines struct {
	dropped        uint64
	insertFailures uint64
	poolWaitCount  int64
	poolWaitMs     int64
	first          bool
}

func StartTelemetryDBReporter(ctx context.Context) {
	if !db.IsDuckDBTelemetry() {
		return
	}
	go func() {
		defer traceway.Recover()

		ticker := time.NewTicker(telemetryDBReportInterval)
		defer ticker.Stop()

		baselines := &telemetryDBBaselines{first: true}

		reportTelemetryDBOnce(ctx, baselines)

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				reportTelemetryDBOnce(ctx, baselines)
			}
		}
	}()
}

func reportTelemetryDBOnce(ctx context.Context, b *telemetryDBBaselines) {
	_, droppedTotal, insertFailures := db.GetTelemetryIngestCounters()
	engine, ok := db.GetTelemetryEngineStats(ctx)
	if !ok {
		return
	}

	if !b.first {
		traceway.CaptureMetric("traceway.duckdb.rows_dropped.delta", float64(safeDelta(b.dropped, droppedTotal)))
		traceway.CaptureMetric("traceway.duckdb.insert_failures.delta", float64(safeDelta(b.insertFailures, insertFailures)))
		traceway.CaptureMetric("traceway.duckdb.read_pool.wait_count.delta", float64(safeDeltaInt64(b.poolWaitCount, engine.ReadPoolWaitCount)))
		traceway.CaptureMetric("traceway.duckdb.read_pool.wait_ms.delta", float64(safeDeltaInt64(b.poolWaitMs, engine.ReadPoolWaitMs)))
	}
	b.dropped = droppedTotal
	b.insertFailures = insertFailures
	b.poolWaitCount = engine.ReadPoolWaitCount
	b.poolWaitMs = engine.ReadPoolWaitMs
	b.first = false

	traceway.CaptureMetric("traceway.duckdb.db_size_mb", float64(engine.DBSizeBytes)/1024.0/1024.0)
	traceway.CaptureMetric("traceway.duckdb.wal_size_mb", float64(engine.WALSizeBytes)/1024.0/1024.0)
	traceway.CaptureMetric("traceway.duckdb.memory_used_mb", float64(engine.MemoryUsedBytes)/1024.0/1024.0)
	traceway.CaptureMetric("traceway.duckdb.read_pool.in_use", float64(engine.ReadPoolInUse))
}

func safeDeltaInt64(prev, cur int64) int64 {
	if cur < prev {
		return cur
	}
	return cur - prev
}
