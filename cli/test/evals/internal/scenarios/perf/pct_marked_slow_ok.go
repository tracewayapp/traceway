package perf

import (
	"time"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/scenarios"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/seed"
)

func init() { scenarios.Register("perf/ctl-marked-slow-ok", pctSeedMarkedSlowOK) }

// pctLedgerExport is the ledger export handler: one range query over a
// quarter of ledger entries whose rows are written as CSV while they are
// scanned (pgx ends the query span when the rows are closed, so it covers
// the encode), then the final flush; all proportional to the row count (at
// most ~1.98s for 54k rows).
func pctLedgerExport(r *seed.Req, rows int) {
	n := float64(rows)
	jit := func(lo, hi float64) float64 { return lo + (hi-lo)*r.Float() }
	r.Gap(r.Ms(0.5, 1.2))
	r.DB("SELECT id, posted_at, account_code, counterparty, description, amount_cents, currency FROM ledger_entries WHERE org_id = $1 AND posted_at >= $2 AND posted_at < $3 ORDER BY posted_at",
		time.Duration(n*(0.019*jit(0.96, 1.04)+0.0135*jit(0.96, 1.04))*float64(time.Millisecond)), seed.Rows(rows))
	r.Gap(time.Duration(n * 0.0025 * jit(0.9, 1.1) * float64(time.Millisecond)))
}

// pctLedgerRows draws the rows one export returns: most customers export
// 18-45k entries a quarter, one in ten (the large accounts) 49-54k.
func pctLedgerRows(r *seed.Req) int {
	if r.Chance(0.1) {
		return 49000 + r.IntN(5000)
	}
	return 18000 + r.IntN(27000)
}

// pctLedgerService declares the ledger API with its everyday endpoints.
func pctLedgerService(s *seed.Scenario, p *seed.Project, window time.Duration) *seed.Service {
	api := p.Service("ledger-api", seed.Go("nethttp", "pgx")).Versions("3.9.2")
	bal := api.Endpoint("GET", "/api/accounts/{id}/balance")
	bal.Traffic(seed.Daily(200), seed.Diurnal(), seed.Over(window))
	bal.Handler(func(r *seed.Req) {
		r.Gap(r.Ms(0.4, 1.0))
		r.DB("SELECT balance_cents, currency, updated_at FROM account_balances WHERE account_id = $1", r.LogNormal(2*time.Millisecond, 9*time.Millisecond))
		r.Gap(r.Ms(0.2, 0.8))
	})
	entries := api.Endpoint("GET", "/api/accounts/{id}/entries")
	entries.Traffic(seed.Daily(120), seed.Diurnal(), seed.Over(window))
	entries.Handler(func(r *seed.Req) {
		r.Gap(r.Ms(0.4, 1.0))
		r.DB("SELECT id, posted_at, description, amount_cents FROM ledger_entries WHERE account_id = $1 ORDER BY posted_at DESC LIMIT 50", r.LogNormal(6*time.Millisecond, 24*time.Millisecond), seed.Rows(50))
		r.Gap(r.Ms(0.6, 2.0))
	})
	post := api.Endpoint("POST", "/api/journal-entries")
	post.Traffic(seed.Daily(60), seed.Diurnal(), seed.Over(window))
	post.Handler(func(r *seed.Req) {
		r.Gap(r.Ms(0.5, 1.4))
		r.DB("INSERT INTO ledger_entries (org_id, account_id, posted_at, description, amount_cents, currency) VALUES ($1, $2, $3, $4, $5, $6)", r.LogNormal(4*time.Millisecond, 15*time.Millisecond))
		r.DB("UPDATE account_balances SET balance_cents = balance_cents + $2, updated_at = now() WHERE account_id = $1", r.LogNormal(3*time.Millisecond, 12*time.Millisecond))
		r.Status(201)
	})
	s.Background(api, seed.TypicalTraffic(6))
	return api
}

// pctSeedMarkedSlowOK: the ledger export is the slowest endpoint (p95 about
// 1.8s, no request over 2s), but an operator marked it slow with a 2000ms
// allowance and it has held that level for the whole month. Expected:
// none_found. Distractor: the export's one big range query and CSV encode
// look like an expensive query or an unbounded result set if the allowance
// is ignored.
func pctSeedMarkedSlowOK(s *seed.Scenario) error {
	p := s.Project("ledger", seed.OTel)
	window := 30 * 24 * time.Hour
	api := pctLedgerService(s, p, window)

	export := api.Endpoint("GET", "/api/reports/export")
	export.Traffic(seed.Daily(120), seed.Diurnal(), seed.Over(window))
	export.Detail(seed.DetailPolicy{Recent: 24 * time.Hour, Rate: 0.3, Max: 12})
	export.Handler(func(r *seed.Req) { pctLedgerExport(r, pctLedgerRows(r)) })

	api.Host().CPU(seed.Flat(0.2, 0.3))
	api.Host().Memory(seed.Flat(0.5, 0.58))

	p.MarkSlow("GET /api/reports/export", 2000, "Full ledger CSV export for the finance team: one quarter of entries per request, streamed. Up to 2s is fine.")

	s.Expose("slowRequest", export.Slowest(24*time.Hour))
	s.Expose("p50Ms24h", pctLazyMs(export, 0.5, s.Ago(24*time.Hour), s.Now))
	s.Expose("p95Ms24h", pctLazyMs(export, 0.95, s.Ago(24*time.Hour), s.Now))
	s.Expose("p99Ms24h", pctLazyMs(export, 0.99, s.Ago(24*time.Hour), s.Now))
	s.Expose("p95MsFirstWeek", pctLazyMs(export, 0.95, s.Ago(window), s.Ago(window-7*24*time.Hour)))
	s.Expose("allowanceMs", 2000)
	return nil
}
