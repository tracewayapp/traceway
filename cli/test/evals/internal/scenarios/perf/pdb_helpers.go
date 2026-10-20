package perf

import (
	"time"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/seed"
)

// pdbRoute is one healthy route of a perf-db scenario: a few fast queries
// and a short encode, so the slow route has neighbours that look normal.
type pdbRoute struct {
	method, route string
	perDay        int
	queries       []string
	// dbMedianMs is the median of each query (default 2ms).
	dbMedianMs float64
}

// pdbHealthy adds healthy routes to a service over window.
func pdbHealthy(api *seed.Service, window time.Duration, routes ...pdbRoute) {
	for _, rt := range routes {
		rt := rt
		med := rt.dbMedianMs
		if med == 0 {
			med = 2
		}
		e := api.Endpoint(rt.method, rt.route)
		e.Traffic(seed.Daily(rt.perDay), seed.Diurnal(), seed.Over(window))
		e.Detail(seed.DetailPolicy{Recent: 6 * time.Hour, Rate: 0.02, Max: 4})
		e.Handler(func(r *seed.Req) {
			r.Gap(r.Ms(0.4, 1.2))
			for _, q := range rt.queries {
				m := time.Duration(med * float64(time.Millisecond))
				r.DB(q, r.LogNormal(m, m*5))
			}
			r.Gap(r.Ms(0.2, 1.4))
			r.Gap(r.Ms(0.1, 0.5))
		})
	}
}

// pdbSlowest returns the slowest request of e started after since that
// satisfies keep, forcing its detail, as a lazy seed.json reference.
func pdbSlowest(e *seed.Endpoint, since time.Time, keep func(r *seed.Request) bool) func() any {
	return func() any {
		r := pdbSlowestRequest(e, since, keep)
		if r == nil {
			return nil
		}
		return r.Ref()
	}
}

func pdbSlowestRequest(e *seed.Endpoint, since time.Time, keep func(r *seed.Request) bool) *seed.Request {
	var best *seed.Request
	for _, r := range e.Requests() {
		if r.Start.Before(since) || (keep != nil && !keep(r)) {
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

// pdbMs is a lazy percentile in milliseconds over [from, to).
func pdbMs(e *seed.Endpoint, p float64, from, to time.Time) func() any {
	return func() any { return float64(e.PercentileBetween(p, from, to)) / float64(time.Millisecond) }
}

// pdbDur converts milliseconds to a duration.
func pdbDur(ms float64) time.Duration { return time.Duration(ms * float64(time.Millisecond)) }
