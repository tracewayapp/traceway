package perf

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"time"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/scenarios"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/seed"
)

func init() { scenarios.Register("perf/app-sync-background", papSeedSyncBackground) }

// papSeedSyncBackground: POST /api/statements renders the statement PDF in
// the request (6-12s) and uploads it before answering, although the
// dashboard only polls GET /api/statements/{id} for the result. Distractor:
// host CPU jumps to 65-95% while a render runs, which looks like saturation,
// but the average is ~20%, the endpoint is just as slow when the host is
// otherwise idle, and every other endpoint is fast.
func papSeedSyncBackground(s *seed.Scenario) error {
	p := s.Project("billing", seed.OTel)
	api := p.Service("statements-api", seed.Go("nethttp", "sql")).Versions("1.12.4")
	window := 7 * 24 * time.Hour

	var renders []papInterval
	var sorted bool
	create := api.Endpoint("POST", "/api/statements")
	create.Traffic(seed.Daily(420), seed.Diurnal(), seed.Over(window))
	create.Detail(seed.DetailPolicy{Recent: 24 * time.Hour, Rate: 0.2, Max: 40})
	create.Handler(func(r *seed.Req) {
		r.Gap(r.Ms(0.6, 1.4))
		r.DB("SELECT id, legal_name, currency, billing_email FROM accounts WHERE id = $1", r.Ms(1.5, 3.5))
		invoices := 40 + r.IntN(400)
		r.DB("SELECT id, number, issued_at, due_at, total_cents, status FROM invoices WHERE account_id = $1 AND issued_at >= $2 AND issued_at < $3 ORDER BY issued_at",
			r.Ms(8, 24), seed.Rows(invoices))
		r.DB("INSERT INTO statements (id, account_id, period_start, period_end, status, created_at) VALUES ($1, $2, $3, $4, 'rendering', now())", r.Ms(2, 5))
		r.Gap(r.Ms(0.2, 0.6))
		pages := 2 + invoices/25
		render := time.Duration((6200 + float64(pages)*250) * (0.9 + 0.2*r.Float()) * float64(time.Millisecond))
		at := r.At()
		r.Compute("render statement pdf", render, seed.A("statement.pages", pages), seed.A("statement.invoices", invoices)).Scope = "statements-api"
		renders = append(renders, papInterval{at, at.Add(render)})
		sorted = false
		key := "statements/" + r.Start().Format("2006") + "/" + papStatementID(r) + ".pdf"
		r.HTTP("PUT", "https://acme-billing-statements.s3.eu-west-1.amazonaws.com/"+key, r.Ms(160, 420), 200,
			seed.A("http.request.body.size", 90000+pages*38000)).Name = "HTTP PUT"
		r.DB("UPDATE statements SET status = 'ready', object_key = $2, rendered_at = now() WHERE id = $1", r.Ms(2, 4))
		r.Gap(r.Ms(0.3, 0.9))
		r.Status(201)
	})

	poll := api.Endpoint("GET", "/api/statements/{id}")
	poll.Traffic(seed.Daily(470), seed.Diurnal(), seed.Over(window))
	poll.Handler(func(r *seed.Req) {
		r.RootAttr("url.path", "/api/statements/"+papStatementID(r))
		r.Gap(r.Ms(0.5, 1.2))
		r.DB("SELECT id, account_id, status, object_key, rendered_at FROM statements WHERE id = $1", r.Ms(1.5, 4))
		r.Gap(r.Ms(0.2, 0.6))
	})

	download := api.Endpoint("GET", "/api/statements/{id}/download")
	download.Traffic(seed.Daily(260), seed.Diurnal(), seed.Over(window))
	download.Handler(func(r *seed.Req) {
		r.RootAttr("url.path", "/api/statements/"+papStatementID(r)+"/download")
		r.Gap(r.Ms(0.5, 1.2))
		r.DB("SELECT id, account_id, status, object_key, rendered_at FROM statements WHERE id = $1", r.Ms(1.5, 3.5))
		r.Compute("presign object url", r.Ms(0.3, 0.9)).Scope = "statements-api"
		r.Status(302)
	})

	invoices := api.Endpoint("GET", "/api/invoices")
	invoices.Traffic(seed.Daily(500), seed.Diurnal(), seed.Over(window))
	invoices.Handler(func(r *seed.Req) {
		r.Gap(r.Ms(0.5, 1.2))
		r.DB("SELECT id, number, issued_at, total_cents, status FROM invoices WHERE account_id = $1 ORDER BY issued_at DESC LIMIT 50", r.Ms(4, 12), seed.Rows(50))
		r.Gap(r.Ms(0.5, 2))
	})

	s.Background(api, seed.TypicalTraffic(8))

	// The host has two vCPUs and a render keeps one of them busy, so a
	// sample taken while a render runs reads ~80%.
	renderLoad := func(t time.Time) float64 {
		if !sorted {
			sort.Slice(renders, func(i, j int) bool { return renders[i].from.Before(renders[j].from) })
			sorted = true
		}
		return 0.6 * float64(papActive(renders, t, 15*time.Second))
	}
	api.Host().CPU(seed.Plus(seed.Flat(0.14, 0.22), renderLoad))
	api.Host().Memory(seed.Flat(0.41, 0.47))

	s.Expose("slowRequest", create.Slowest(24*time.Hour))
	s.Expose("p50Ms24h", func() any { return create.Percentile(0.5, 24*time.Hour).Seconds() * 1000 })
	s.Expose("p95Ms24h", func() any { return create.Percentile(0.95, 24*time.Hour).Seconds() * 1000 })
	return nil
}

// papStatementID is a statement id as the service makes them (16 random
// bytes, hex), derived from the request so it uses no scenario randomness.
func papStatementID(r *seed.Req) string {
	h := sha256.Sum256([]byte(r.Start().Format(time.RFC3339Nano)))
	return hex.EncodeToString(h[:16])
}
