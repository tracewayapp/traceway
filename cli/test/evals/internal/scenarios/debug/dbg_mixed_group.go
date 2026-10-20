package debug

import (
	"fmt"
	"time"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/scenarios"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/seed"
)

func init() { scenarios.Register("debug/mixed-group", dbgSeedMixedGroup) }

type dbgDest struct {
	country, postal string
	weight          float64
}

// dbgSeedMixedGroup: every error of POST /api/shipments is recorded by one
// helper (httpx.Error, RecordError with a stack), so different failures
// share one stack and, with the message stripped, one hash. From 6 to 2
// days ago the zone-5 rate table of one carrier was missing ("no rate
// table", dormant since it was loaded). Since v3.8.0 (20h ago) a US-only
// ZIP pattern rejects every other country's postal codes. The group's
// firstSeen and most of its count belong to the dormant error; the latest
// occurrences are the new one.
func dbgSeedMixedGroup(s *seed.Scenario) error {
	deployAt := s.Ago(20 * time.Hour).Truncate(time.Minute).Add(-4 * time.Minute)
	gapFrom := s.Ago(6*24*time.Hour + 2*time.Hour).Truncate(time.Hour)
	gapTo := s.Ago(2*24*time.Hour + 5*time.Hour).Truncate(time.Hour)
	p := s.Project("shipping", seed.OTel)
	api := p.Service("shipping-api", seed.Go("nethttp", "pgx")).Versions("3.7.4", seed.Deploy{At: deployAt, Version: "3.8.0"})
	const mod = "example.com/shipping-api/internal"
	stack := dbgOtelStack(3317,
		mod+"/telemetry.init.0.func1({0xe714c8?, 0x348199aae4e0?}, {0xe656e0, 0x348199a0c1a0})\n\t/app/internal/telemetry/otel.go:26 +0xe5",
		mod+"/httpx.Error({0xe6ec50, 0x3481998620f0}, 0x348199922000, {0xe656e0, 0x348199a0c1a0})\n\t/app/internal/httpx/errors.go:21 +0xc3",
		mod+"/shipments.(*Handler).Create(0x3481998521d8, {0xe6ec50, 0x3481998620f0}, 0x348199922000)\n\t/app/internal/shipments/handler.go:26 +0x274",
		"net/http.HandlerFunc.ServeHTTP(0x3481999a4180?, {0xe6ec50?, 0x3481998620f0?}, 0x34819991f3f0?)\n\t/usr/local/go/src/net/http/server.go:2306 +0x29",
		"net/http.(*ServeMux).ServeHTTP(0x348199c8e780?, {0xe6ec50, 0x3481998620f0}, 0x348199922000)\n\t/usr/local/go/src/net/http/server.go:2848 +0x1c7",
		"main.main.AccessLog.func1({0xe70280, 0x348199a26060}, 0x348199922000)\n\t/app/internal/telemetry/access.go:26 +0xd5",
		"net/http.HandlerFunc.ServeHTTP(0x34819991f608?, {0xe70280?, 0x348199a26060?}, 0x348199a340a0?)\n\t/usr/local/go/src/net/http/server.go:2306 +0x29",
		mod+"/telemetry.init.0.func2.routeAttr.1({0xe70280?, 0x348199a26060?}, 0x348199922000)\n\t/app/internal/telemetry/otel_http.go:25 +0x4c",
		"net/http.HandlerFunc.ServeHTTP(0xe714c8?, {0xe70280?, 0x348199a26060?}, 0xe60168?)\n\t/usr/local/go/src/net/http/server.go:2306 +0x29",
		"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp.(*middleware).serveHTTP(0x348199995930, {0xe6ed40, 0x3481999c2000}, 0x3481999ca000, {0xe64a20, 0x3481998629a8})\n\t/go/pkg/mod/go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp@v0.63.0/handler.go:180 +0x1258",
		"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp.NewMiddleware.func1.1({0xe6ed40?, 0x3481999c2000?}, 0x4bbc53?)\n\t/go/pkg/mod/go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp@v0.63.0/handler.go:67 +0x35",
	)

	dests := []dbgDest{
		{"US", "", 86}, {"MX", "06600", 2}, {"DE", "10117", 2.2}, {"FR", "75008", 1.6}, {"ES", "28013", 0.9}, {"IT", "00184", 0.8},
		{"GB", "SW1A 2AA", 1.3}, {"GB", "EC2V 7HH", 0.6}, {"CA", "M5V 3L9", 1.1}, {"NL", "1012 LG", 0.6}, {"AU", "2000", 0.5}, {"JP", "100-8111", 0.4},
	}
	usZips := []string{"94105", "10001", "60601", "73301", "98101", "30303", "02108", "80202"}
	zone := func(c string) int {
		switch c {
		case "US":
			return 1
		case "CA", "MX":
			return 3
		case "GB", "FR", "DE", "NL", "ES", "IT":
			return 5
		default:
			return 7
		}
	}
	usFormat := func(code string) bool {
		if len(code) != 5 {
			return false
		}
		for _, c := range code {
			if c < '0' || c > '9' {
				return false
			}
		}
		return true
	}
	total := 0.0
	for _, d := range dests {
		total += d.weight
	}

	create := api.Endpoint("POST", "/api/shipments")
	create.Traffic(seed.Daily(1000), seed.Diurnal(), seed.Over(7*24*time.Hour))
	create.Detail(seed.DetailPolicy{Recent: 6 * time.Hour, Rate: 0.03, Max: 20})
	create.Handler(func(r *seed.Req) {
		x := r.Float() * total
		d := dests[len(dests)-1]
		for _, c := range dests {
			if x -= c.weight; x <= 0 {
				d = c
				break
			}
		}
		postal := d.postal
		if d.country == "US" {
			postal = usZips[r.IntN(len(usZips))]
		}
		carrier := []string{"ups-ground", "ups-ground", "dhl-express", "usps-priority"}[r.IntN(4)]
		if d.country == "US" && carrier == "dhl-express" {
			carrier = "ups-ground"
		}
		r.RootAttr("shipment.carrier", carrier)
		r.RootAttr("shipment.destination.country", d.country)
		r.Gap(r.Ms(0.4, 1.0))
		if r.Version() == "3.8.0" && !usFormat(postal) {
			msg := fmt.Sprintf("validate address: invalid request: postal code %q is not valid for %s", postal, d.country)
			r.Gap(r.Ms(0.05, 0.2))
			r.Error(seed.Exc{Type: "*fmt.wrapError", Message: msg, Stack: stack})
			r.Status(422)
			r.Gap(r.Ms(0.2, 0.5))
			return
		}
		at := r.Start()
		q := "SELECT price_cents FROM rate_tables WHERE carrier = $1 AND zone = $2 AND max_weight_grams >= $3 ORDER BY max_weight_grams LIMIT 1"
		z := zone(d.country)
		if carrier == "dhl-express" && z == 5 && !at.Before(gapFrom) && at.Before(gapTo) {
			r.DB(q, r.Ms(1, 3), seed.Rows(0))
			r.Error(seed.Exc{Type: "*fmt.wrapError", Message: fmt.Sprintf("rate shipment: carrier %q has no rate table for zone %d", carrier, z), Stack: stack})
			r.Status(500)
			r.Gap(r.Ms(0.2, 0.5))
			return
		}
		r.DB(q, r.Ms(1, 3), seed.Rows(1))
		r.DB("INSERT INTO shipments (order_id, carrier, price_cents, created_at) VALUES ($1, $2, $3, now()) RETURNING id", r.Ms(2, 5))
		r.Gap(r.Ms(0.2, 0.6))
		r.Status(201)
	})

	track := api.Endpoint("GET", "/api/shipments/{id}")
	track.Traffic(seed.Daily(600), seed.Diurnal(), seed.Over(7*24*time.Hour))
	track.Handler(func(r *seed.Req) {
		r.Gap(r.Ms(0.3, 0.8))
		r.DB("SELECT id, order_id, carrier, price_cents FROM shipments WHERE id = $1", r.Ms(1, 4))
		r.Gap(r.Ms(0.2, 0.6))
	})
	s.Background(api, seed.TypicalTraffic(5))

	s.Expose("deployAt", deployAt.Format(time.RFC3339))
	s.Expose("gapTo", gapTo.Format(time.RFC3339))
	s.AfterFlush(func() error {
		hash, err := p.GroupMatching(8*24*time.Hour, "shipments.(*Handler).Create")
		if err != nil {
			return err
		}
		s.Expose("hash", hash)
		occs, err := dbgOccurrences(s, p, hash, 100)
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
