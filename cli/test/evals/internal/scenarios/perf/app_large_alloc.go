package perf

import (
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/scenarios"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/seed"
)

func init() { scenarios.Register("perf/app-large-alloc", papSeedLargeAlloc) }

// papWeighted is an interval of in-memory work with the rows it holds.
type papWeighted struct {
	from, to time.Time
	rows     float64
}

// papSeedLargeAlloc: GET /api/insights/retention loads every event of the
// requested range into memory (0.1-2.5M rows) and builds the cohort table in
// Java. While one runs, the G1 old generation jumps by ~1.4KB per row and
// young collections run back to back, then the heap falls back to its
// baseline. Distractor: GC as the root cause; but the collections only
// happen while this endpoint runs, they add up to a small share of its time,
// the heap after GC returns to ~0.6GB (no leak), and every other endpoint on
// the JVM keeps its normal p99.
func papSeedLargeAlloc(s *seed.Scenario) error {
	p := s.Project("insights", seed.OTel)
	api := p.Service("insights-api", seed.Java("spring")).Versions("2.31.0")
	window := 7 * 24 * time.Hour

	var work []papWeighted
	var sorted bool
	workOf := map[*seed.Request]papWeighted{}
	daysOf := map[*seed.Request]int{}
	ranges := []int{30, 90, 180}
	retention := api.Endpoint("GET", "/api/insights/retention")
	retention.Traffic(seed.Daily(300), seed.Diurnal(), seed.Over(window))
	retention.Detail(seed.DetailPolicy{Recent: 24 * time.Hour, Rate: 0.1, Max: 30})
	retention.Handler(func(r *seed.Req) {
		days := ranges[r.IntN(len(ranges))]
		daysOf[r.Request()] = days
		workspace := 1000 + r.IntN(8000)
		end := r.Start().UTC().Truncate(24 * time.Hour)
		r.RootAttr("url.query", fmt.Sprintf("workspace=%d&from=%s&to=%s&interval=week", workspace,
			end.AddDate(0, 0, -days).Format("2006-01-02"), end.Format("2006-01-02")))
		size := float64(r.LogNormal(350*time.Millisecond, 1700*time.Millisecond).Milliseconds()) * 1000
		rows := math.Min(2.6e6, math.Max(40000, size*float64(days)/90))
		r.Gap(r.Ms(0.8, 2))
		var w papWeighted
		spanAt := r.At() // Req.At inside Span counts from the span's start
		r.Span("RetentionService.buildCohorts", func(r *seed.Req) {
			r.DB("SELECT id, name, timezone FROM workspaces WHERE id = ?", r.Ms(0.8, 2))
			fetch := time.Duration((0.15 + rows*0.9e-6) * (0.9 + 0.2*r.Float()) * float64(time.Second))
			start := spanAt.Add(r.At().Sub(r.Start()))
			r.DB("SELECT user_id, event_name, occurred_at, properties FROM events WHERE workspace_id = ? AND occurred_at >= ? AND occurred_at < ? ORDER BY occurred_at", fetch)
			build := time.Duration(rows * 3.2e-6 * (0.9 + 0.2*r.Float()) * float64(time.Second))
			r.Gap(build) // group by user, sort, bucket into cohorts
			w = papWeighted{start, start.Add(fetch + build), rows}
		}, seed.A("code.function", "buildCohorts"), seed.A("code.namespace", "com.acme.insights.retention.RetentionService"))
		work = append(work, w)
		workOf[r.Request()] = w
		sorted = false
		r.Gap(r.Ms(2, 6)) // Jackson writes the 26x26 cohort table
	})

	funnels := api.Endpoint("GET", "/api/insights/funnels")
	funnels.Traffic(seed.Daily(300), seed.Diurnal(), seed.Over(window))
	funnels.Handler(func(r *seed.Req) {
		r.Gap(r.Ms(0.8, 2))
		r.DB("SELECT step, count(DISTINCT user_id) FROM funnel_steps WHERE workspace_id = ? AND funnel_id = ? AND day >= ? GROUP BY step ORDER BY step", r.LogNormal(60*time.Millisecond, 180*time.Millisecond))
		r.Gap(r.Ms(1, 3))
	})

	dash := api.Endpoint("GET", "/api/insights/dashboards/{id}")
	dash.Traffic(seed.Daily(500), seed.Diurnal(), seed.Over(window))
	dash.Handler(func(r *seed.Req) {
		r.Gap(r.Ms(0.8, 2))
		r.DB("SELECT id, name, layout, updated_at FROM dashboards WHERE id = ? AND workspace_id = ?", r.Ms(1, 3))
		r.DB("SELECT id, kind, query, position FROM dashboard_cards WHERE dashboard_id = ? ORDER BY position", r.Ms(1.5, 4))
		r.Gap(r.Ms(1, 2.5))
	})

	s.Background(api, seed.TypicalTraffic(6))

	ensure := func() {
		if !sorted {
			sort.Slice(work, func(i, j int) bool { return work[i].from.Before(work[j].from) })
			sorted = true
		}
	}
	held := func(t time.Time) float64 { // rows in memory at t
		ensure()
		i := sort.Search(len(work), func(i int) bool { return !work[i].from.Before(t.Add(-time.Minute)) })
		v := 0.0
		for ; i < len(work) && !work[i].from.After(t); i++ {
			if t.Before(work[i].to) {
				v += work[i].rows
			}
		}
		return v
	}
	churn := func(t time.Time) float64 { // million-row-seconds of work in the minute before t
		ensure()
		lo := t.Add(-time.Minute)
		i := sort.Search(len(work), func(i int) bool { return !work[i].from.Before(lo.Add(-time.Minute)) })
		v := 0.0
		for ; i < len(work) && work[i].from.Before(t); i++ {
			a, b := work[i].from, work[i].to
			if a.Before(lo) {
				a = lo
			}
			if b.After(t) {
				b = t
			}
			if b.After(a) {
				v += b.Sub(a).Seconds() * work[i].rows / 1e6
			}
		}
		return v
	}

	pool := func(name string) map[string]any {
		return map[string]any{"jvm.memory.type": "heap", "jvm.memory.pool.name": name}
	}
	old := func(t time.Time) float64 { return math.Min(3.7e9, 6.2e8+held(t)*1400) }
	api.Gauge("jvm.memory.used", "By", pool("G1 Old Gen"), seed.Plus(seed.Flat(-2e7, 2e7), old))
	api.Gauge("jvm.memory.used", "By", pool("G1 Eden Space"), seed.Flat(1.2e8, 7.8e8))
	api.Gauge("jvm.memory.used", "By", pool("G1 Survivor Space"), seed.Plus(seed.Flat(1e7, 3e7), func(t time.Time) float64 { return math.Min(1.2e8, held(t)*40) }))
	api.Gauge("jvm.memory.used_after_last_gc", "By", pool("G1 Old Gen"), seed.Plus(seed.Flat(5.9e8, 6.4e8), func(t time.Time) float64 { return held(t) * 1100 }))
	api.Gauge("jvm.memory.limit", "By", pool("G1 Old Gen"), seed.Const(4.0e9))
	young := map[string]any{"jvm.gc.name": "G1 Young Generation", "jvm.gc.action": "end of minor GC"}
	api.Histogram("jvm.gc.duration", "s", young,
		func(t time.Time) float64 { return 0.008 + 0.0022*math.Min(14, churn(t)) },
		func(t time.Time) float64 { return 3 + 1.2*churn(t) })
	api.Host().CPU(seed.Plus(seed.Flat(0.18, 0.26), func(t time.Time) float64 { return math.Min(0.4, 0.35*math.Min(1, held(t)/1e6)) }))
	api.Host().Memory(seed.Flat(0.71, 0.74)) // the JVM reserves its 4GB heap up front

	var sampled *seed.Request
	slowSampled := func() *seed.Request {
		if sampled != nil {
			return sampled
		}
		for _, rq := range retention.Requests() {
			w := workOf[rq]
			if rq.Start.Before(s.Ago(24*time.Hour)) || w.from.Truncate(time.Minute).Equal(w.to.Truncate(time.Minute)) {
				continue
			}
			if sampled == nil || rq.Duration() > sampled.Duration() {
				sampled = rq
			}
		}
		sampled.KeepDetail()
		return sampled
	}
	s.Expose("slowRequest", func() any { return slowSampled().Ref() })
	s.Expose("slowFrom", func() any {
		return slowSampled().Start.Add(-10 * time.Minute).Truncate(time.Minute).Format(time.RFC3339)
	})
	s.Expose("slowTo", func() any {
		return slowSampled().Start.Add(11 * time.Minute).Truncate(time.Minute).Format(time.RFC3339)
	})
	s.Expose("fastRequest", retention.Pick(func(rq *seed.Request) bool {
		return rq.Start.After(s.Ago(24*time.Hour)) && daysOf[rq] == 30 && rq.Duration() < 900*time.Millisecond
	}))
	s.Expose("p50Ms24h", func() any { return retention.Percentile(0.5, 24*time.Hour).Seconds() * 1000 })
	s.Expose("p99Ms24h", func() any { return retention.Percentile(0.99, 24*time.Hour).Seconds() * 1000 })
	return nil
}
