package perf

import (
	"fmt"
	"net/url"
	"time"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/scenarios"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/seed"
)

func init() { scenarios.Register("perf/xs-frontend-timeouts", pctSeedXSFrontendTimeouts) }

// pctSearchTerms are what shoppers type.
var pctSearchTerms = []string{"running shoes", "rain jacket", "wool socks", "trail", "linen shirt", "backpack", "kids boots", "yoga mat", "sunglasses", "denim"}

// pctSeedXSFrontendTimeouts: the web shop aborts product searches after 8s
// (AbortSignal.timeout) and reports the TimeoutError. Since ~10h ago the
// hosted search provider the API calls has been slow (median 1.5s, tail past
// 15s), so GET /api/search's p95 is ~8s. Every browser crash lines up with a
// backend request that ran past 8s; the link is the URL path and the time,
// since the browser SDK's trace id does not join OTLP traces. Expected:
// slow_third_party_upstream_api. Distractor: a frontend bug (the 8s timeout).
func pctSeedXSFrontendTimeouts(s *seed.Scenario) error {
	web := s.Project("shop-web", seed.Browser, "react")
	apiProj := s.Project("shop-api", seed.OTel)
	degradedAt := s.Ago(10*time.Hour + 20*time.Minute)
	api := apiProj.Service("shop-api", seed.Node("express", "pg")).Versions("3.12.0")

	search := api.Endpoint("GET", "/api/search")
	search.Traffic(seed.Daily(2000), seed.Diurnal(), seed.Over(3*24*time.Hour))
	search.Detail(seed.DetailPolicy{Recent: 10 * time.Hour, Rate: 0.03, Max: 20})
	search.Handler(func(r *seed.Req) {
		q := pctSearchTerms[r.IntN(len(pctSearchTerms))]
		r.RootAttr("url.path", "/api/search")
		r.RootAttr("url.query", "q="+url.QueryEscape(q))
		r.Gap(r.Ms(0.5, 1.5))
		provider := r.LogNormal(110*time.Millisecond, 380*time.Millisecond)
		if r.Start().After(degradedAt) {
			provider = r.LogNormal(1500*time.Millisecond, 16*time.Second)
		}
		r.HTTP("POST", "https://shop-prod.search-cloud.example/1/indexes/products/query", provider, 200,
			seed.A("http.request.body.size", 180+r.IntN(60)), seed.A("http.response.body.size", 9000+r.IntN(6000)))
		r.DB("SELECT id, stock, price_cents FROM product_stock WHERE id = ANY($1)", r.LogNormal(3*time.Millisecond, 9*time.Millisecond), seed.Rows(24))
		r.Compute("merge results", r.Ms(1, 4)).Scope = "shop-api"
	})
	product := api.Endpoint("GET", "/api/products/:id")
	product.Traffic(seed.Daily(900), seed.Diurnal(), seed.Over(3*24*time.Hour))
	product.Handler(func(r *seed.Req) {
		r.Gap(r.Ms(0.5, 1.5))
		r.DB("SELECT id, name, slug, description, price_cents, images FROM products WHERE id = $1", r.LogNormal(3*time.Millisecond, 10*time.Millisecond), seed.Rows(1))
		r.Gap(r.Ms(0.5, 2))
	})
	cart := api.Endpoint("POST", "/api/cart/items")
	cart.Traffic(seed.Daily(400), seed.Diurnal(), seed.Over(3*24*time.Hour))
	cart.Handler(func(r *seed.Req) {
		r.Gap(r.Ms(0.5, 1.5))
		r.DB("INSERT INTO cart_items (cart_id, product_id, qty) VALUES ($1, $2, $3) ON CONFLICT (cart_id, product_id) DO UPDATE SET qty = cart_items.qty + EXCLUDED.qty", r.LogNormal(3*time.Millisecond, 10*time.Millisecond))
		r.Status(204)
	})
	s.Background(api, seed.TypicalTraffic(4))
	api.Host().CPU(seed.Flat(0.2, 0.3))
	api.Host().Memory(seed.Flat(0.5, 0.58))

	// The browser side: every search that ran past 8s on the API, for two
	// shoppers in three, ended in the web app's TimeoutError, reported with
	// the network and navigation actions the SDK captured.
	const timeout = 8 * time.Second
	// Chromium words the abort as "signal timed out"; that is the group.
	browsers := []string{"Chrome 141 / macOS", "Chrome 141 / Windows", "Chrome 141 / Android", "Edge 141 / Windows", "Chrome 141 / Linux"}
	stack := "TimeoutError: signal timed out\nsearchProducts()\n    src/api/search.ts:9:21\nrunSearch()\n    src/pages/SearchPage.tsx:13:21"
	var newest *seed.Request
	crashes := 0
	// Crashes are derived from the generated backend requests, so they are
	// built lazily, once, when the scenario flushes.
	done := false
	build := func() {
		if done {
			return
		}
		done = true
		for i, r := range search.Requests() {
			if r.Start.Before(degradedAt) || r.Duration() < timeout+300*time.Millisecond || r.Start.After(s.Ago(2*time.Minute)) {
				continue
			}
			if i%3 == 0 {
				continue
			}
			r.KeepDetail()
			newest = r
			q, _ := r.Root.Attrs["url.query"].(string)
			at := r.Start.Add(timeout + 40*time.Millisecond)
			term, _ := url.QueryUnescape(q[2:])
			prev := r.Start.Add(-time.Duration(20+i%40) * time.Second)
			product := fmt.Sprintf("/products/%d", 1000+i%900)
			typed := r.Start.Add(-time.Duration(3+i%9) * time.Second)
			web.Crash(seed.Crash{
				At: at, Stack: stack, AppVersion: "2026.10.14", Device: browsers[i%len(browsers)],
				Attributes: map[string]string{"page": "/search", "release": "web-2026.10.14"},
				Actions: []seed.Action{
					{Type: "navigation", At: prev, From: "/", To: product},
					{Type: "network", At: prev.Add(400 * time.Millisecond), Method: "GET", URL: "https://www.shop.example/api" + product, DurationMs: float64(40 + i%60), Status: 200, RespBytes: 4200},
					{Type: "navigation", At: typed, From: product, To: "/search"},
					{Type: "network", At: at, Method: "GET", URL: "https://www.shop.example/api/search?" + q, DurationMs: float64(timeout.Milliseconds()) + 2, Error: "TimeoutError: signal timed out"},
				},
				Logs: []seed.ActionLog{{At: at.Add(5 * time.Millisecond), Level: "warn", Message: fmt.Sprintf("search for %q gave up after 8000ms", term)}},
			})
			crashes++
		}
	}
	s.Expose("browserCrashes", func() any { build(); return crashes })
	s.Expose("slowRequest", func() any {
		build()
		if newest == nil {
			return nil
		}
		return newest.Ref()
	})
	s.Expose("degradedAt", pctRFC3339(degradedAt))
	s.Expose("p95MsBefore", pctLazyMs(search, 0.95, s.Ago(3*24*time.Hour), degradedAt))
	s.Expose("p95MsAfter", pctLazyMs(search, 0.95, degradedAt, s.Now))
	s.Expose("primaryProject", "shop-web")
	s.AfterFlush(func() error {
		hash, err := web.GroupMatching(24*time.Hour, "signal timed out")
		if err != nil {
			return err
		}
		s.Expose("hash", hash)
		return nil
	})
	return nil
}
