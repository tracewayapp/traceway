package perf

import (
	"math"
	"time"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/scenarios"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/seed"
)

func init() { scenarios.Register("perf/infra-memory-gc", papSeedMemoryGC) }

// papSeedMemoryGC: geo-api's autocomplete memo never evicts, so the live heap
// climbs from ~0.2GB to its 900MiB GOMEMLIMIT over seven hours; near the
// limit the GC runs back to back and requests on every endpoint stall in
// pauses (gaps with no span, up to ~0.4s), until the container is OOM killed
// and restarts with an empty heap. Distractor: the batch geocode endpoint,
// which allocates a lot per request; but its traffic is flat and small, its
// time is its own bounded work, the heap climbs smoothly with no step at its
// calls, and the pauses hit every endpoint.
func papSeedMemoryGC(s *seed.Scenario) error {
	p := s.Project("maps", seed.OTel)
	host := "geo-api-5d8c7b9f4-hq2mz"
	api := p.Service("geo-api", seed.Go("nethttp", "pgx")).Versions("2.9.4").Instances(host)
	window := 3 * 24 * time.Hour
	period := 7 * time.Hour

	// fill is how far the heap has climbed since the last restart (0..1).
	fill := func(t time.Time) float64 { return float64(t.Sub(t.Truncate(period))) / float64(period) }
	pause := func(r *seed.Req) {
		h := fill(r.At())
		if r.Chance(0.02 + 0.55*math.Pow(h, 4)) {
			r.Gap(time.Duration((15 + 380*math.Pow(h, 3)*(0.4+0.6*r.Float())) * float64(time.Millisecond)))
		}
	}
	detail := seed.DetailPolicy{Recent: 24 * time.Hour, Rate: 0.06, Max: 45}

	auto := api.Endpoint("GET", "/api/autocomplete")
	auto.Traffic(papJitter("auto", 0.06), seed.Daily(1500), seed.Diurnal(), seed.Over(window))
	auto.Detail(detail)
	auto.Handler(func(r *seed.Req) {
		r.Gap(r.Ms(0.3, 0.8))
		pause(r)
		r.Gap(r.Ms(0.05, 0.2)) // memo lookup
		if r.Chance(0.5) {
			r.Gap(r.Ms(0.2, 0.6))
			return
		}
		r.DB("SELECT id, name, kind, country, lat, lon FROM places WHERE name_prefix @@ to_tsquery($1) ORDER BY popularity DESC LIMIT 10", r.Ms(3, 9), seed.Rows(10))
		pause(r)
		r.Gap(r.Ms(0.4, 1.2))
	})

	place := api.Endpoint("GET", "/api/places/{id}")
	place.Traffic(papJitter("place", 0.06), seed.Daily(900), seed.Diurnal(), seed.Over(window))
	place.Detail(detail)
	place.Handler(func(r *seed.Req) {
		r.Gap(r.Ms(0.3, 0.8))
		pause(r)
		r.DB("SELECT id, name, kind, country, lat, lon, address, timezone FROM places WHERE id = $1", r.Ms(1.2, 3.5))
		pause(r)
		r.Gap(r.Ms(0.2, 0.6))
	})

	reverse := api.Endpoint("GET", "/api/reverse")
	reverse.Traffic(papJitter("reverse", 0.06), seed.Daily(600), seed.Diurnal(), seed.Over(window))
	reverse.Detail(detail)
	reverse.Handler(func(r *seed.Req) {
		r.Gap(r.Ms(0.3, 0.8))
		pause(r)
		r.DB("SELECT id, name, kind, country FROM places WHERE ST_DWithin(geom, ST_SetSRID(ST_MakePoint($1, $2), 4326), 200) ORDER BY geom <-> ST_SetSRID(ST_MakePoint($1, $2), 4326) LIMIT 5", r.Ms(4, 12))
		pause(r)
		r.Gap(r.Ms(0.3, 0.9))
	})

	batch := api.Endpoint("POST", "/api/geocode/batch")
	batch.Traffic(papJitter("batch", 0.1), seed.Daily(60), seed.Over(window))
	batch.Detail(seed.DetailPolicy{Recent: 24 * time.Hour, Rate: 0.3, Max: 15})
	batch.Handler(func(r *seed.Req) {
		r.Gap(r.Ms(0.5, 1.5))
		n := 500 + r.IntN(4500)
		r.Compute("decode batch", time.Duration(float64(n)*0.004*float64(time.Millisecond)), seed.A("batch.size", n)).Scope = "geo-api"
		pause(r)
		// pgx ends the query span when the rows are closed, so it covers
		// scanning the matches into the result.
		r.DB("SELECT q.idx, p.id, p.lat, p.lon FROM places p JOIN unnest($1::text[]) WITH ORDINALITY AS q(addr, idx) ON p.normalized_address = q.addr", time.Duration(float64(n)*0.11*float64(time.Millisecond))+r.Ms(10, 25), seed.Rows(n))
		pause(r)
		r.Gap(time.Duration(float64(n)*0.01*float64(time.Millisecond)) + r.Ms(1, 3))
	})

	limit := 900.0 * (1 << 20)
	heap := func(t time.Time) float64 { return 1.9e8 + (8.9e8-1.9e8)*fill(t) }
	api.Gauge("go.memory.used", "By", map[string]any{"go.memory.type": "other"}, seed.Plus(seed.Flat(-1e7, 1e7), heap))
	api.Gauge("go.memory.used", "By", map[string]any{"go.memory.type": "stack"}, seed.Flat(5.5e6, 7.5e6))
	// The DSL jitters every gauge by up to 1.5%, so the goal is capped a
	// little under GOMEMLIMIT to stay below it.
	api.Gauge("go.memory.gc.goal", "By", nil, func(t time.Time) float64 { return math.Min(0.984*limit, 2*(heap(t)-6e7)) })
	api.Gauge("go.goroutine.count", "{goroutine}", nil, seed.Flat(120, 175))
	busy := func(t time.Time) float64 { return 0.21 + 0.4*math.Pow(fill(t), 5) } // GC CPU near the limit
	api.Host().CPU(seed.Plus(seed.Flat(-0.02, 0.03), busy))
	api.Host().Memory(seed.Plus(seed.Flat(0.17, 0.19), func(t time.Time) float64 { return heap(t) / 1.6e9 }))

	var lastRestart time.Time
	for t := s.Ago(window).Truncate(period).Add(period); t.Before(s.Now); t = t.Add(period) {
		at := t.Add(time.Duration(4+int(t.Unix()/3600)%5) * time.Second)
		api.Log(at, seed.SevInfo, "starting geo-api", seed.A("version", "2.9.4"), seed.A("addr", ":8080"), seed.A("gomemlimit", "900MiB"))
		lastRestart = t
	}

	// A request that stalled after its query returned: the pause is plainly
	// not a wait for a connection.
	gappy := func(e *seed.Endpoint, lo, hi time.Duration) func() any {
		return e.Pick(func(rq *seed.Request) bool {
			cs := rq.Root.Children
			if len(cs) == 0 || !rq.Start.After(s.Ago(24*time.Hour)) {
				return false
			}
			last := cs[len(cs)-1]
			tail := rq.Duration() - (last.Start + last.Dur)
			return tail >= lo && tail <= hi
		})
	}
	s.Expose("pausedRequest", gappy(place, 180*time.Millisecond, time.Second))
	s.Expose("pausedAutocomplete", gappy(auto, 150*time.Millisecond, time.Second))
	s.Expose("calmRequest", place.Pick(func(rq *seed.Request) bool {
		return rq.Start.After(s.Ago(24*time.Hour)) && fill(rq.Start) < 0.2 && rq.Duration() < 5*time.Millisecond
	}))
	s.Expose("lastRestart", lastRestart.Format(time.RFC3339))
	s.Expose("restartFrom", lastRestart.Add(-2*time.Hour).Format(time.RFC3339))
	s.Expose("restartTo", lastRestart.Add(30*time.Minute).Format(time.RFC3339))
	// The heaviest batch of the day while the heap was still low, so its
	// spans account for its time: the pauses near the limit hit it too.
	s.Expose("batchRequest", func() any {
		var best *seed.Request
		for _, rq := range batch.Requests() {
			if rq.Start.Before(s.Ago(24*time.Hour)) || fill(rq.Start) >= 0.4 {
				continue
			}
			if best == nil || rq.Duration() > best.Duration() {
				best = rq
			}
		}
		if best == nil {
			return nil
		}
		best.KeepDetail()
		return best.Ref()
	})
	return nil
}
