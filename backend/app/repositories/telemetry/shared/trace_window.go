package shared

import "time"

const traceLookupWindow = 24 * time.Hour

const distributedTraceLookupWindow = 48 * time.Hour

func TraceWindowBounds(recordedAt time.Time) (time.Time, time.Time) {
	return recordedAt.Add(-traceLookupWindow), recordedAt.Add(traceLookupWindow)
}

func DistributedTraceWindowBounds(recordedAt time.Time) (time.Time, time.Time) {
	return recordedAt.Add(-distributedTraceLookupWindow), recordedAt.Add(distributedTraceLookupWindow)
}
