package db

import (
	"context"
	"errors"
	"sync"
)

// ErrIngestQueueFull is returned by telemetry insert methods when the
// background write queue for a table is at capacity. Controllers map it to
// 503 + Retry-After so the client retries — the data was never acked.
var ErrIngestQueueFull = errors.New("telemetry ingest write queue full")

type TelemetryEngineStats struct {
	DBSizeBytes       int64 `json:"dbSizeBytes"`
	WALSizeBytes      int64 `json:"walSizeBytes"`
	MemoryUsedBytes   int64 `json:"memoryUsedBytes"`
	ReadPoolInUse     int   `json:"readPoolInUse"`
	ReadPoolWaitCount int64 `json:"readPoolWaitCount"`
	ReadPoolWaitMs    int64 `json:"readPoolWaitMs"`
}

var telemetryEngineStatsFn func(context.Context) TelemetryEngineStats

func GetTelemetryEngineStats(ctx context.Context) (TelemetryEngineStats, bool) {
	if telemetryEngineStatsFn == nil {
		return TelemetryEngineStats{}, false
	}
	return telemetryEngineStatsFn(ctx), true
}

func TelemetryBackendName() string {
	if telemetryIsDuckDB {
		return "duckdb"
	}
	if TelemetryDB != nil {
		return "sqlite"
	}
	return "clickhouse"
}

var (
	telemetryIngestMu       sync.Mutex
	telemetryDroppedRows    = map[string]uint64{}
	telemetryInsertFailures uint64
	telemetryIngestRejects  uint64
)

// RecordIngestRejected counts requests turned away by the ingest admission
// gate (503). Rejects are load shedding, not data loss — the client retries.
func RecordIngestRejected() {
	telemetryIngestMu.Lock()
	telemetryIngestRejects++
	telemetryIngestMu.Unlock()
}

func GetIngestRejects() uint64 {
	telemetryIngestMu.Lock()
	defer telemetryIngestMu.Unlock()
	return telemetryIngestRejects
}

func RecordTelemetryRowDropped(table string) {
	telemetryIngestMu.Lock()
	telemetryDroppedRows[table]++
	telemetryIngestMu.Unlock()
}

// RecordTelemetryRowsDropped counts a bulk loss, e.g. a failed background
// flush discarding rows that were already acked to the client.
func RecordTelemetryRowsDropped(table string, n uint64) {
	telemetryIngestMu.Lock()
	telemetryDroppedRows[table] += n
	telemetryIngestMu.Unlock()
}

var telemetryWriteQueueDepthFn func() map[string]int

func SetTelemetryWriteQueueDepthFn(fn func() map[string]int) {
	telemetryWriteQueueDepthFn = fn
}

func GetTelemetryWriteQueueDepths() (map[string]int, bool) {
	if telemetryWriteQueueDepthFn == nil {
		return nil, false
	}
	return telemetryWriteQueueDepthFn(), true
}

func RecordTelemetryInsertFailure() {
	telemetryIngestMu.Lock()
	telemetryInsertFailures++
	telemetryIngestMu.Unlock()
}

func GetTelemetryIngestCounters() (dropped map[string]uint64, droppedTotal uint64, insertFailures uint64) {
	telemetryIngestMu.Lock()
	defer telemetryIngestMu.Unlock()
	dropped = make(map[string]uint64, len(telemetryDroppedRows))
	for table, n := range telemetryDroppedRows {
		dropped[table] = n
		droppedTotal += n
	}
	return dropped, droppedTotal, telemetryInsertFailures
}
