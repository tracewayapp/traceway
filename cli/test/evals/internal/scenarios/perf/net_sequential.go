package perf

import (
	"fmt"
	"time"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/scenarios"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/seed"
)

func init() { scenarios.Register("perf/net-sequential", pncSeedSequential) }

// pncNestScope is the scope of the NestJS controller/handler spans.
const pncNestScope = "@opentelemetry/instrumentation-nestjs-core"

// pncNest wraps a NestJS route the way @opentelemetry/instrumentation-nestjs-core
// does: a request_context span for Controller.method with the handler span
// inside it.
func pncNest(r *seed.Req, controller, method, route string, body func(r *seed.Req)) {
	ctx := r.Span(controller+"."+method, func(r *seed.Req) {
		h := r.Span(method, body, seed.A("nestjs.type", "handler"), seed.A("nestjs.callback", method))
		h.Scope = pncNestScope
	}, seed.A("nestjs.type", "request_context"), seed.A("nestjs.controller", controller), seed.A("nestjs.callback", method),
		seed.A("http.route", route), seed.A("component", "@nestjs/core"), seed.A("nestjs.version", "11.1.6"))
	ctx.Scope = pncNestScope
}

// pncCall is r.Call with the concrete path and query the caller really
// requested, on both the CLIENT span and the downstream SERVER span.
func pncCall(r *seed.Req, target *seed.Endpoint, rtt time.Duration, path, query string) *seed.Span {
	c := r.Call(target, rtt)
	full := "http://" + target.Service().Name + ".internal:8080" + path
	if query != "" {
		full += "?" + query
	}
	c.Attrs["url.full"] = full
	if len(c.Children) > 0 {
		c.Children[0].Attrs["url.path"] = path
		if query != "" {
			c.Children[0].Attrs["url.query"] = query
		}
	}
	return c
}

// pncSeedSequential: the portal's dashboard endpoint (NestJS) calls four
// internal services one after another. Each service is fast and steady on
// its own; the dashboard pays their sum. Distractor: recommendations is the
// longest hop, but it is an internal service in line with its own p95, and no
// hop dominates.
func pncSeedSequential(s *seed.Scenario) error {
	p := s.Project("portal", seed.OTel)
	window := 3 * 24 * time.Hour

	web := p.Service("web-api", seed.Node("nestjs", "pg")).Versions("3.7.0")
	accounts := p.Service("accounts-service", seed.Go("chi", "pgx")).Versions("1.22.4")
	billing := p.Service("billing-service", seed.Java("spring-boot")).Versions("5.3.1")
	notify := p.Service("notifications-service", seed.Node("express", "pg")).Versions("2.0.9")
	reco := p.Service("recommendations-service", seed.Python("fastapi", "psycopg")).Versions("0.41.2")

	profile := accounts.Endpoint("GET", "/internal/accounts/{id}/profile")
	profile.Handler(func(r *seed.Req) {
		r.Gap(r.Ms(0.2, 0.6))
		r.DB("SELECT a.id, a.name, a.plan, p.seats FROM accounts a JOIN plans p ON p.id = a.plan_id WHERE a.id = $1", r.LogNormal(20*time.Millisecond, 45*time.Millisecond))
		r.DB("SELECT user_id, role FROM account_members WHERE account_id = $1", r.LogNormal(13*time.Millisecond, 32*time.Millisecond), seed.Rows(4+r.IntN(20)))
		r.DB("SELECT flag_key, enabled FROM feature_flags WHERE account_id = $1", r.LogNormal(9*time.Millisecond, 22*time.Millisecond), seed.Rows(18))
		r.Compute("encode response", r.Ms(1, 3))
	})
	summary := billing.Endpoint("GET", "/internal/billing/{accountId}/summary")
	summary.Handler(func(r *seed.Req) {
		r.Gap(r.Ms(0.5, 1.5))
		r.DB("SELECT id, number, status, total_cents, due_at FROM invoices WHERE account_id = ? ORDER BY due_at DESC LIMIT 3", r.LogNormal(18*time.Millisecond, 42*time.Millisecond), seed.Rows(3))
		r.DB("SELECT SUM(amount_cents) FROM ledger_entries WHERE account_id = ? AND posted_at >= ?", r.LogNormal(52*time.Millisecond, 110*time.Millisecond))
		r.Compute("BillingSummaryMapper.toDto", r.Ms(1, 3))
	})
	unread := notify.Endpoint("GET", "/internal/notifications/unread")
	unread.Handler(func(r *seed.Req) {
		r.Gap(r.Ms(0.3, 0.8))
		r.DB("SELECT count(*) FROM notifications WHERE account_id = $1 AND read_at IS NULL", r.LogNormal(22*time.Millisecond, 50*time.Millisecond))
		r.DB("SELECT id, kind, title, created_at FROM notifications WHERE account_id = $1 AND read_at IS NULL ORDER BY created_at DESC LIMIT 5", r.LogNormal(16*time.Millisecond, 38*time.Millisecond), seed.Rows(5))
		r.Compute("serialize", r.Ms(0.5, 2))
	})
	suggest := reco.Endpoint("GET", "/internal/recommendations")
	suggest.Handler(func(r *seed.Req) {
		r.Gap(r.Ms(0.4, 1.2))
		r.Cache("GET", "reco:features", r.Ms(0.6, 1.8))
		r.DB("SELECT item_id, category, popularity FROM catalog_items WHERE category = ANY(%s) AND active LIMIT 400", r.LogNormal(24*time.Millisecond, 55*time.Millisecond), seed.Rows(400))
		r.Compute("score_candidates", r.LogNormal(72*time.Millisecond, 130*time.Millisecond))
	})

	dash := web.Endpoint("GET", "/api/dashboard")
	dash.Traffic(seed.Daily(800), seed.Diurnal(), seed.Over(window))
	dash.Detail(seed.DetailPolicy{Recent: 12 * time.Hour, Rate: 0.05, Max: 12})
	dash.Handler(func(r *seed.Req) {
		acct := fmt.Sprint(20000 + r.IntN(60000))
		r.Gap(r.Ms(0.6, 1.6)) // auth guard
		pncNest(r, "DashboardController", "getDashboard", "/api/dashboard", func(r *seed.Req) {
			pncCall(r, profile, r.Ms(1.2, 3), "/internal/accounts/"+acct+"/profile", "")
			pncCall(r, summary, r.Ms(1.2, 3), "/internal/billing/"+acct+"/summary", "")
			pncCall(r, unread, r.Ms(1.2, 3), "/internal/notifications/unread", "accountId="+acct)
			pncCall(r, suggest, r.Ms(1.2, 3), "/internal/recommendations", "accountId="+acct+"&limit=8")
			c := r.Compute("DashboardPresenter.compose", r.Ms(2, 6))
			c.Scope = "web-api"
		})
		r.Gap(r.Ms(0.3, 0.8))
	})

	s.Background(web, seed.TrafficOpts{Endpoints: 4, PerDay: 150})
	web.Host().CPU(seed.Flat(0.16, 0.27))
	web.Host().Memory(seed.Flat(0.44, 0.52))

	s.Expose("slowRequest", dash.Slowest(24*time.Hour))
	s.Expose("p50Ms24h", func() any { return dash.Percentile(0.5, 24*time.Hour).Seconds() * 1000 })
	s.Expose("p95Ms24h", func() any { return dash.Percentile(0.95, 24*time.Hour).Seconds() * 1000 })
	return nil
}
