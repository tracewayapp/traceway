package perf

import (
	"fmt"
	"time"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/scenarios"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/seed"
)

func init() { scenarios.Register("perf/net-chatty-fanout", pncSeedChattyFanout) }

var pncSkuPrefixes = []string{"SH", "TS", "JK", "BG", "SK", "HT", "SC", "BT"}

func pncSku(r *seed.Req) string {
	return fmt.Sprintf("%s-%05d", pncSkuPrefixes[r.IntN(len(pncSkuPrefixes))], 1000+r.IntN(80000))
}

// pncSeedChattyFanout: the storefront gateway builds the cart view by asking
// three services in two other projects about every cart line, one line at a
// time: catalog-service and pricing-service (catalog project) and
// inventory-service (fulfillment project). A ten-line cart is ~31 hops, each
// spending ~10ms on the wire for ~2ms of server work. Distractor: the hops
// are sequential, which reads like "run them in parallel", but they are
// dozens of tiny per-line calls to the same three endpoints; the batch
// endpoints that exist (POST /products/batch, used by the wishlist) remove
// the round trips.
func pncSeedChattyFanout(s *seed.Scenario) error {
	window := 24 * time.Hour
	store := s.Project("storefront", seed.OTel)
	catalogP := s.Project("catalog", seed.OTel)
	fulfil := s.Project("fulfillment", seed.OTel)
	s.Expose("primaryProject", "storefront")

	gw := store.Service("storefront-gateway", seed.Node("express", "pg")).Versions("4.18.2")
	carts := store.Service("cart-service", seed.Go("chi", "pgx")).Versions("2.6.0")
	catalog := catalogP.Service("catalog-service", seed.Go("nethttp", "pgx")).Versions("7.3.1")
	pricing := catalogP.Service("pricing-service", seed.Java("spring-boot")).Versions("3.11.0")
	inventory := fulfil.Service("inventory-service", seed.Python("fastapi", "psycopg")).Versions("1.9.4")

	sku := ""
	cartGet := carts.Endpoint("GET", "/carts/:id")
	cartGet.Handler(func(r *seed.Req) {
		r.Gap(r.Ms(0.2, 0.5))
		r.DB("SELECT sku, qty, added_at FROM cart_lines WHERE cart_id = $1 ORDER BY added_at", r.LogNormal(3*time.Millisecond, 8*time.Millisecond))
		r.Compute("encode response", r.Ms(0.2, 0.6))
	})
	product := catalog.Endpoint("GET", "/products/:sku")
	product.Handler(func(r *seed.Req) {
		r.Gap(r.Ms(0.1, 0.3))
		r.Cache("get", "product:"+sku, r.Ms(0.3, 0.9))
		if r.Chance(0.08) {
			r.DB("SELECT sku, title, brand, image_url, price_list FROM products WHERE sku = $1", r.Ms(2, 5))
		}
		r.Compute("encode response", r.Ms(0.1, 0.4))
	})
	productsBatch := catalog.Endpoint("POST", "/products/batch")
	productsBatch.Handler(func(r *seed.Req) {
		r.Gap(r.Ms(0.1, 0.3))
		r.Cache("mget", "product:"+pncSku(r), r.Ms(0.5, 1.4))
		r.Compute("encode response", r.Ms(0.4, 1.2))
	})
	price := pricing.Endpoint("GET", "/prices/:sku")
	price.Handler(func(r *seed.Req) {
		r.Gap(r.Ms(0.2, 0.6))
		r.DB("SELECT amount_minor, currency, valid_until FROM price_book WHERE sku = ? AND price_list = ? AND currency = ?", r.LogNormal(1500*time.Microsecond, 4*time.Millisecond))
	})
	stock := inventory.Endpoint("GET", "/stock/:sku")
	stock.Handler(func(r *seed.Req) {
		r.Gap(r.Ms(0.2, 0.5))
		r.Cache("GET", "stock:eu-central:"+sku, r.Ms(0.4, 1.1))
		r.Compute("encode response", r.Ms(0.1, 0.3))
	})

	rtt := func(r *seed.Req) time.Duration { return r.LogNormal(9*time.Millisecond, 15*time.Millisecond) }
	lines := map[*seed.Request]int{}
	cart := gw.Endpoint("GET", "/api/cart")
	cart.Traffic(seed.Daily(420), seed.Diurnal(), seed.Over(window))
	cart.Detail(seed.DetailPolicy{Recent: 6 * time.Hour, Rate: 0.03, Max: 4})
	cart.Handler(func(r *seed.Req) {
		n := 5 + r.IntN(12)
		lines[r.Request()] = n
		cartID := fmt.Sprintf("c_%07d", r.IntN(9000000))
		r.Gap(r.Ms(0.4, 1.0))
		r.Cache("GET", "sess:"+fmt.Sprintf("%x", r.IntN(1<<30)), r.Ms(0.4, 1.2))
		pncCall(r, cartGet, rtt(r), "/carts/"+cartID, "")
		for range n {
			sku = pncSku(r)
			pncCall(r, product, rtt(r), "/products/"+sku, "")
			pncCall(r, price, rtt(r), "/prices/"+sku, "currency=EUR")
			pncCall(r, stock, rtt(r), "/stock/"+sku, "")
		}
		r.Gap(r.Ms(1.7, 4.6)) // build the view
	})

	wish := gw.Endpoint("GET", "/api/wishlist")
	wish.Traffic(seed.Daily(240), seed.Diurnal(), seed.Over(window))
	wish.Handler(func(r *seed.Req) {
		r.Gap(r.Ms(0.4, 1.0))
		r.Cache("GET", "sess:"+fmt.Sprintf("%x", r.IntN(1<<30)), r.Ms(0.4, 1.2))
		r.DB("SELECT sku FROM wishlist_items WHERE customer_id = $1 ORDER BY added_at DESC LIMIT 50", r.LogNormal(3*time.Millisecond, 8*time.Millisecond), seed.Rows(5+r.IntN(30)))
		pncCall(r, productsBatch, rtt(r), "/products/batch", "")
		r.Gap(r.Ms(0.8, 2))
	})

	s.Background(gw, seed.TrafficOpts{Endpoints: 4, PerDay: 150})
	gw.Host().CPU(seed.Flat(0.14, 0.24))
	gw.Host().Memory(seed.Flat(0.41, 0.49))
	catalog.Host().CPU(seed.Flat(0.11, 0.19))
	inventory.Host().CPU(seed.Flat(0.09, 0.16))

	s.Expose("cartRequest", cart.Pick(func(r *seed.Request) bool {
		n := lines[r]
		return n >= 8 && n <= 11 && r.Start.After(s.Ago(6*time.Hour))
	}))
	s.Expose("p50Ms24h", func() any { return cart.Percentile(0.5, 24*time.Hour).Seconds() * 1000 })
	s.Expose("p95Ms24h", func() any { return cart.Percentile(0.95, 24*time.Hour).Seconds() * 1000 })
	return nil
}
