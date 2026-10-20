package perf

import (
	"fmt"
	"strings"
	"time"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/scenarios"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/seed"
)

func init() { scenarios.Register("perf/cache-cold-start", pncSeedColdStart) }

const pncAnnotationsScope = "io.opentelemetry.opentelemetry-instrumentation-annotations-1.16"

// pncSeedColdStart: pricing-service (Spring Boot) runs with
// spring.main.lazy-initialization=true, so a new pod starts in seconds but
// its first request creates the web and data beans and the Hikari pool
// (5-8s), and the next few run cold, interpreted code. Every release, on
// every pod, the first requests are outliers and latency is back to normal
// after a dozen requests. Distractor: the latest release (2.16.0, ~3h ago)
// rewrote the promotion rules, which invites a regression reading, but the
// same outliers follow the older releases and steady-state latency did not
// move.
func pncSeedColdStart(s *seed.Scenario) error {
	window := 10 * 24 * time.Hour
	deploys := []seed.Deploy{
		{At: s.Ago(216*time.Hour + 14*time.Minute), Version: "2.14.1"},
		{At: s.Ago(144*time.Hour + 41*time.Minute), Version: "2.15.0"},
		{At: s.Ago(48*time.Hour + 6*time.Minute), Version: "2.15.1"},
		{At: s.Ago(3*time.Hour + 2*time.Minute), Version: "2.16.0"},
	}
	p := s.Project("pricing", seed.OTel)
	svc := p.Service("pricing-service", seed.Java("spring-boot")).Versions("2.14.0", deploys...).
		Instances("pricing-service-0", "pricing-service-1", "pricing-service-2")

	deployIndex := func(t time.Time) int {
		i := -1
		for k, d := range deploys {
			if !t.Before(d.At) {
				i = k
			}
		}
		return i
	}
	served := map[string]int{}        // deploy|host -> requests served since the pod became ready
	readyAt := map[string]time.Time{} // deploy|host -> when the new pod reported started
	ordinal := map[*seed.Request]int{}

	quote := svc.Endpoint("POST", "/api/prices/quote")
	quote.Traffic(seed.Daily(1500), seed.Diurnal(), seed.Over(window))
	quote.Detail(seed.DetailPolicy{Recent: 2 * time.Hour, Rate: 0.03, Max: 8})
	quote.Handler(func(r *seed.Req) {
		// Pods that were already running when the window opened, and old pods
		// still serving while their replacement starts, are warm.
		n := 1000
		if di := deployIndex(r.Start()); di >= 0 {
			key := fmt.Sprintf("%d|%s", di, r.Host())
			if !r.Start().Before(readyAt[key]) {
				served[key]++
				n = served[key]
			}
		}
		ordinal[r.Request()] = n
		lines := 1 + r.IntN(8)
		engine := r.LogNormal(9*time.Millisecond, 22*time.Millisecond)
		dbScale := 1.0
		thread := seed.A("thread.name", "http-nio-8080-exec-1")
		if n == 1 {
			r.KeepDetail()
			r.Log(seed.SevInfo, "Initializing Spring DispatcherServlet 'dispatcherServlet'", thread)
			r.Log(seed.SevInfo, "Initializing Servlet 'dispatcherServlet'", thread)
			d := r.Ms(900, 1400)
			r.Gap(d)
			r.Log(seed.SevInfo, fmt.Sprintf("Completed initialization in %d ms", d.Milliseconds()), thread)
			r.Gap(r.Ms(2000, 3200)) // lazy bean creation for the quote path
			engine = r.Ms(700, 1300)
			dbScale = 6
		} else if n <= 3 {
			engine += r.Ms(350, 800)
			dbScale = 2.5
		} else if n <= 8 {
			engine += time.Duration(float64(r.Ms(60, 160)) * float64(9-n) / 5)
		}
		r.Gap(r.Ms(0.5, 1.5))
		eng := r.Span("PricingEngine.quote", func(r *seed.Req) {
			if n == 1 {
				// The pool starts on the first getConnection, inside quote().
				r.Log(seed.SevInfo, "HikariPool-1 - Starting...", thread)
				r.Gap(r.Ms(600, 1100))
				r.Log(seed.SevInfo, fmt.Sprintf("HikariPool-1 - Added connection org.postgresql.jdbc.PgConnection@%x", 0x10000000+r.IntN(0x6fffffff)), thread)
				r.Log(seed.SevInfo, "HikariPool-1 - Start completed.", thread)
			}
			r.DB("SELECT sku, list_price_minor FROM price_list_items WHERE price_list_id = ? AND sku IN ("+strings.TrimSuffix(strings.Repeat("?, ", lines), ", ")+")",
				time.Duration(float64(r.LogNormal(4*time.Millisecond, 11*time.Millisecond))*dbScale), seed.Rows(lines))
			r.DB("SELECT id, kind, priority, conditions, effect FROM promotion_rules WHERE active AND ? BETWEEN starts_at AND ends_at ORDER BY priority",
				time.Duration(float64(r.LogNormal(3*time.Millisecond, 8*time.Millisecond))*dbScale), seed.Rows(30+r.IntN(20)))
			r.Gap(engine)
		}, seed.A("code.function", "quote"), seed.A("code.namespace", "com.example.pricing.engine.PricingEngine"))
		eng.Scope = pncAnnotationsScope
		r.Gap(r.Ms(0.5, 1.5))
	})

	item := svc.Endpoint("GET", "/api/prices/:sku")
	item.Traffic(seed.Daily(160), seed.Diurnal(), seed.Over(window))
	item.Handler(func(r *seed.Req) {
		r.Gap(r.Ms(0.5, 1.5))
		r.DB("SELECT sku, list_price_minor, currency FROM price_list_items WHERE price_list_id = ? AND sku = ?", r.LogNormal(3*time.Millisecond, 9*time.Millisecond))
		r.Gap(r.Ms(0.5, 1.5))
	})

	// Startup lines of every pod at every release (no request context);
	// the new pods take traffic as soon as they report started.
	for i, d := range append([]seed.Deploy{{At: s.Ago(window + time.Hour), Version: "2.14.0"}}, deploys...) {
		for k, h := range svc.Hosts {
			at := d.At.Add(time.Duration(k*3) * time.Second)
			secs := 6.4 + 0.9*s.RNG.Float64()
			readyAt[fmt.Sprintf("%d|%s", i-1, h)] = at.Add(time.Duration(secs * float64(time.Second)))
			svc.LogOn(h, at, seed.SevInfo, fmt.Sprintf("Starting PricingServiceApplication v%s using Java 21.0.4 with PID 1 (/app/app.jar started by app in /app)", d.Version), seed.A("thread.name", "main"))
			svc.LogOn(h, at.Add(1200*time.Millisecond), seed.SevInfo, "Tomcat initialized with port 8080 (http)", seed.A("thread.name", "main"))
			svc.LogOn(h, at.Add(time.Duration(secs*float64(time.Second))), seed.SevInfo, fmt.Sprintf("Started PricingServiceApplication in %.3f seconds (process running for %.3f)", secs, secs+0.7), seed.A("thread.name", "main"))
		}
	}

	s.Background(svc, seed.TrafficOpts{Endpoints: 2, PerDay: 60})
	for _, h := range svc.Hosts {
		svc.HostNamed(h).CPU(seed.Flat(0.16, 0.26))
		svc.HostNamed(h).Memory(seed.Flat(0.55, 0.62))
	}

	s.Expose("coldRequest", quote.Pick(func(r *seed.Request) bool {
		return ordinal[r] == 1 && !r.Start.Before(deploys[3].At)
	}))
	s.Expose("olderColdRequest", quote.Pick(func(r *seed.Request) bool {
		return ordinal[r] == 1 && !r.Start.Before(deploys[2].At) && r.Start.Before(deploys[3].At)
	}))
	for i, d := range deploys {
		s.Expose(fmt.Sprintf("deploy%d", i+1), d.At.UTC().Format(time.RFC3339))
	}
	s.Expose("beforeFrom", deploys[3].At.Add(-24*time.Hour).UTC().Format(time.RFC3339))
	s.Expose("afterFrom", deploys[3].At.Add(time.Hour).UTC().Format(time.RFC3339))
	s.Expose("p50MsBefore", func() any {
		return quote.PercentileBetween(0.5, deploys[3].At.Add(-24*time.Hour), deploys[3].At).Seconds() * 1000
	})
	s.Expose("p50MsAfter", func() any { return quote.PercentileBetween(0.5, deploys[3].At.Add(time.Hour), s.Now).Seconds() * 1000 })
	s.Expose("p95MsBefore", func() any {
		return quote.PercentileBetween(0.95, deploys[3].At.Add(-24*time.Hour), deploys[3].At).Seconds() * 1000
	})
	s.Expose("p95MsAfter", func() any { return quote.PercentileBetween(0.95, deploys[3].At.Add(time.Hour), s.Now).Seconds() * 1000 })
	return nil
}
