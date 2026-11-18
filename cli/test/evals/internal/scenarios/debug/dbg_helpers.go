package debug

import (
	"math"
	"sort"
	"time"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/seed"
)

// dbgTimes returns n instants in [from, to), in order. With ramp, arrivals
// thicken towards `to` (a staged rollout reaching more users).
func dbgTimes(s *seed.Scenario, from, to time.Time, n int, ramp bool) []time.Time {
	out := make([]time.Time, 0, n)
	span := float64(to.Sub(from))
	for i := 0; i < n; i++ {
		u := s.RNG.Float64()
		if ramp {
			u = math.Sqrt(u)
		}
		out = append(out, from.Add(time.Duration(u*span)))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Before(out[j]) })
	return out
}

// dbgPick returns one element of xs from the scenario's RNG.
func dbgPick[T any](s *seed.Scenario, xs []T) T { return xs[s.RNG.IntN(len(xs))] }
