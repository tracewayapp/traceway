package perf

import (
	"fmt"
	"math"
	"time"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/scenarios"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/seed"
)

func init() { scenarios.Register("perf/ctl-alert-handoff", pctSeedAlertHandoff) }

// pctCategories are the storefront's category slugs, most popular first.
var pctCategories = []string{
	"shoes", "jackets", "t-shirts", "jeans", "sneakers", "dresses", "hoodies", "bags", "accessories", "socks",
	"shirts", "shorts", "skirts", "sweaters", "boots", "hats", "scarves", "belts", "swimwear", "sportswear",
	"underwear", "sunglasses", "watches", "gloves",
}

// pctSeedAlertHandoff: the p95 alert on the category listing fired five
// minutes after v5.1.0 restarted all three catalog instances. Each instance
// starts with an empty in-process category cache, which a background preload
// fills one category every 25s, and an empty pgx pool; until a category is
// preloaded, the first request for it on that instance runs the listing
// query (one to two seconds), and the first few requests open connections.
// Ten minutes after the restart the listing is back to ~20ms on v5.1.0;
// v5.0.4, 30h earlier, shows the same transient. Expected:
// cold_start_jit_connection_warmup. Distractor: a code regression in v5.1.0
// (steady-state latency is unchanged).
func pctSeedAlertHandoff(s *seed.Scenario) error {
	p := s.Project("catalog", seed.OTel)
	window := 48 * time.Hour
	prevDeploy := s.Ago(30 * time.Hour)
	deployAt := s.Ago(48 * time.Minute)
	fireAt := deployAt.Add(5 * time.Minute)
	api := p.Service("catalog-api", seed.Go("nethttp", "pgx")).
		Instances("catalog-api-a", "catalog-api-b", "catalog-api-c").
		Versions("5.0.3", seed.Deploy{At: prevDeploy, Version: "5.0.4"}, seed.Deploy{At: deployAt, Version: "5.1.0"})

	// Per-instance process state: the version it runs (a change means the
	// process restarted), the categories it has cached, the connections
	// its pool has opened. Each endpoint generates its requests minute by
	// minute on its own, so each keeps its own view of the processes.
	type proc struct {
		version string
		started time.Time
		cached  map[string]bool
		conns   int
	}
	restartOf := func(t time.Time) time.Time {
		switch {
		case !t.Before(deployAt):
			return deployAt
		case !t.Before(prevDeploy):
			return prevDeploy
		}
		return s.Ago(window + time.Hour)
	}
	newState := func() func(r *seed.Req) *proc {
		procs := map[string]*proc{}
		return func(r *seed.Req) *proc {
			st := procs[r.Host()]
			if st == nil || st.version != r.Version() {
				fresh := &proc{version: r.Version(), started: restartOf(r.Start()), cached: map[string]bool{}}
				if st == nil {
					// The window opens on warm processes.
					for _, c := range pctCategories {
						fresh.cached[c] = true
					}
					fresh.conns = 10
				}
				st = fresh
				procs[r.Host()] = st
			}
			return st
		}
	}
	weights := make([]float64, len(pctCategories))
	total := 0.0
	for i := range weights {
		weights[i] = 1 / math.Sqrt(float64(i+1))
		total += weights[i]
	}
	pickCategory := func(r *seed.Req) string {
		x := r.Float() * total
		for i, w := range weights {
			x -= w
			if x <= 0 {
				return pctCategories[i]
			}
		}
		return pctCategories[len(pctCategories)-1]
	}
	connect := func(r *seed.Req, st *proc) {
		if st.conns >= 10 || !r.Chance(0.8) {
			return
		}
		st.conns++
		sp := r.Compute("connect", r.Ms(90, 220), seed.A("db.system.name", "postgresql"), seed.A("server.address", "catalog-db.internal"), seed.A("server.port", 5432))
		sp.Kind, sp.Scope = seed.KindClient, "github.com/exaring/otelpgx"
	}

	// Shoppers in other time zones keep the night from going quiet.
	night := seed.RateFunc(func(t time.Time) float64 { return math.Max(1, 0.9/pctDiurnal(t)) })
	listing := api.Endpoint("GET", "/api/categories/{slug}/products")
	listing.Traffic(seed.Daily(3600), seed.Diurnal(), night, seed.Over(window))
	listing.Detail(seed.DetailPolicy{Recent: 70 * time.Minute, Rate: 0.35, Max: 60})
	listingState := newState()
	listing.Handler(func(r *seed.Req) {
		st := listingState(r)
		cat := pickCategory(r)
		r.RootAttr("url.path", "/api/categories/"+cat+"/products")
		r.Gap(r.Ms(0.3, 0.9))
		preloaded := false
		for i, c := range pctCategories {
			if c == cat {
				preloaded = r.Start().After(st.started.Add(time.Duration(i+1) * 25 * time.Second))
			}
		}
		if !st.cached[cat] && !preloaded {
			connect(r, st)
			r.DB("SELECT p.id, p.slug, p.name, p.brand, p.thumbnail_url, pp.amount_cents, pp.currency, i.available FROM products p JOIN product_prices pp ON pp.product_id = p.id AND pp.price_list = $2 JOIN inventory i ON i.product_id = p.id WHERE p.category_id = (SELECT id FROM categories WHERE slug = $1) AND p.active ORDER BY p.rank LIMIT 500",
				r.LogNormal(1250*time.Millisecond, 1900*time.Millisecond), seed.Rows(380+r.IntN(121)))
			st.cached[cat] = true
		}
		r.Gap(r.Ms(7.2, 21.6)) // page, badges, encode
	})

	product := api.Endpoint("GET", "/api/products/{id}")
	product.Traffic(seed.Daily(1500), seed.Diurnal(), night, seed.Over(window))
	productState := newState()
	product.Handler(func(r *seed.Req) {
		st := productState(r)
		r.Gap(r.Ms(0.3, 0.9))
		connect(r, st)
		r.DB("SELECT p.id, p.slug, p.name, p.brand, p.description, pp.amount_cents, pp.currency, i.available FROM products p JOIN product_prices pp ON pp.product_id = p.id AND pp.price_list = $2 JOIN inventory i ON i.product_id = p.id WHERE p.id = $1",
			r.LogNormal(3*time.Millisecond, 11*time.Millisecond), seed.Rows(1))
		r.Gap(r.Ms(0.5, 2))
	})
	s.Background(api, seed.TypicalTraffic(4))
	for _, h := range api.Hosts {
		api.HostNamed(h).CPU(seed.Flat(0.18, 0.3))
		api.HostNamed(h).Memory(seed.Flat(0.4, 0.5))
	}

	email := p.EmailChannel("Catalog team email")
	rule := p.Rule(seed.Rule{Name: "Category listing p95 above 1s", Type: "endpoint_p95_threshold", Channel: email, Cooldown: 30,
		Config: map[string]any{"endpoint": "GET /api/categories/{slug}/products", "thresholdMs": 1000, "lookbackMinutes": 10}})

	s.Expose("ruleId", rule)
	s.Expose("deployAt", pctRFC3339(deployAt))
	s.Expose("prevDeployAt", pctRFC3339(prevDeploy))
	s.Expose("fireAt", pctRFC3339(fireAt))
	s.Expose("linkFrom", pctRFC3339(fireAt.Add(-time.Minute)))
	s.Expose("linkTo", pctRFC3339(fireAt.Add(2*time.Minute)))
	s.Expose("alertP95", func() any {
		return math.Round(listing.PercentileBetween(0.95, fireAt.Add(-10*time.Minute), fireAt).Seconds() * 1000)
	})
	s.Expose("p95MsLast20m", pctLazyMs(listing, 0.95, s.Ago(20*time.Minute), s.Now))
	s.Expose("p95MsBeforeDeploy", pctLazyMs(listing, 0.95, deployAt.Add(-6*time.Hour), deployAt))
	s.Expose("slowRequest", pctRef(func() *seed.Request { return pctSlowestBetween(listing, deployAt, fireAt) }))
	s.Expose("warmRequest", pctRef(func() *seed.Request {
		return pctNearestBetween(listing, s.Ago(15*time.Minute), s.Now, listing.PercentileBetween(0.5, s.Ago(15*time.Minute), s.Now))
	}))
	// Window edges for the oracle, as "<minutes> after the deploy".
	for _, m := range []int{-60, 15, 30} {
		key := fmt.Sprintf("deployPlus%dm", m)
		if m < 0 {
			key = fmt.Sprintf("deployMinus%dm", -m)
		}
		s.Expose(key, pctRFC3339(deployAt.Add(time.Duration(m)*time.Minute)))
	}
	return nil
}
