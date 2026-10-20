package perf

import (
	"math"
	"time"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/scenarios"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/seed"
)

func init() { scenarios.Register("perf/ctl-metric-p95-trap", pctSeedMetricP95Trap) }

// pctAfternoon is the afternoon traffic peak: up to 3x between 13:00 and
// 16:30 UTC, with 30-minute ramps.
func pctAfternoon(t time.Time) float64 {
	h := float64(t.Hour()) + float64(t.Minute())/60
	switch {
	case h < 12.5 || h >= 17:
		return 1
	case h < 13:
		return 1 + 2*(h-12.5)/0.5
	case h < 16.5:
		return 3
	default:
		return 3 - 2*(h-16.5)/0.5
	}
}

// pctBusy is the query hosts' busy CPU share under a load factor.
func pctBusy(load float64) float64 { return math.Min(0.985, 0.2+0.17*load) }

// pctSlowdown is how much CPU contention stretches in-process work: none
// below 60% busy, about 4x when the hosts are pegged.
func pctSlowdown(busy float64) float64 {
	if busy <= 0.6 {
		return 1
	}
	x := (busy - 0.6) / 0.38
	return 1 + 3*x*x
}

// pctSeedMetricP95Trap: every afternoon traffic triples and the two query
// hosts run at 97-100% CPU, stretching every endpoint (no single long pole).
// The user relays ops' "p95 CPU around 45%", which is what metrics query
// returns for --aggregation p95 over a day: the server has no quantiles for
// metric points and silently averages. Expected: cpu_saturation, reported
// with the max (or bucketed) CPU, not a fake percentile.
func pctSeedMetricP95Trap(s *seed.Scenario) error {
	p := s.Project("search", seed.OTel)
	window := 3 * 24 * time.Hour
	api := p.Service("query-api", seed.Go("nethttp", "pgx")).Instances("query-api-1", "query-api-2").Versions("8.2.1")

	load := func(t time.Time) float64 { return pctDiurnal(t) * pctAfternoon(t) }
	stretch := func(r *seed.Req) (cpu, db float64) {
		m := pctSlowdown(pctBusy(load(r.Start())))
		return m, 1 + (m-1)*0.25
	}
	query := api.Endpoint("GET", "/api/query")
	query.Traffic(seed.Daily(1300), seed.Diurnal(), seed.RateFunc(pctAfternoon), seed.Over(window))
	query.Detail(seed.DetailPolicy{Recent: 30 * time.Hour, Rate: 0.01, Max: 24})
	query.Handler(func(r *seed.Req) {
		cpu, db := stretch(r)
		ms := func(lo, hi, f float64) time.Duration { return time.Duration(float64(r.Ms(lo, hi)) * f) }
		r.Gap(ms(0.4, 1.2, cpu))
		r.Compute("parse query", ms(0.8, 2.5, cpu))
		r.DB("SELECT d.id, d.title, d.snippet, d.updated_at, ts_rank_cd(d.tsv, q) AS rank FROM documents d, websearch_to_tsquery('simple', $1) q WHERE d.tenant_id = $2 AND d.tsv @@ q ORDER BY rank DESC LIMIT 200",
			time.Duration(float64(r.LogNormal(18*time.Millisecond, 60*time.Millisecond))*db), seed.Rows(40+r.IntN(161)))
		r.Compute("rank results", ms(20, 55, cpu))
		r.Compute("render highlights", ms(8, 22, cpu))
		r.Gap(ms(0.3, 0.9, cpu))
	})
	suggest := api.Endpoint("GET", "/api/suggest")
	suggest.Traffic(seed.Daily(1100), seed.Diurnal(), seed.RateFunc(pctAfternoon), seed.Over(window))
	suggest.Handler(func(r *seed.Req) {
		cpu, db := stretch(r)
		ms := func(lo, hi, f float64) time.Duration { return time.Duration(float64(r.Ms(lo, hi)) * f) }
		r.Gap(ms(0.3, 0.8, cpu))
		r.DB("SELECT term, hits FROM suggest_terms WHERE tenant_id = $1 AND term LIKE $2 ORDER BY hits DESC LIMIT 10", time.Duration(float64(r.LogNormal(3*time.Millisecond, 10*time.Millisecond))*db), seed.Rows(10))
		r.Compute("encode response", ms(2, 6, cpu))
	})
	doc := api.Endpoint("GET", "/api/documents/:id")
	doc.Traffic(seed.Daily(700), seed.Diurnal(), seed.RateFunc(pctAfternoon), seed.Over(window))
	doc.Handler(func(r *seed.Req) {
		cpu, db := stretch(r)
		ms := func(lo, hi, f float64) time.Duration { return time.Duration(float64(r.Ms(lo, hi)) * f) }
		r.Gap(ms(0.3, 0.8, cpu))
		r.DB("SELECT id, title, body, updated_at FROM documents WHERE id = $1 AND tenant_id = $2", time.Duration(float64(r.LogNormal(3*time.Millisecond, 9*time.Millisecond))*db), seed.Rows(1))
		r.Compute("render document", ms(6, 18, cpu))
	})
	feedback := api.Endpoint("POST", "/api/feedback")
	feedback.Traffic(seed.Daily(240), seed.Diurnal(), seed.RateFunc(pctAfternoon), seed.Over(window))
	feedback.Handler(func(r *seed.Req) {
		cpu, db := stretch(r)
		ms := func(lo, hi, f float64) time.Duration { return time.Duration(float64(r.Ms(lo, hi)) * f) }
		r.Gap(ms(0.3, 0.8, cpu))
		r.DB("INSERT INTO search_feedback (tenant_id, query, document_id, verdict, created_at) VALUES ($1, $2, $3, $4, now())", time.Duration(float64(r.LogNormal(3*time.Millisecond, 10*time.Millisecond))*db))
		r.Compute("encode response", ms(0.3, 1, cpu))
	})

	for i, h := range api.Hosts {
		off := 0.01 * float64(i)
		busy := func(t time.Time) float64 {
			return math.Min(0.985, pctBusy(load(t))+off+0.02*(math.Sin(float64(t.Unix())/97)))
		}
		api.HostNamed(h).CPU(busy)
		api.HostNamed(h).Memory(seed.Flat(0.52, 0.6))
	}

	// What the hosts' CPU looks like to the different questions, from the
	// same busy curve the CPU series follow (one host, minute samples).
	busyAt := func(t time.Time) float64 {
		return math.Min(0.985, pctBusy(load(t))+0.02*math.Sin(float64(t.Unix())/97))
	}
	var sum, peak float64
	var mins []float64
	for t := s.Ago(24 * time.Hour); t.Before(s.Now); t = t.Add(time.Minute) {
		b := busyAt(t)
		sum += b
		peak = math.Max(peak, b)
		mins = append(mins, b)
	}
	s.Expose("busyAvg24h", math.Round(sum/float64(len(mins))*100))
	s.Expose("busyMax24h", math.Round(peak*100))

	// The most recent complete afternoon peak and a quiet morning before it.
	day := s.Now.Truncate(24 * time.Hour)
	peakFrom := day.Add(13 * time.Hour)
	if s.Now.Before(day.Add(16*time.Hour + 30*time.Minute)) {
		peakFrom = peakFrom.Add(-24 * time.Hour)
	}
	peakTo := peakFrom.Add(3*time.Hour + 30*time.Minute)
	quietFrom, quietTo := peakFrom.Add(-6*time.Hour), peakFrom.Add(-2*time.Hour)
	s.Expose("peakFrom", pctRFC3339(peakFrom))
	s.Expose("peakTo", pctRFC3339(peakTo))
	s.Expose("quietFrom", pctRFC3339(quietFrom))
	s.Expose("quietTo", pctRFC3339(quietTo))
	s.Expose("p95MsPeak", pctLazyMs(query, 0.95, peakFrom, peakTo))
	s.Expose("p95MsQuiet", pctLazyMs(query, 0.95, quietFrom, quietTo))
	s.Expose("peakRequest", pctRef(func() *seed.Request {
		return pctNearestBetween(query, peakFrom.Add(time.Hour), peakTo.Add(-time.Hour), query.PercentileBetween(0.9, peakFrom, peakTo))
	}))
	s.Expose("quietRequest", pctRef(func() *seed.Request {
		return pctNearestBetween(query, quietFrom, quietTo, query.PercentileBetween(0.9, quietFrom, quietTo))
	}))
	return nil
}
