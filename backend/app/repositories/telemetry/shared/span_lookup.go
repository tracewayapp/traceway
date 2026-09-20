package shared

import (
	"time"

	"github.com/google/uuid"
)

const SpanLookupBatchSize = 100

// SpanLookup names the spans to read: a trace in a project, and for a subtree the span it hangs under.
type SpanLookup struct {
	ProjectId  uuid.UUID
	TraceId    string
	SpanId     string
	RecordedAt *time.Time
	// StartUnixNano is the start of the span named by SpanId, used to re-read a trace from that point when the row cap
	// cut it off.
	StartUnixNano uint64
}

// NewSpanLookup names the subtree under an endpoint, a task or an AI trace. Each of them is recorded at its span's start.
func NewSpanLookup(projectId uuid.UUID, traceId, spanId string, recordedAt time.Time) SpanLookup {
	lookup := SpanLookup{ProjectId: projectId, TraceId: traceId, SpanId: spanId, RecordedAt: &recordedAt}
	if recordedAt.Unix() > 0 {
		lookup.StartUnixNano = uint64(recordedAt.Unix())*uint64(time.Second) + uint64(recordedAt.Nanosecond())
	}
	return lookup
}

// OtelLookupWindow is the recorded_at range every read of spans is held to: 24 hours either side of the rows looked up.
// On ClickHouse that is what prunes the read to a few daily partitions. A lookup that names no time is anchored on now,
// so no caller can reach an unbounded read.
func OtelLookupWindow(lookups []SpanLookup) (from, to time.Time) {
	for i, lookup := range lookups {
		anchor := time.Now().UTC()
		if lookup.RecordedAt != nil {
			anchor = *lookup.RecordedAt
		} else if lookup.StartUnixNano != 0 {
			anchor = OtelNanosToTime(lookup.StartUnixNano)
		}
		low, high := TraceWindowBounds(anchor)
		if i == 0 || low.Before(from) {
			from = low
		}
		if i == 0 || high.After(to) {
			to = high
		}
	}
	return from, to
}
