package perf

import (
	"time"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/scenarios"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/seed"
)

func init() { scenarios.Register("perf/ctl-marked-slow-regressed", pctSeedMarkedSlowRegressed) }

// pctSeedMarkedSlowRegressed: the monthly invoice export is marked slow with
// a 2000ms allowance and sat at p95 ~1.5s until v4.8.0 (26h ago) added a tax
// breakdown that loads each invoice's tax lines with its own query. Since
// the deploy p95 is ~4s, well past the allowance. Distractors: the marking
// itself (the latency looks "accepted"), and the big invoices/lines JOIN,
// which is unchanged by the deploy.
func pctSeedMarkedSlowRegressed(s *seed.Scenario) error {
	p := s.Project("invoicing", seed.OTel)
	window := 14 * 24 * time.Hour
	deployAt := s.Ago(26 * time.Hour)
	api := p.Service("invoicing-api", seed.Go("nethttp", "sql")).Versions("4.7.3", seed.Deploy{At: deployAt, Version: "4.8.0"})

	export := api.Endpoint("GET", "/api/exports/invoices")
	export.Traffic(seed.Daily(110), seed.Diurnal(), seed.Over(window))
	export.Detail(seed.DetailPolicy{Recent: 20 * time.Hour, Rate: 0.3, Max: 14})
	export.Handler(func(r *seed.Req) {
		// Invoices in the month: most customers 60-100, large ones 112-125,
		// each with 8-12 lines.
		n := 60 + r.IntN(41)
		if r.Chance(0.12) {
			n = 112 + r.IntN(14)
		}
		lines := n * (8 + r.IntN(5))
		r.Gap(r.Ms(0.5, 1.2))
		r.DB("SELECT i.id, i.number, i.issued_at, i.customer_name, i.total_cents, l.sku, l.description, l.qty, l.unit_price_cents FROM invoices i JOIN invoice_lines l ON l.invoice_id = i.id WHERE i.org_id = $1 AND i.issued_at >= $2 AND i.issued_at < $3 ORDER BY i.issued_at, i.id",
			time.Duration(float64(lines)*0.95*(0.95+0.1*r.Float())*float64(time.Millisecond)), seed.Rows(lines))
		if r.Version() == "4.8.0" {
			for range n {
				r.DB("SELECT rate_code, rate_bps, amount_cents FROM invoice_tax_lines WHERE invoice_id = $1", r.Ms(18, 27), seed.Rows(1+r.IntN(3)))
			}
		}
		r.Compute("encode csv", time.Duration(float64(lines)*0.28*(0.95+0.1*r.Float())*float64(time.Millisecond)))
		r.Gap(r.Ms(0.3, 0.9))
	})

	list := api.Endpoint("GET", "/api/invoices")
	list.Traffic(seed.Daily(240), seed.Diurnal(), seed.Over(window))
	list.Handler(func(r *seed.Req) {
		r.Gap(r.Ms(0.4, 1.0))
		r.DB("SELECT id, number, issued_at, customer_name, total_cents, status FROM invoices WHERE org_id = $1 ORDER BY issued_at DESC LIMIT 50", r.LogNormal(5*time.Millisecond, 20*time.Millisecond), seed.Rows(50))
		r.Compute("encode response", r.Ms(0.5, 1.8))
	})
	get := api.Endpoint("GET", "/api/invoices/:id")
	get.Traffic(seed.Daily(320), seed.Diurnal(), seed.Over(window))
	get.Handler(func(r *seed.Req) {
		r.Gap(r.Ms(0.4, 1.0))
		r.DB("SELECT id, number, issued_at, customer_name, total_cents, status FROM invoices WHERE id = $1", r.LogNormal(2*time.Millisecond, 8*time.Millisecond))
		r.DB("SELECT sku, description, qty, unit_price_cents FROM invoice_lines WHERE invoice_id = $1", r.LogNormal(2*time.Millisecond, 9*time.Millisecond))
		r.DB("SELECT rate_code, rate_bps, amount_cents FROM invoice_tax_lines WHERE invoice_id = $1", r.LogNormal(2*time.Millisecond, 8*time.Millisecond))
		r.Compute("encode response", r.Ms(0.3, 1.0))
	})
	create := api.Endpoint("POST", "/api/invoices")
	create.Traffic(seed.Daily(90), seed.Diurnal(), seed.Over(window))
	create.Handler(func(r *seed.Req) {
		r.Gap(r.Ms(0.5, 1.4))
		r.DB("INSERT INTO invoices (org_id, number, issued_at, customer_name, total_cents, status) VALUES ($1, $2, $3, $4, $5, 'open') RETURNING id", r.LogNormal(4*time.Millisecond, 14*time.Millisecond))
		r.DB("INSERT INTO invoice_lines (invoice_id, sku, description, qty, unit_price_cents) SELECT $1, * FROM unnest($2::text[], $3::text[], $4::int[], $5::bigint[])", r.LogNormal(4*time.Millisecond, 15*time.Millisecond))
		r.DB("INSERT INTO invoice_tax_lines (invoice_id, rate_code, rate_bps, amount_cents) SELECT $1, * FROM unnest($2::text[], $3::int[], $4::bigint[])", r.LogNormal(3*time.Millisecond, 12*time.Millisecond))
	})
	s.Background(api, seed.TypicalTraffic(6))
	api.Host().CPU(seed.Flat(0.22, 0.34))
	api.Host().Memory(seed.Flat(0.45, 0.53))

	p.MarkSlow("GET /api/exports/invoices", 2000, "Monthly invoice CSV for accounting: every invoice of the month with its lines. Up to 2s is expected.")

	s.Expose("deployAt", pctRFC3339(deployAt))
	s.Expose("slowRequest", export.Slowest(20*time.Hour))
	s.Expose("beforeRequest", pctRef(func() *seed.Request {
		return pctNearestBetween(export, deployAt.Add(-20*time.Hour), deployAt, export.PercentileBetween(0.9, s.Ago(window), deployAt))
	}))
	s.Expose("p95MsBefore", pctLazyMs(export, 0.95, s.Ago(window), deployAt))
	s.Expose("p95MsAfter", pctLazyMs(export, 0.95, deployAt, s.Now))
	s.Expose("p50MsAfter", pctLazyMs(export, 0.5, deployAt, s.Now))
	s.Expose("p95Ms24h", pctLazyMs(export, 0.95, s.Ago(24*time.Hour), s.Now))
	return nil
}
