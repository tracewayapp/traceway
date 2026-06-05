package monitoring

import (
	"strconv"
	"sync/atomic"

	traceway "go.tracewayapp.com"
)

const (
	SignalTraces  = "traces"
	SignalMetrics = "metrics"
	SignalLogs    = "logs"
	SignalNative  = "native"
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
	traceway.CaptureMetricWithTags("traceway.ingest.batch.convert_ms", convertMs, tags)
	traceway.CaptureMetricWithTags("traceway.ingest.batch.insert_ms", insertMs, tags)
	traceway.CaptureMetricWithTags("traceway.ingest.batch.size", float64(size), tags)
	traceway.CaptureMetricWithTags("traceway.ingest.batch.bytes", float64(bytes), tags)
}

func RecordHealthchecksDropped(signal string, count int) {
	traceway.CaptureMetricWithTags("traceway.ingest.healthchecks_dropped", float64(count), map[string]string{
		"signal": signal,
	})
}

func RecordRateLimited(orgID int) {
	traceway.CaptureMetricWithTags("traceway.ingest.rate_limited", 1, map[string]string{
		"org_id": strconv.Itoa(orgID),
	})
}
