package debug

import (
	"fmt"
	"time"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/scenarios"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/seed"
)

func init() { scenarios.Register("debug/timeout-downstream", dbgSeedTimeoutDownstream) }

// dbgSeedTimeoutDownstream: the cart page's quote calls the pricing service,
// which has always taken 0.25-4.2s (p50 1.1s, p95 2.4s, p99 3.1-3.4s, no release
// all week). cart-api v1.14.0 (26h ago) lowered its default upstream timeout
// from 5s to 2s, so about one quote in nine now fails with context deadline
// exceeded. Distractor: the pricing service looks slow, but it is unchanged.
func dbgSeedTimeoutDownstream(s *seed.Scenario) error {
	deployAt := s.Ago(26 * time.Hour).Truncate(time.Minute).Add(-3 * time.Minute)
	window := 7 * 24 * time.Hour
	p := s.Project("storefront", seed.OTel)
	cart := p.Service("cart-api", seed.Go("nethttp", "pgx")).Versions("1.13.2", seed.Deploy{At: deployAt, Version: "1.14.0"})
	pricing := p.Service("pricing-service", seed.Python("fastapi", "psycopg")).Versions("4.2.1")
	const mod = "example.com/cart-api/internal"
	stack := dbgOtelStack(6152,
		mod+"/telemetry.init.0.func1({0x1d4c3a8, 0xc0004e60e0}, {0x1b6e0a0, 0xc0003f81e0})\n\t/app/internal/telemetry/otel.go:17 +0x8e",
		mod+"/quote.(*Handler).ServeHTTP(0xc0001a4b40, {0x1d4f2c0, 0xc0004d2380}, 0xc0004c8780)\n\t/app/internal/quote/handler.go:33 +0x1f8",
		"net/http.(*ServeMux).ServeHTTP(0xc00012a0e0, {0x1d4f2c0, 0xc0004d2380}, 0xc0004c8500)\n\t/usr/local/go/src/net/http/server.go:2848 +0x1c4",
		mod+"/telemetry.init.0.func2.routeAttr.1({0x1d4f2c0?, 0xc0004d2380?}, 0xc0004c8500)\n\t/app/internal/telemetry/otel_http.go:29 +0x4c",
		"net/http.HandlerFunc.ServeHTTP(0xc0001c8a80?, {0x1d4f2c0?, 0xc0004d2380?}, 0x1b2a4e0?)\n\t/usr/local/go/src/net/http/server.go:2306 +0x29",
		"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp.(*middleware).serveHTTP(0xc0001c6000, {0x1d4e1a0, 0xc0004c6000}, 0xc0004c8280, {0x1d3b5a0, 0xc00012a0e0})\n\t/go/pkg/mod/go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp@v0.63.0/handler.go:180 +0x1a4c",
		"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp.NewMiddleware.func1.1({0x1d4e1a0?, 0xc0004c6000?}, 0x0?)\n\t/go/pkg/mod/go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp@v0.63.0/handler.go:67 +0x35",
		"net/http.HandlerFunc.ServeHTTP(0xc0001c8000?, {0x1d4e1a0?, 0xc0004c6000?}, 0xc0004c6001?)\n\t/usr/local/go/src/net/http/server.go:2306 +0x29",
		"net/http.serverHandler.ServeHTTP({0xc000180d20?}, {0x1d4e1a0?, 0xc0004c6000?}, 0x6?)\n\t/usr/local/go/src/net/http/server.go:3331 +0x8e",
	)

	quote := cart.Endpoint("GET", "/api/cart/quote")
	quote.Traffic(seed.Daily(800), seed.Diurnal(), seed.Over(window))
	quote.Detail(seed.DetailPolicy{Recent: 8 * time.Hour, Rate: 0.04, Max: 25})
	quote.Handler(func(r *seed.Req) {
		cartID := fmt.Sprintf("c_%x", r.IntN(1<<24))
		r.Gap(r.Ms(0.3, 0.8))
		// The pricing service's own time: stable for the whole window.
		d := r.LogNormal(1100*time.Millisecond, 3400*time.Millisecond)
		if d < 250*time.Millisecond {
			d = 250 * time.Millisecond
		}
		if d > 4200*time.Millisecond {
			d = 4200 * time.Millisecond
		}
		rtt := r.Ms(1.5, 4)
		timeout := 5 * time.Second
		if r.Version() == "1.14.0" {
			timeout = 2 * time.Second
		}
		url := "http://pricing.internal:8080/v1/quotes?cart=" + cartID
		timedOut := d+rtt > timeout
		cd := d + rtt
		if timedOut {
			cd = timeout
		}
		client := r.HTTP("GET", url, cd, 200)
		srv := dbgDownstream(s, client, pricing, "GET", "/v1/quotes", "/v1/quotes", rtt/2, d, 200)
		rules := 3 + r.IntN(6)
		srv.Children = append(srv.Children,
			&seed.Span{Name: "SELECT price_rules", Kind: seed.KindClient, Start: r.Ms(0.5, 1.5), Dur: r.Ms(4, 12), Scope: pricing.Stack.DBScope, SpanID: dbgRandID(s),
				Attrs: map[string]any{"db.system.name": "postgresql", "db.operation.name": "SELECT", "db.collection.name": "price_rules", "db.namespace": "pricing",
					"db.query.text": "SELECT id, kind, params FROM price_rules WHERE active AND region = %(region)s", "server.address": "pricing-db.internal", "server.port": 5432, "db.response.returned_rows": rules}},
		)
		evalStart := 15 * time.Millisecond
		srv.Children = append(srv.Children, &seed.Span{Name: "evaluate promotions", Kind: seed.KindInternal, Start: evalStart, Dur: d - evalStart - 2*time.Millisecond,
			Scope: "pricing.promotions", SpanID: dbgRandID(s), Attrs: map[string]any{"pricing.rules": rules}})
		dbgRecord(r, url, "SELECT price_rules", "evaluate promotions")
		if timedOut {
			msg := fmt.Sprintf("Get %q: context deadline exceeded (Client.Timeout exceeded while awaiting headers)", url)
			delete(client.Attrs, "http.response.status_code")
			client.Attrs["error.type"] = "context.deadlineExceededError"
			client.Status, client.StatusMsg = seed.StatusError, "context deadline exceeded"
			r.Error(seed.Exc{Type: "*url.Error", Message: msg, Stack: stack})
			r.Status(502)
			r.Gap(r.Ms(0.2, 0.5))
			return
		}
		r.Compute("encode response", r.Ms(0.2, 0.6))
	})

	s.Background(cart, seed.TypicalTraffic(4))
	cart.Host().CPU(seed.Flat(0.15, 0.27))
	pricing.Host().CPU(seed.Flat(0.38, 0.52))

	s.Expose("deployAt", deployAt.Format(time.RFC3339))
	s.Expose("weekAgo", s.Ago(window-time.Hour).Format(time.RFC3339))
	s.Expose("timeoutShare", func() any {
		n, bad := 0, 0
		for _, rq := range quote.Requests() {
			if rq.Start.Before(deployAt) {
				continue
			}
			n++
			if rq.Status == 502 {
				bad++
			}
		}
		if n == 0 {
			return 0.0
		}
		return float64(bad) / float64(n)
	})
	s.AfterFlush(func() error {
		hash, err := p.GroupMatching(15*24*time.Hour, "quote.(*Handler).ServeHTTP")
		if err != nil {
			return err
		}
		s.Expose("hash", hash)
		occs, err := dbgOccurrences(s, p, hash, 5)
		if err != nil {
			return err
		}
		if len(occs) > 0 {
			s.Expose("latest", dbgOccRef(occs[0]))
		}
		return nil
	})
	return nil
}
