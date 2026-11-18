package write

import (
	"fmt"
	"time"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/scenarios"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/seed"
)

func init() { scenarios.Register("write/page-resolve-archive", wrSeedPageResolveArchive) }

const wrPriceCentsStack = "TypeError: Cannot read properties of undefined (reading 'priceCents')\n" +
	"    at applyPromotion (/app/src/pricing/promotions.js:42:31)\n" +
	"    at Array.map (<anonymous>)\n" +
	"    at priceCart (/app/src/pricing/cart.js:18:24)\n" +
	"    at async addItem (/app/src/routes/cart.js:57:23)"

const wrBeaconStack = "SyntaxError: Unexpected end of JSON input\n" +
	"    at JSON.parse (<anonymous>)\n" +
	"    at parse (/app/node_modules/body-parser/lib/types/json.js:92:19)\n" +
	"    at /app/node_modules/body-parser/lib/read.js:128:18\n" +
	"    at AsyncResource.runInAsyncScope (node:async_hooks:206:9)"

// wrSeedPageResolveArchive: cart-api 2.8.0 throws a TypeError for carts with
// a promotion; the new-error rule pages on-call, the user rolls back to
// 2.7.4 within a minute, and traffic is clean afterwards. The agent should
// confirm that, ask, then resolve the page and archive that one issue.
// Distractors: an open latency page for recommendations and an ignored,
// still-occurring beacon JSON error that must stay unarchived.
//
// The new-error page opens when the first TypeError is ingested, so the
// error window is laid out around ingest time: errors from 100 seconds
// before the seed starts until the rollback 25 seconds after it (the page
// opens in between), then clean traffic until 2.5 minutes after it, which
// the seed waits out.
func wrSeedPageResolveArchive(s *seed.Scenario) error {
	r0 := wrClock(s)
	badDeploy := r0.Add(-100 * time.Second)
	rollback := r0.Add(25 * time.Second)
	end := r0.Add(150 * time.Second)
	wrEndTelemetryAt(s, end)

	p := s.Project("storefront", seed.OTel)
	paging := p.PagingChannel("Storefront on-call")
	errRule := p.Rule(seed.Rule{Name: "New errors in storefront", Type: "new_error", Channel: paging, Cooldown: 60, Severity: "critical",
		Config: map[string]any{"ignorePatterns": []string{"SyntaxError"}}})
	api := p.Service("cart-api", seed.Node("express", "pg")).Versions("2.7.4",
		seed.Deploy{At: badDeploy, Version: "2.8.0"}, seed.Deploy{At: rollback, Version: "2.7.4"})

	var errors, afterRollback int
	var lastError time.Time
	items := api.Endpoint("POST", "/api/cart/items")
	items.Traffic(seed.Daily(20000), seed.Diurnal(), seed.Over(6*time.Hour))
	items.Detail(seed.DetailPolicy{Recent: 30 * time.Minute, Rate: 0.08, Max: 30})
	items.Handler(func(r *seed.Req) {
		r.Gap(r.Ms(0.4, 1.1))
		r.DB("SELECT id, user_id, promotion_id, currency FROM carts WHERE id = $1", r.Ms(1, 4))
		r.DB("SELECT sku, name, price_cents FROM products WHERE sku = $1", r.Ms(1, 3))
		promo := r.Chance(0.45)
		if promo {
			r.DB("SELECT id, kind, percent_off, sku FROM promotions WHERE id = $1", r.Ms(1, 3))
		}
		if !r.Start().Before(rollback) {
			afterRollback++
		}
		if r.Version() == "2.8.0" && promo {
			r.Compute("price cart", r.Ms(0.2, 0.8))
			r.Error(seed.Exc{Type: "TypeError", Message: "Cannot read properties of undefined (reading 'priceCents')", Stack: wrPriceCentsStack, Escaped: true})
			r.Log(seed.SevError, "Unhandled error in POST /api/cart/items", seed.A("error.type", "TypeError"))
			r.Status(500)
			errors++
			if r.Start().After(lastError) {
				lastError = r.Start()
			}
			return
		}
		r.Compute("price cart", r.Ms(0.5, 2))
		r.DB("INSERT INTO cart_items (cart_id, sku, qty, unit_price_cents) VALUES ($1, $2, $3, $4) ON CONFLICT (cart_id, sku) DO UPDATE SET qty = cart_items.qty + EXCLUDED.qty", r.Ms(2, 5))
		r.Status(201)
		r.Gap(r.Ms(0.2, 0.6))
	})

	events := api.Endpoint("POST", "/api/events")
	events.Traffic(seed.Daily(4000), seed.Diurnal(), seed.Over(6*time.Hour))
	events.Detail(seed.DetailPolicy{Recent: time.Hour, Rate: 0.01, Max: 5})
	events.Handler(func(r *seed.Req) {
		r.Gap(r.Ms(0.2, 0.6))
		if r.Chance(0.012) {
			r.Error(seed.Exc{Type: "SyntaxError", Message: "Unexpected end of JSON input", Stack: wrBeaconStack})
			r.Status(400)
			return
		}
		r.DB("INSERT INTO analytics_events (id, session_id, name, payload, created_at) VALUES ($1, $2, $3, $4, now())", r.Ms(1, 3))
		r.Status(202)
	})

	recs := api.Endpoint("GET", "/api/recommendations")
	recs.Traffic(seed.Daily(2880), seed.Diurnal(), seed.Over(6*time.Hour))
	recs.Detail(seed.DetailPolicy{Recent: time.Hour, Rate: 0.05, Max: 10})
	recs.Handler(func(r *seed.Req) {
		r.Gap(r.Ms(0.3, 0.9))
		slow := r.Start().After(r0.Add(-40 * time.Minute))
		d := r.LogNormal(180*time.Millisecond, 600*time.Millisecond)
		if slow {
			d = r.LogNormal(1700*time.Millisecond, 3200*time.Millisecond)
		}
		r.HTTP("GET", "https://recs.internal.example/v1/recommendations", d, 200)
		r.Compute("encode response", r.Ms(0.5, 2))
	})
	recRule := p.Rule(seed.Rule{Name: "Recommendations p95 above 1.5s", Type: "endpoint_p95_threshold", Channel: paging, Cooldown: 30, Severity: "warning",
		Config: map[string]any{"endpoint": "GET /api/recommendations", "thresholdMs": 1500, "lookbackMinutes": 15}})

	s.Background(api, seed.TrafficOpts{Endpoints: 6, PerDay: 150, NoErrors: true})
	s.Expose("rollbackAt", rollback.Format(time.RFC3339))

	s.AfterFlush(func() error {
		pg, err := p.WaitPage(errRule, 90*time.Second)
		if err != nil {
			return err
		}
		other, err := p.WaitPage(recRule, 90*time.Second)
		if err != nil {
			return err
		}
		hash, err := p.GroupMatching(24*time.Hour, "reading 'priceCents'")
		if err != nil {
			return err
		}
		beacon, err := p.GroupMatching(24*time.Hour, "Unexpected end of JSON input")
		if err != nil {
			return err
		}
		if !s.Sink.Dry() && pg.IssueHash != hash {
			return fmt.Errorf("page %d is for issue %q, want %q", pg.ID, pg.IssueHash, hash)
		}
		s.Expose("pageId", pg.ID)
		s.Expose("otherPageId", other.ID)
		s.Expose("hash", hash)
		s.Expose("otherHash", beacon)
		s.Expose("errorCount", errors)
		s.Expose("lastErrorAt", lastError.UTC().Format(time.RFC3339))
		s.Expose("requestsAfterRollback", afterRollback)
		wrSleepUntil(s, end.Add(3*time.Second))
		return nil
	})
	return nil
}
