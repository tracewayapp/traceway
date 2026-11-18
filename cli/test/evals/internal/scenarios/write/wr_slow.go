package write

import (
	"math"
	"time"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/scenarios"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/seed"
)

func init() { scenarios.Register("write/mark-slow", wrSeedMarkSlow) }

// wrSeedMarkSlow: POST /api/reports/export renders a PDF inside the request,
// steadily around 3-4.5 seconds, and the team accepts that. The agent should
// read its p95 and mark it expected-slow with an offset a bit above it and
// the reason. Distractor: GET /api/reports is the next slowest endpoint and
// must not be marked.
func wrSeedMarkSlow(s *seed.Scenario) error {
	p := s.Project("backoffice", seed.OTel)
	api := p.Service("reports-api", seed.Python("fastapi", "psycopg")).Versions("4.7.2")

	export := api.Endpoint("POST", "/api/reports/export")
	export.Traffic(seed.Daily(900), seed.Diurnal(), seed.Over(2*24*time.Hour))
	export.Detail(seed.DetailPolicy{Recent: 6 * time.Hour, Rate: 0.05, Max: 15})
	export.Handler(func(r *seed.Req) {
		r.Gap(r.Ms(0.5, 1.5))
		r.DB("SELECT l.account_id, l.posted_on, l.amount_cents, l.currency, a.name FROM ledger_entries l JOIN accounts a ON a.id = l.account_id WHERE l.posted_on BETWEEN %s AND %s ORDER BY l.posted_on",
			r.LogNormal(380*time.Millisecond, 900*time.Millisecond))
		r.Compute("render pdf", r.LogNormal(2400*time.Millisecond, 3600*time.Millisecond))
		r.HTTP("PUT", "https://reports-archive.s3.eu-west-1.amazonaws.com/exports/2026/10/ledger.pdf", r.Ms(80, 260), 200)
		r.Gap(r.Ms(0.5, 2))
	})

	list := api.Endpoint("GET", "/api/reports")
	list.Traffic(seed.Daily(1200), seed.Diurnal(), seed.Over(2*24*time.Hour))
	list.Detail(seed.DetailPolicy{Recent: 6 * time.Hour, Rate: 0.02, Max: 10})
	list.Handler(func(r *seed.Req) {
		r.Gap(r.Ms(0.3, 1))
		r.DB("SELECT id, kind, status, created_by, created_at FROM reports WHERE account_id = %s ORDER BY created_at DESC LIMIT 50", r.LogNormal(160*time.Millisecond, 700*time.Millisecond))
		r.Compute("serialize", r.Ms(5, 30))
	})

	download := api.Endpoint("GET", "/api/reports/{report_id}/download")
	download.Traffic(seed.Daily(800), seed.Diurnal(), seed.Over(2*24*time.Hour))
	download.Detail(seed.DetailPolicy{Recent: 6 * time.Hour, Rate: 0.02, Max: 5})
	download.Handler(func(r *seed.Req) {
		r.Gap(r.Ms(0.3, 1))
		r.DB("SELECT id, storage_key, status FROM reports WHERE id = %s", r.Ms(1, 4))
		r.HTTP("GET", "https://reports-archive.s3.eu-west-1.amazonaws.com/exports/2026/10/ledger.pdf", r.Ms(20, 90), 200)
	})
	s.Background(api, seed.TrafficOpts{Endpoints: 5, PerDay: 150, NoErrors: true})

	s.Expose("endpoint", export.Name())
	s.Expose("otherEndpoint", list.Name())
	// The export's p95 is 3.4-3.8 s in every window an agent might read
	// (last hour, day or two days, at any time of day), which is what the
	// case's fixed offset_ms range is built on.
	s.Expose("p95", func() any {
		ms := func(d time.Duration) float64 { return math.Round(float64(d) / float64(time.Millisecond)) }
		p1, p24 := ms(export.Percentile(0.95, time.Hour)), ms(export.Percentile(0.95, 24*time.Hour))
		return map[string]any{
			"lastHourMs": p1, "lastDayMs": p24,
			"suggestedOffsetMs": math.Ceil(p24*1.15/100) * 100,
		}
	})
	return nil
}
