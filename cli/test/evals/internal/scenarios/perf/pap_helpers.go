package perf

import (
	"fmt"
	"hash/fnv"
	"sort"
	"time"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/seed"
)

// Helpers shared by the application and infrastructure perf seeds.

// papInterval is a stretch of wall time during which some work ran.
type papInterval struct{ from, to time.Time }

// papJitter varies an endpoint's rate by a few percent per hour, so the same
// hours of two days never carry identical request counts.
func papJitter(salt string, depth float64) seed.Shape {
	return seed.RateFunc(func(t time.Time) float64 {
		h := fnv.New64a()
		fmt.Fprintf(h, "%s|%s", salt, t.UTC().Format("2006-01-02T15"))
		u := float64(h.Sum64()>>11) / float64(1<<53)
		return 1 - depth + 2*depth*u
	})
}

// papActive counts the intervals that cover t. ivs must be sorted by from.
func papActive(ivs []papInterval, t time.Time, maxLen time.Duration) int {
	i := sort.Search(len(ivs), func(i int) bool { return !ivs[i].from.Before(t.Add(-maxLen)) })
	n := 0
	for ; i < len(ivs) && !ivs[i].from.After(t); i++ {
		if t.Before(ivs[i].to) {
			n++
		}
	}
	return n
}
