package repositories

import "time"

const traceLookupWindow = 24 * time.Hour

func traceWindowBounds(recordedAt time.Time) (time.Time, time.Time) {
	return recordedAt.Add(-traceLookupWindow), recordedAt.Add(traceLookupWindow)
}
