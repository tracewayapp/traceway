package perf

import (
	"math"
	"time"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/seed"
)

// pctMs exposes a duration as float milliseconds rounded to one decimal.
func pctMs(d time.Duration) float64 {
	return math.Round(float64(d)/float64(time.Millisecond)*10) / 10
}

// pctLazyMs is a lazy Expose of an endpoint percentile over [from, to).
func pctLazyMs(e *seed.Endpoint, p float64, from, to time.Time) func() any {
	return func() any { return pctMs(e.PercentileBetween(p, from, to)) }
}

// pctRFC3339 formats a time for templates.
func pctRFC3339(t time.Time) string { return t.UTC().Format(time.RFC3339) }

// pctDiurnal is seed.Diurnal's factor, so CPU can follow the same load.
func pctDiurnal(t time.Time) float64 {
	h := float64(t.Hour()) + float64(t.Minute())/60
	return 1 + 0.6*math.Cos((h-14)/24*2*math.Pi)
}
