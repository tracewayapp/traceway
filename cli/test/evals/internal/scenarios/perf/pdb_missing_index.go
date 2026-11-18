package perf

import (
	"time"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/scenarios"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/seed"
)

func init() { scenarios.Register("perf/db-missing-index", pdbSeedMissingIndex) }

// Session.get loads by primary key with table-prefixed column labels.
const (
	pdbGetUser = "SELECT users.id AS users_id, users.email AS users_email, users.display_name AS users_display_name, users.org_id AS users_org_id, users.created_at AS users_created_at FROM users WHERE users.id = $1::INTEGER"
	pdbGetOrg  = "SELECT orgs.id AS orgs_id, orgs.name AS orgs_name, orgs.plan AS orgs_plan FROM orgs WHERE orgs.id = $1::INTEGER"
)

// pdbSeedMissingIndex: the user lookup filters users by email, a column with
// no index, so Postgres reads the whole table on every call. The table grows
// steadily with signups, and so does the one query span, for every request
// (it returns one row or none). Primary-key lookups on the same table stay at
// a millisecond or two. Distractor: release 1.13.0 two weeks ago changed the
// invite endpoint (two extra primary-key reads and an email enqueue); the
// lookup curve has no step there and the release did not touch it.
func pdbSeedMissingIndex(s *seed.Scenario) error {
	window := 29 * 24 * time.Hour
	deployAt := s.Ago(14*24*time.Hour + 2*time.Hour) // two hours after the head commit
	p := s.Project("teamspace", seed.OTel)
	api := p.Service("accounts-api", seed.Python("fastapi", "sqlalchemy")).
		Versions("1.12.4", seed.Deploy{At: deployAt, Version: "1.13.0"})

	lookup := api.Endpoint("GET", "/api/users/lookup")
	lookup.Traffic(seed.Daily(160), seed.Diurnal(), seed.Over(window))
	lookup.Detail(seed.DetailPolicy{Recent: 12 * time.Hour, Rate: 0.25, Max: 12})
	lookup.Handler(func(r *seed.Req) {
		users := r.Lerp(1.62e6, 3.94e6) // rows in users; signups accrue steadily
		r.Gap(r.Ms(0.6, 1.6))           // session dependency
		found := !r.Chance(0.12)
		rows := 0
		if found {
			rows = 1
		}
		// A sequential scan reads every row whether or not one matches.
		scan := pdbDur(users / 7600 * (0.93 + 0.14*r.Float()))
		r.DB("SELECT users.id, users.email, users.display_name, users.org_id, users.created_at FROM users WHERE users.email = $1::VARCHAR",
			scan, seed.Rows(rows))
		if !found {
			r.Status(404)
			r.Gap(r.Ms(0.2, 0.6))
			return
		}
		r.DB(pdbGetOrg, r.Ms(0.7, 2.2), seed.Rows(1))
		r.Gap(r.Ms(0.3, 0.9)) // pydantic response model
	})

	invites := api.Endpoint("POST", "/api/invites")
	invites.Traffic(seed.Daily(50), seed.Diurnal(), seed.Over(window))
	invites.Detail(seed.DetailPolicy{Recent: 12 * time.Hour, Rate: 0.1, Max: 4})
	invites.Handler(func(r *seed.Req) {
		r.Gap(r.Ms(0.6, 1.6))
		r.DB("SELECT org_members.role FROM org_members WHERE org_members.org_id = $1::INTEGER AND org_members.user_id = $2::INTEGER", r.Ms(0.7, 2.0), seed.Rows(1))
		r.DB("INSERT INTO invites (org_id, email, role, token, invited_by, created_at, accepted_at) VALUES ($1::INTEGER, $2::VARCHAR, $3::VARCHAR, $4::VARCHAR, $5::INTEGER, $6::TIMESTAMP WITHOUT TIME ZONE, $7::TIMESTAMP WITHOUT TIME ZONE) RETURNING invites.id",
			r.Ms(1.4, 3.8))
		if r.Version() == "1.13.0" {
			// 1.13.0: the invite email names the inviter and the org.
			r.DB(pdbGetOrg, r.Ms(0.7, 2.0), seed.Rows(1))
			r.DB(pdbGetUser, r.Ms(0.8, 2.4), seed.Rows(1))
			r.Cache("RPUSH", "queue:emails", r.Ms(0.3, 0.9))
		}
		r.Gap(r.Ms(0.3, 0.9))
	})

	pdbHealthy(api, window,
		pdbRoute{method: "GET", route: "/api/me", perDay: 45, queries: []string{pdbGetUser, pdbGetOrg}, dbMedianMs: 1.2},
		pdbRoute{method: "GET", route: "/api/orgs/{org_id}/members", perDay: 30, queries: []string{
			"SELECT users.id, users.email, users.display_name, org_members.role FROM org_members JOIN users ON users.id = org_members.user_id WHERE org_members.org_id = $1::INTEGER ORDER BY users.display_name LIMIT $2::INTEGER",
		}, dbMedianMs: 3},
		pdbRoute{method: "GET", route: "/api/invites", perDay: 20, queries: []string{
			"SELECT invites.id, invites.email, invites.role, invites.created_at FROM invites WHERE invites.org_id = $1::INTEGER AND invites.accepted_at IS NULL ORDER BY invites.created_at DESC",
		}},
		pdbRoute{method: "POST", route: "/api/invites/{token}/accept", perDay: 15, queries: []string{
			"SELECT invites.id, invites.org_id, invites.email, invites.role, invites.token, invites.invited_by, invites.created_at, invites.accepted_at FROM invites WHERE invites.token = $1::VARCHAR",
			"UPDATE invites SET accepted_at=$1::TIMESTAMP WITHOUT TIME ZONE WHERE invites.id = $2::INTEGER",
			"INSERT INTO org_members (org_id, user_id, role) VALUES ($1::INTEGER, $2::INTEGER, $3::VARCHAR)",
		}},
		pdbRoute{method: "PATCH", route: "/api/me", perDay: 12, queries: []string{
			"UPDATE users SET display_name=$1::VARCHAR WHERE users.id = $2::INTEGER", pdbGetUser, pdbGetOrg,
		}, dbMedianMs: 1.5},
	)
	api.Host().CPU(seed.Flat(0.17, 0.26))
	api.Host().Memory(seed.Flat(0.41, 0.47))

	s.Expose("slowRequest", pdbSlowest(lookup, s.Ago(24*time.Hour), func(r *seed.Request) bool { return r.Status == 200 }))
	s.Expose("deployAt", deployAt.Format(time.RFC3339))
	s.Expose("p50Ms24h", pdbMs(lookup, 0.5, s.Ago(24*time.Hour), s.Now))
	s.Expose("p95Ms24h", pdbMs(lookup, 0.95, s.Ago(24*time.Hour), s.Now))
	s.Expose("p50MsFirstWeek", pdbMs(lookup, 0.5, s.Ago(window), s.Ago(window-7*24*time.Hour)))
	s.Expose("invitesP50MsBeforeDeploy", pdbMs(invites, 0.5, s.Ago(window), deployAt))
	s.Expose("invitesP50MsAfterDeploy", pdbMs(invites, 0.5, deployAt, s.Now))
	return nil
}
