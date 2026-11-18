package monitoring

import (
	"strconv"
	"sync/atomic"

	"github.com/tracewayapp/traceway/backend/v2/app/traceway"
)

const (
	SignalTraces   = "traces"
	SignalMetrics  = "metrics"
	SignalLogs     = "logs"
	SignalProfiles = "profiles"
	SignalNative   = "native"
)

var inFlightIngest atomic.Int64

func IngestStarted() {
	inFlightIngest.Add(1)
}

func IngestFinished() {
	inFlightIngest.Add(-1)
}

func InFlightIngest() int64 {
	return inFlightIngest.Load()
}

func RecordIngestBatch(signal, table string, convertMs, insertMs float64, size, bytes int) {
	tags := map[string]string{
		"signal": signal,
		"table":  table,
	}
	traceway.CaptureCount("traceway.ingest.batches", 1, tags)
	traceway.CaptureCount("traceway.ingest.rows", int64(size), tags)
	traceway.CaptureMetricMax("traceway.ingest.batch.convert_ms", convertMs, tags)
	traceway.CaptureMetricMax("traceway.ingest.batch.insert_ms", insertMs, tags)
	traceway.CaptureMetricMax("traceway.ingest.batch.size", float64(size), tags)
	traceway.CaptureMetricMax("traceway.ingest.batch.bytes", float64(bytes), tags)
}

func RecordHealthchecksDropped(signal string, count int) {
	traceway.CaptureCount("traceway.ingest.healthchecks_dropped", int64(count), map[string]string{
		"signal": signal,
	})
}

func RecordRateLimited(orgID int) {
	traceway.CaptureCount("traceway.ingest.rate_limited", 1, map[string]string{
		"org_id": strconv.Itoa(orgID),
	})
}
