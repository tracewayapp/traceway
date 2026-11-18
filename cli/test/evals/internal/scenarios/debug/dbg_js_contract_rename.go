package debug

import (
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/scenarios"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/seed"
)

func init() { scenarios.Register("debug/js-contract-rename", dbgSeedJSContractRename) }

// dbgSeedJSContractRename: storefront-api v5.9.0 (6h ago) shared the order
// summary serializer between list and detail and renamed total_cents to
// totalCents on the way. The web app, unchanged for two weeks, reads
// order.total_cents, so rendering the order history throws in formatCents.
// Every crash follows a GET /api/orders response, captured as a network
// action. Distractor: a steady, harmless ResizeObserver message group.
func dbgSeedJSContractRename(s *seed.Scenario) error {
	deployAt := s.Ago(6 * time.Hour).Truncate(time.Minute).Add(-4 * time.Minute)
	web := s.Project("storefront-web", seed.Browser, "react")
	apiP := s.Project("storefront-api", seed.OTel)
	api := apiP.Service("storefront-api", seed.Node("express", "pg")).Versions("5.8.2", seed.Deploy{At: deployAt, Version: "5.9.0"})
	const webVersion = "2026.10.3"
	crashStack := "TypeError: Cannot read properties of undefined (reading 'toString')\n" +
		"formatCents()\n    ../../src/lib/money.js:5:24\n" +
		"orderTotalLabel()\n    ../../src/lib/orders.js:5:10\n" +
		"OrderRow()\n    ../../src/components/OrderHistory.jsx:10:12\n" +
		"renderWithHooks()\n    ../../../node_modules/react-dom/cjs/react-dom-client.production.js:4645:21\n" +
		"updateFunctionComponent()\n    ../../../node_modules/react-dom/cjs/react-dom-client.production.js:6799:15\n" +
		"beginWork()\n    ../../../node_modules/react-dom/cjs/react-dom-client.production.js:7858:14\n" +
		"performUnitOfWork()\n    ../../../node_modules/react-dom/cjs/react-dom-client.production.js:12707:14\n" +
		"workLoopSync()\n    ../../../node_modules/react-dom/cjs/react-dom-client.production.js:12584:37\n"

	list := api.Endpoint("GET", "/api/orders")
	list.Traffic(seed.Daily(420), seed.Diurnal(), seed.Over(7*24*time.Hour))
	list.Detail(seed.DetailPolicy{Recent: 6 * time.Hour, Rate: 0.05, Max: 15})
	list.Handler(func(r *seed.Req) {
		r.Gap(r.Ms(0.4, 1.0))
		r.DB("SELECT account_id FROM sessions WHERE id = $1 AND expires_at > now()", r.Ms(0.8, 2))
		n := r.IntN(21)
		r.DB("SELECT id, created_at, status, item_count, total_cents, currency FROM orders WHERE account_id = $1 ORDER BY created_at DESC LIMIT 20 OFFSET $2", r.Ms(2, 6), seed.Rows(n))
		r.Gap(r.Ms(0.3, 1.0))
		r.RootAttr("http.response.body.size", 40+n*148)
		if r.Version() != "5.9.0" || n == 0 {
			return
		}
		// The browser renders the response and crashes.
		end := r.At()
		sid := uuid.NewSHA1(uuid.NameSpaceOID, []byte(fmt.Sprintf("%s-%d", r.Request().TraceHex(), n))).String()
		resp := end.Add(r.Ms(25, 90))
		at := resp.Add(r.Ms(8, 40))
		started := resp.Add(-time.Duration(30+r.IntN(400)) * time.Second)
		web.Session(sid, started, at.Add(time.Duration(5+r.IntN(60))*time.Second), map[string]string{}, webVersion)
		nav := r.Start().Add(-r.Ms(150, 900))
		sent := r.Start().Add(-r.Ms(20, 60))
		web.Crash(seed.Crash{At: at, Stack: crashStack, AppVersion: webVersion, SessionID: sid,
			Actions: []seed.Action{
				{Type: "navigation", At: nav, From: "/account", To: "/account/orders"},
				{Type: "network", At: sent, Method: "GET", URL: "https://shop.example.com/api/orders?page=1", DurationMs: float64(resp.Sub(sent).Microseconds()) / 1000, Status: 200, RespBytes: 40 + n*148},
			}})
	})
	detail := api.Endpoint("GET", "/api/orders/:id")
	detail.Traffic(seed.Daily(260), seed.Diurnal(), seed.Over(7*24*time.Hour))
	detail.Handler(func(r *seed.Req) {
		r.Gap(r.Ms(0.4, 1.0))
		r.DB("SELECT account_id FROM sessions WHERE id = $1 AND expires_at > now()", r.Ms(0.8, 2))
		r.DB("SELECT id, created_at, status, item_count, total_cents, currency, shipping_address FROM orders WHERE id = $1 AND account_id = $2", r.Ms(1, 4))
		r.DB("SELECT sku, name, qty, unit_price_cents FROM order_items WHERE order_id = $1", r.Ms(1, 4), seed.Rows(1+r.IntN(6)))
		r.Gap(r.Ms(0.2, 0.5))
	})
	s.Background(api, seed.TypicalTraffic(3))

	// A harmless browser message every few hours, for the whole week.
	for t := s.Ago(7 * 24 * time.Hour).Add(37 * time.Minute); t.Before(s.Ago(20 * time.Minute)); t = t.Add(time.Duration(70+s.RNG.IntN(200)) * time.Minute) {
		web.Crash(seed.Crash{At: t, Stack: "ResizeObserver loop completed with undelivered notifications.", AppVersion: webVersion})
	}

	s.Expose("primaryProject", "storefront-web")
	s.Expose("deployAt", deployAt.Format(time.RFC3339))
	s.AfterFlush(func() error {
		hash, err := web.GroupMatching(8*24*time.Hour, "formatCents")
		if err != nil {
			return err
		}
		s.Expose("hash", hash)
		other, err := web.GroupMatching(8*24*time.Hour, "ResizeObserver")
		if err != nil {
			return err
		}
		s.Expose("otherHash", other)
		occs, err := dbgOccurrences(s, web, hash, 100)
		if err != nil {
			return err
		}
		if len(occs) == 0 {
			return fmt.Errorf("no occurrences of %s", hash)
		}
		// The link a customer-support agent pastes: a recent occurrence.
		pick := occs[0]
		if len(occs) > 3 {
			pick = occs[2]
		}
		s.Expose("occ", dbgOccRef(pick))
		return nil
	})
	return nil
}
