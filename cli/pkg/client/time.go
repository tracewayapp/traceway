package client

import "github.com/tracewayapp/traceway/cli/pkg/access"

import "time"

// TimeRange is an inclusive [From, To] interval used in resource queries.
// It marshals to RFC3339 strings via the request structs that embed it.
type TimeRange = access.Window

// TimeRangeFromSince returns a TimeRange ending now and starting `d` ago.
// Equivalent to TimeRangeFromSinceAt(d, time.Now()).
func TimeRangeFromSince(d time.Duration) TimeRange {
	return TimeRangeFromSinceAt(d, time.Now())
}

// TimeRangeFromSinceAt is the testable form: caller supplies "now".
func TimeRangeFromSinceAt(d time.Duration, now time.Time) TimeRange {
	return TimeRange{From: now.Add(-d), To: now}
}

// TimeRangeFromExplicit constructs a TimeRange from two explicit instants.
// Caller is responsible for ensuring From <= To.
func TimeRangeFromExplicit(from, to time.Time) TimeRange {
	return TimeRange{From: from, To: to}
}
