package perf

import (
	"time"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/scenarios"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/seed"
)

func init() { scenarios.Register("perf/db-unbounded-result", pdbSeedUnboundedResult) }

// pdbWebMvcScope reports the controller spans the Java agent records with
// controller telemetry on (ClassName.method around the handler invocation).
const pdbWebMvcScope = "io.opentelemetry.spring-webmvc-6.0"

func pdbController(r *seed.Req, name string, f func(r *seed.Req)) {
	sp := r.Span(name, f)
	sp.Scope = pdbWebMvcScope
}

// pdbSeedUnboundedResult: the admin audit log returns every event an
// organization ever recorded, with no LIMIT. Events accrue every day, so the
// rows each request returns (db.response.returned_rows) grow week over week,
// the query span and the controller's serialization grow with them at a
// constant cost per row, and p50 creeps up with no deploy. Distractor: a
// missing index (the user's guess). The dashboard widget reads the same table
// through the same (org_id, created_at) index with LIMIT 20 in ~2ms, and the
// big query's cost per row returned stays flat while the rows grow.
func pdbSeedUnboundedResult(s *seed.Scenario) error {
	window := 29 * 24 * time.Hour
	const roleSQL = "SELECT m.role FROM org_members m WHERE m.org_id = ? AND m.user_id = ?"
	p := s.Project("ledgerly", seed.OTel)
	api := p.Service("admin-api", seed.Java("spring")).Versions("3.18.1")

	// An organization's event count at the window start; events keep
	// accruing at a rate proportional to its size.
	orgEvents := func(r *seed.Req) float64 {
		switch x := r.Float(); {
		case x < 0.6:
			return 1500 + 4500*r.Float()
		case x < 0.9:
			return 8000 + 12000*r.Float()
		default:
			return 30000 + 30000*r.Float()
		}
	}

	audit := api.Endpoint("GET", "/api/admin/audit-log")
	audit.Traffic(seed.Daily(110), seed.Diurnal(), seed.Over(window))
	// Detail is chosen here rather than by a DetailPolicy (whose cap counts
	// forced requests): a quarter of the last 12 hours, plus a few traces
	// from every week.
	audit.Detail(seed.DetailPolicy{})
	recentCut := s.Ago(12 * time.Hour)
	audit.Handler(func(r *seed.Req) {
		n := int(orgEvents(r) * (1 + 1.3*r.Progress()))
		if r.Chance(0.012) || (!r.Start().Before(recentCut) && r.Chance(0.25)) {
			r.KeepDetail()
		}
		r.Gap(r.Ms(0.6, 1.8)) // Spring Security filter chain
		pdbController(r, "AuditLogController.list", func(r *seed.Req) {
			r.DB(roleSQL, r.Ms(0.5, 1.4), seed.Rows(1))
			perRow := 0.017 + 0.003*r.Float()
			r.DB("SELECT e.id, e.actor_id, e.action, e.target_type, e.target_id, e.ip_address, e.user_agent, e.created_at FROM audit_events e WHERE e.org_id = ? ORDER BY e.created_at DESC",
				pdbDur(1.5+perRow*float64(n)), seed.Rows(n))
			r.Gap(pdbDur(0.011 * float64(n) * (0.9 + 0.2*r.Float()))) // map rows, Jackson writes the JSON array
		})
		r.Gap(r.Ms(0.2, 0.6))
	})

	recent := api.Endpoint("GET", "/api/admin/dashboard/activity")
	recent.Traffic(seed.Daily(140), seed.Diurnal(), seed.Over(window))
	recent.Detail(seed.DetailPolicy{Recent: 12 * time.Hour, Rate: 0.04, Max: 4})
	recent.Handler(func(r *seed.Req) {
		r.Gap(r.Ms(0.6, 1.8))
		pdbController(r, "AuditLogController.activity", func(r *seed.Req) {
			r.DB(roleSQL, r.Ms(0.5, 1.4), seed.Rows(1))
			r.DB("SELECT e.id, e.actor_id, e.action, e.created_at FROM audit_events e WHERE e.org_id = ? ORDER BY e.created_at DESC LIMIT 20",
				r.Ms(1.1, 2.6), seed.Rows(20))
			r.Gap(r.Ms(0.3, 0.8))
		})
		r.Gap(r.Ms(0.2, 0.6))
	})

	settings := api.Endpoint("PUT", "/api/admin/settings")
	settings.Traffic(seed.Daily(18), seed.Diurnal(), seed.Over(window))
	settings.Handler(func(r *seed.Req) {
		r.Gap(r.Ms(0.6, 1.8))
		pdbController(r, "SettingsController.update", func(r *seed.Req) {
			r.DB(roleSQL, r.Ms(0.5, 1.4), seed.Rows(1))
			r.DB("UPDATE org_settings SET locale = ?, timezone = ?, invoice_prefix = ?, updated_at = now() WHERE org_id = ?", r.Ms(1.0, 2.5))
			r.DB("INSERT INTO audit_events (org_id, actor_id, action, target_type, target_id, ip_address, user_agent, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, now())", r.Ms(0.8, 2.0))
		})
		r.Gap(r.Ms(0.2, 0.6))
		r.Status(204)
	})

	for _, rt := range []struct {
		route, controller, query string
		perDay                   int
		dbMedianMs               float64
	}{
		{"/api/admin/members", "MembersController.list", "SELECT u.id, u.email, u.full_name, m.role, m.joined_at FROM org_members m JOIN users u ON u.id = m.user_id WHERE m.org_id = ? ORDER BY u.full_name", 50, 2.5},
		{"/api/admin/settings", "SettingsController.get", "SELECT s.locale, s.timezone, s.invoice_prefix, s.updated_at FROM org_settings s WHERE s.org_id = ?", 35, 1},
		{"/api/admin/invoices", "InvoicesController.list", "SELECT i.id, i.number, i.status, i.total_cents, i.issued_at FROM invoices i WHERE i.org_id = ? ORDER BY i.issued_at DESC LIMIT 50", 30, 2},
	} {
		rt := rt
		e := api.Endpoint("GET", rt.route)
		e.Traffic(seed.Daily(rt.perDay), seed.Diurnal(), seed.Over(window))
		e.Detail(seed.DetailPolicy{Recent: 6 * time.Hour, Rate: 0.02, Max: 4})
		e.Handler(func(r *seed.Req) {
			r.Gap(r.Ms(0.6, 1.8))
			pdbController(r, rt.controller, func(r *seed.Req) {
				m := pdbDur(rt.dbMedianMs)
				r.DB(rt.query, r.LogNormal(m, m*5))
				r.Gap(r.Ms(0.2, 1.4))
			})
			r.Gap(r.Ms(0.1, 0.5))
		})
	}
	api.Host().CPU(seed.Flat(0.19, 0.28))
	api.Host().Memory(seed.Flat(0.56, 0.62))

	s.Expose("slowRequest", pdbSlowest(audit, s.Ago(24*time.Hour), nil))
	// The newest widget request keeps its trace and access log, so a fast
	// LIMIT 20 query to compare with is there at any hour; overnight the 4%
	// sample alone can leave the last six hours without one.
	s.Expose("widgetRequest", recent.Pick(func(*seed.Request) bool { return true }))
	s.Expose("p50Ms24h", pdbMs(audit, 0.5, s.Ago(24*time.Hour), s.Now))
	s.Expose("p95Ms24h", pdbMs(audit, 0.95, s.Ago(24*time.Hour), s.Now))
	s.Expose("p50MsFirstWeek", pdbMs(audit, 0.5, s.Ago(window), s.Ago(window-7*24*time.Hour)))
	return nil
}
