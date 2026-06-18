package repositories

import "time"

const traceLookupWindow = 24 * time.Hour

const distributedTraceLookupWindow = 48 * time.Hour

func traceWindowBounds(recordedAt time.Time) (time.Time, time.Time) {
	return recordedAt.Add(-traceLookupWindow), recordedAt.Add(traceLookupWindow)
}

func distributedTraceWindowBounds(recordedAt time.Time) (time.Time, time.Time) {
	return recordedAt.Add(-distributedTraceLookupWindow), recordedAt.Add(distributedTraceLookupWindow)
}
