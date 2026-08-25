package db

import (
	"context"
	"sync"
)

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
	if telemetryIsFirebolt {
		return "firebolt"
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
