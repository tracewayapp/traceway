package perf

import (
	"time"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/scenarios"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/seed"
)

func init() { scenarios.Register("perf/ctl-onset-bisect", pctSeedOnsetBisect) }

// pctSeedOnsetBisect: GET /api/account/summary picked up a monthly-spend
// aggregate (orders JOIN order_items GROUP BY month) in 2.32.0, deployed 52h
// ago, and its p95 stepped from ~20-25ms to ~400-450ms. Six heavier endpoints
// (imports, reports, avatar uploads, recommendations, search, login) keep it
// out of the top 5 of endpoints chart for p95 and p50, so the onset has to be
// found by comparing `endpoints list --search` windows.
// Distractor: data growth; the orders count query has been creeping up all
// along, so a coarse before/after comparison looks like gradual growth.
func pctSeedOnsetBisect(s *seed.Scenario) error {
	p := s.Project("members", seed.OTel)
	window := 10 * 24 * time.Hour
	deployAt := s.Ago(52 * time.Hour)
	api := p.Service("members-api", seed.Python("fastapi", "sqlalchemy")).Versions("2.31.4", seed.Deploy{At: deployAt, Version: "2.32.0"})

	summary := api.Endpoint("GET", "/api/account/summary")
	summary.Traffic(seed.Daily(130), seed.Diurnal(), seed.Over(window))
	summary.Detail(seed.DetailPolicy{Recent: 30 * time.Hour, Rate: 0.12, Max: 14})
	summary.Handler(func(r *seed.Req) {
		r.Gap(r.Ms(0.8, 1.8))
		r.DB("SELECT accounts.id, accounts.email, accounts.display_name, accounts.plan, accounts.created_at FROM accounts WHERE accounts.id = %(id_1)s", r.LogNormal(2*time.Millisecond, 6*time.Millisecond), seed.Rows(1))
		// orders grows every day; the count slowly gets dearer.
		r.DB("SELECT count(*) AS count_1 FROM orders WHERE orders.account_id = %(account_id_1)s",
			time.Duration(r.Lerp(4, 11)*(0.8+0.5*r.Float())*float64(time.Millisecond)), seed.Rows(1))
		r.DB("SELECT orders.id, orders.status, orders.total_cents, orders.created_at FROM orders WHERE orders.account_id = %(account_id_1)s ORDER BY orders.created_at DESC LIMIT %(param_1)s",
			r.LogNormal(3*time.Millisecond, 9*time.Millisecond), seed.Rows(5))
		if r.Version() == "2.32.0" {
			r.DB("SELECT date_trunc(%(date_trunc_1)s, orders.created_at) AS month, count(DISTINCT orders.id) AS orders, sum(order_items.qty * order_items.unit_price_cents) AS spend_cents FROM orders JOIN order_items ON order_items.order_id = orders.id WHERE orders.account_id = %(account_id_1)s GROUP BY date_trunc(%(date_trunc_1)s, orders.created_at) ORDER BY month",
				r.LogNormal(260*time.Millisecond, 520*time.Millisecond), seed.Rows(12+r.IntN(13)))
		}
		r.Gap(r.Ms(0.8, 2.5))
		r.Gap(r.Ms(0.3, 0.9))
	})

	imports := api.Endpoint("POST", "/api/imports/contacts")
	imports.Traffic(seed.Daily(27), seed.Diurnal(), seed.Over(window))
	imports.Handler(func(r *seed.Req) {
		r.Gap(r.Ms(1, 3))
		r.Compute("parse csv", r.LogNormal(1100*time.Millisecond, 2600*time.Millisecond)).Scope = "members-api"
		r.DB("INSERT INTO contacts (account_id, email, name, tags) VALUES (%(account_id_m0)s, %(email_m0)s, %(name_m0)s, %(tags_m0)s::VARCHAR[])", r.LogNormal(300*time.Millisecond, 700*time.Millisecond))
	})
	reports := api.Endpoint("GET", "/api/reports/activity")
	reports.Traffic(seed.Daily(90), seed.Diurnal(), seed.Over(window))
	reports.Handler(func(r *seed.Req) {
		r.Gap(r.Ms(0.8, 1.8))
		r.DB("SELECT activity_events.kind, count(*) AS count_1 FROM activity_events WHERE activity_events.account_id = %(account_id_1)s AND activity_events.created_at >= %(created_at_1)s GROUP BY activity_events.kind", r.LogNormal(700*time.Millisecond, 1500*time.Millisecond), seed.Rows(9))
		r.Gap(r.Ms(1, 3))
	})
	avatar := api.Endpoint("POST", "/api/media/avatar")
	avatar.Traffic(seed.Daily(20), seed.Diurnal(), seed.Over(window))
	avatar.Handler(func(r *seed.Req) {
		r.Gap(r.Ms(2, 6))
		r.Compute("resize image", r.LogNormal(650*time.Millisecond, 1300*time.Millisecond)).Scope = "members-api"
		r.DB("UPDATE accounts SET avatar_key=%(avatar_key)s WHERE accounts.id = %(id_1)s", r.LogNormal(3*time.Millisecond, 9*time.Millisecond))
	})
	recs := api.Endpoint("GET", "/api/recommendations")
	recs.Traffic(seed.Daily(300), seed.Diurnal(), seed.Over(window))
	recs.Handler(func(r *seed.Req) {
		r.Gap(r.Ms(0.8, 1.8))
		r.HTTP("POST", "http://recs-model.internal:8501/v1/models/members:predict", r.LogNormal(430*time.Millisecond, 1100*time.Millisecond), 200)
		r.Gap(r.Ms(0.5, 2))
	})
	search := api.Endpoint("GET", "/api/search")
	search.Traffic(seed.Daily(360), seed.Diurnal(), seed.Over(window))
	search.Handler(func(r *seed.Req) {
		r.Gap(r.Ms(0.8, 1.8))
		r.DB("SELECT contacts.id, contacts.name, contacts.email FROM contacts WHERE contacts.account_id = %(account_id_1)s AND (contacts.search_vector @@ plainto_tsquery(%(plainto_tsquery_1)s)) ORDER BY ts_rank(contacts.search_vector, plainto_tsquery(%(plainto_tsquery_1)s)) DESC LIMIT %(param_1)s",
			r.LogNormal(390*time.Millisecond, 900*time.Millisecond), seed.Rows(20))
		r.Gap(r.Ms(0.5, 2))
	})
	login := api.Endpoint("POST", "/api/auth/login")
	login.Traffic(seed.Daily(260), seed.Diurnal(), seed.Over(window))
	login.Handler(func(r *seed.Req) {
		r.Gap(r.Ms(0.8, 1.8))
		r.DB("SELECT accounts.id, accounts.password_hash FROM accounts WHERE accounts.email = %(email_1)s", r.LogNormal(2*time.Millisecond, 6*time.Millisecond), seed.Rows(1))
		r.Compute("bcrypt.checkpw", r.Ms(400, 540)).Scope = "members-api"
		r.DB("INSERT INTO sessions (account_id, token_hash, expires_at) VALUES (%(account_id)s, %(token_hash)s, %(expires_at)s) RETURNING sessions.id", r.LogNormal(3*time.Millisecond, 9*time.Millisecond))
	})
	s.Background(api, seed.TypicalTraffic(6))
	api.Host().CPU(seed.Flat(0.24, 0.36))
	api.Host().Memory(seed.Flat(0.55, 0.62))

	s.Expose("deployAt", pctRFC3339(deployAt))
	s.Expose("slowRequest", summary.Slowest(24*time.Hour))
	s.Expose("p95MsBefore24h", pctLazyMs(summary, 0.95, deployAt.Add(-24*time.Hour), deployAt))
	s.Expose("p95MsAfter24h", pctLazyMs(summary, 0.95, deployAt, deployAt.Add(24*time.Hour)))
	s.Expose("p95MsFirstDays", pctLazyMs(summary, 0.95, s.Ago(window), s.Ago(window-3*24*time.Hour)))
	// Window edges the oracle's bisection uses, as "<hours>h ago".
	for _, h := range []float64{240, 120, 60, 54, 52.5, 51, 48} {
		s.Expose(pctAgoKey(h), pctRFC3339(s.Now.Add(-time.Duration(h*float64(time.Hour)))))
	}
	s.Expose("primaryProject", "members")
	return nil
}
