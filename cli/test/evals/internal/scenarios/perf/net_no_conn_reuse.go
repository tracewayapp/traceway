package perf

import (
	"fmt"
	"strings"
	"time"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/scenarios"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/seed"
)

func init() { scenarios.Register("perf/net-no-conn-reuse", pncSeedNoConnReuse) }

const pncHTTPTraceScope = "go.opentelemetry.io/contrib/instrumentation/net/http/httptrace/otelhttptrace"

// pncTracedGet records an outgoing GET the way otelhttp plus otelhttptrace
// report it: the CLIENT span, with http.getconn (and, on a new connection,
// http.dns, http.connect and http.tls inside it), http.send and
// http.receive as children. server is the time from the request being sent
// to the first response byte.
func pncTracedGet(r *seed.Req, url, host, remote string, fresh bool, server time.Duration) *seed.Span {
	port := 443
	if strings.HasPrefix(url, "http://") {
		port = 8080
	}
	addr := fmt.Sprintf("%s:%d", remote, port)
	var client *seed.Span
	client = r.Span("GET", func(r *seed.Req) {
		gc := r.Span("http.getconn", func(r *seed.Req) {
			if fresh {
				dns := r.Compute("http.dns", r.LogNormal(16*time.Millisecond, 42*time.Millisecond), seed.A("net.host.name", host), seed.A("http.dns.addrs", remote))
				dns.Scope = pncHTTPTraceScope
				conn := r.Compute("http.connect", r.LogNormal(26*time.Millisecond, 58*time.Millisecond), seed.A("http.remote", addr), seed.A("http.conn.start.network", "tcp"))
				conn.Scope = pncHTTPTraceScope
				tls := r.Compute("http.tls", r.LogNormal(48*time.Millisecond, 105*time.Millisecond))
				tls.Scope = pncHTTPTraceScope
			} else {
				r.Gap(r.Ms(0.01, 0.06))
			}
		}, seed.A("net.host.name", host), seed.A("http.remote", addr), seed.A("http.local", fmt.Sprintf("10.4.%d.%d:%d", r.IntN(8), 10+r.IntN(200), 40000+r.IntN(20000))),
			seed.A("http.conn.reused", !fresh), seed.A("http.conn.wasidle", !fresh))
		gc.Scope = pncHTTPTraceScope
		send := r.Compute("http.send", r.Ms(0.02, 0.12))
		send.Scope = pncHTTPTraceScope
		r.Gap(server)
		recv := r.Compute("http.receive", r.Ms(0.3, 1.8))
		recv.Scope = pncHTTPTraceScope
	}, seed.A("http.request.method", "GET"), seed.A("url.full", url), seed.A("server.address", host), seed.A("server.port", port),
		seed.A("http.response.status_code", 200), seed.A("network.protocol.version", "1.1"))
	client.Kind = seed.KindClient
	client.Scope = "go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	return client
}

var pncPlaces = []struct{ q, ll string }{
	{"10115+Berlin", "52.5323,13.3846"}, {"80331+M%C3%BCnchen", "48.1374,11.5755"}, {"20095+Hamburg", "53.5503,10.0007"},
	{"50667+K%C3%B6ln", "50.9384,6.9599"}, {"60311+Frankfurt", "50.1109,8.6821"}, {"70173+Stuttgart", "48.7784,9.1800"},
	{"04109+Leipzig", "51.3397,12.3731"}, {"01067+Dresden", "51.0504,13.7373"},
}

