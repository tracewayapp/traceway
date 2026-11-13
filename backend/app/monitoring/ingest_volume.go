package monitoring

import (
	"strconv"
	"sync"
	"time"

	"github.com/tracewayapp/traceway/backend/v2/app/traceway"
)

type volumeBucket struct {
	second                                           int64
	received, accepted, dropped, saturated, failures int64
}
type logVolume struct {
	mu      sync.Mutex
	buckets [30]volumeBucket
}

var ingestLogVolume logVolume

func (v *logVolume) add(now time.Time, received, accepted, dropped, saturated, failures int) {
	v.mu.Lock()
	defer v.mu.Unlock()
	second := now.Unix()
	b := &v.buckets[second%30]
	if b.second != second {
		*b = volumeBucket{second: second}
	}
	b.received += int64(received)
	b.accepted += int64(accepted)
	b.dropped += int64(dropped)
	b.saturated += int64(saturated)
	b.failures += int64(failures)
}

func (v *logVolume) snapshot(now time.Time) volumeBucket {
	v.mu.Lock()
	defer v.mu.Unlock()
	var total volumeBucket
	for _, b := range v.buckets {
		if b.second > now.Unix()-30 && b.second <= now.Unix() {
			total.received += b.received
			total.accepted += b.accepted
			total.dropped += b.dropped
			total.saturated += b.saturated
			total.failures += b.failures
		}
	}
	return total
}

func RecordLogsReceived(n int) {
	ingestLogVolume.add(time.Now(), n, 0, 0, 0, 0)
	traceway.CaptureCount("traceway.ingest.logs.received", int64(n), nil)
}
func RecordLogsAccepted(n int) {
	ingestLogVolume.add(time.Now(), 0, n, 0, 0, 0)
	traceway.CaptureCount("traceway.ingest.logs.accepted", int64(n), nil)
}
func RecordLogsDropped(n int, saturated bool) {
	s := 0
	if saturated {
		s = 1
	}
	ingestLogVolume.add(time.Now(), 0, 0, n, s, 0)
	traceway.CaptureCount("traceway.ingest.logs.dropped", int64(n), nil)
}
func RecordIngestFailure() { ingestLogVolume.add(time.Now(), 0, 0, 0, 0, 1) }
func reportIngestVolume() {
	v := ingestLogVolume.snapshot(time.Now())
	traceway.CaptureMetric("traceway.ingest.logs.received_30s", float64(v.received))
	traceway.CaptureMetric("traceway.ingest.logs.accepted_30s", float64(v.accepted))
	traceway.CaptureMetric("traceway.ingest.logs.dropped_30s", float64(v.dropped))
	traceway.CaptureMetric("traceway.ingest.logs.limiter_saturated_30s", float64(v.saturated))
	traceway.CaptureMetric("traceway.ingest.failures_30s", float64(v.failures))
}

func RecordIngestOutcome(route string, status int, elapsed time.Duration) {
	if status >= 400 {
		RecordIngestFailure()
	}
	tags := map[string]string{"route": route, "status_class": strconv.Itoa(status/100) + "xx"}
	traceway.CaptureCount("traceway.ingest.requests", 1, tags)
	traceway.CaptureMetricMax("traceway.ingest.request.duration_ms", float64(elapsed.Microseconds())/1000, tags)
}
