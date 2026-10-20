package perf

import (
	"fmt"
	"time"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/seed"
)

// pctSlowestBetween returns the slowest request started in [from, to) and
// forces its detail.
func pctSlowestBetween(e *seed.Endpoint, from, to time.Time) *seed.Request {
	var best *seed.Request
	for _, r := range e.Requests() {
		if r.Start.Before(from) || !r.Start.Before(to) {
			continue
		}
		if best == nil || r.Duration() > best.Duration() {
			best = r
		}
	}
	if best != nil {
		best.KeepDetail()
	}
	return best
}

// pctRef is a lazy Expose of a request picked when the scenario flushes.
func pctRef(pick func() *seed.Request) func() any {
	return func() any {
		r := pick()
		if r == nil {
			return nil
		}
		return r.Ref()
	}
}

// pctNearestBetween returns the request started in [from, to) whose duration
// is closest to target, and forces its detail.
func pctNearestBetween(e *seed.Endpoint, from, to time.Time, target time.Duration) *seed.Request {
	var best *seed.Request
	var bestD time.Duration
	for _, r := range e.Requests() {
		if r.Start.Before(from) || !r.Start.Before(to) {
			continue
		}
		d := r.Duration() - target
		if d < 0 {
			d = -d
		}
		if best == nil || d < bestD {
			best, bestD = r, d
		}
	}
	if best != nil {
		best.KeepDetail()
	}
	return best
}

// pctAgoKey names an exposed time "h" hours before now: ago54h, ago52h30m.
func pctAgoKey(h float64) string {
	d := time.Duration(h * float64(time.Hour))
	if m := int(d.Minutes()) % 60; m != 0 {
		return fmt.Sprintf("ago%dh%dm", int(d.Hours()), m)
	}
	return fmt.Sprintf("ago%dh", int(d.Hours()))
}