// pncSeedNoConnReuse: the store finder geocodes the query and asks for
// driving distances from a maps provider. The provider client builds a new
// http.Transport for every call, so each call pays DNS, TCP and TLS
// (http.conn.reused=false) before ~20ms of provider work. Distractor: the
// provider calls are the long pole, which reads like a slow upstream, but
// the provider answers quickly once a request is on the wire.
func pncSeedNoConnReuse(s *seed.Scenario) error {
	window := 2 * 24 * time.Hour
	p := s.Project("stores", seed.OTel)
	api := p.Service("store-locator-api", seed.Go("nethttp", "pgx")).Versions("1.14.0")
	const mapsHost = "maps.geo.example"

	nearby := api.Endpoint("GET", "/api/stores/nearby")
	nearby.Traffic(seed.Daily(1800), seed.Diurnal(), seed.Over(window))
	nearby.Detail(seed.DetailPolicy{Recent: 6 * time.Hour, Rate: 0.02, Max: 10})
	nearby.Handler(func(r *seed.Req) {
		remote := fmt.Sprintf("198.51.100.%d", 20+r.IntN(4))
		r.Gap(r.Ms(0.2, 0.6))
		place := pncPlaces[r.IntN(len(pncPlaces))]
		pncTracedGet(r, "https://"+mapsHost+"/v1/geocode?q="+place.q+"&region=de", mapsHost, remote, true, r.LogNormal(19*time.Millisecond, 45*time.Millisecond))
		r.DB("SELECT id, name, address, lat, lng FROM stores WHERE ST_DWithin(location, ST_MakePoint($1, $2)::geography, $3) ORDER BY location <-> ST_MakePoint($1, $2)::geography LIMIT 10",
			r.LogNormal(4*time.Millisecond, 10*time.Millisecond), seed.Rows(10))
		dest := make([]string, 10)
		for i := range dest {
			dest[i] = fmt.Sprintf("store%%3A%d", 1000+r.IntN(400))
		}
		pncTracedGet(r, "https://"+mapsHost+"/v1/distancematrix?origins="+place.ll+"&destinations="+strings.Join(dest, "%7C")+"&mode=driving", mapsHost, remote, true, r.LogNormal(24*time.Millisecond, 55*time.Millisecond))
		r.Gap(r.Ms(0.4, 1.2))
	})

	avail := api.Endpoint("GET", "/api/stores/{id}/availability")
	avail.Traffic(seed.Daily(900), seed.Diurnal(), seed.Over(window))
	avail.Handler(func(r *seed.Req) {
		r.Gap(r.Ms(0.2, 0.6))
		pncTracedGet(r, fmt.Sprintf("http://inventory.internal:8080/v2/stock?store=%d", 100+r.IntN(300)), "inventory.internal", fmt.Sprintf("10.20.3.%d", 10+r.IntN(5)), false, r.LogNormal(6*time.Millisecond, 14*time.Millisecond))
		r.Gap(r.Ms(0.2, 0.8))
	})

	hours := api.Endpoint("GET", "/api/stores/{id}/hours")
	hours.Traffic(seed.Daily(700), seed.Diurnal(), seed.Over(window))
	hours.Handler(func(r *seed.Req) {
		r.Gap(r.Ms(0.2, 0.6))
		r.DB("SELECT weekday, opens_at, closes_at FROM store_hours WHERE store_id = $1", r.LogNormal(2*time.Millisecond, 5*time.Millisecond), seed.Rows(7))
		r.Gap(r.Ms(0.1, 0.5))
	})

	s.Background(api, seed.TrafficOpts{Endpoints: 3, PerDay: 150})
	api.Host().CPU(seed.Flat(0.12, 0.2))
	api.Host().Memory(seed.Flat(0.38, 0.45))

	s.Expose("slowRequest", nearby.Pick(func(r *seed.Request) bool {
		return r.Start.After(s.Ago(3*time.Hour)) && r.Duration() > 260*time.Millisecond && r.Duration() < 420*time.Millisecond
	}))
	s.Expose("availRequest", avail.Pick(func(r *seed.Request) bool { return r.Start.After(s.Ago(3 * time.Hour)) }))
	s.Expose("p50Ms24h", func() any { return nearby.Percentile(0.5, 24*time.Hour).Seconds() * 1000 })
	s.Expose("p95Ms24h", func() any { return nearby.Percentile(0.95, 24*time.Hour).Seconds() * 1000 })
	return nil
}
