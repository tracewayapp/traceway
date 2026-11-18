package perf

import (
	"math"
	"time"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/scenarios"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/seed"
)

func init() { scenarios.Register("perf/xs-ios-watchdog", pctSeedXSIOSWatchdog) }

// pctIOSWatchdogStack is the launch watchdog termination as the app's
// MetricKit forwarder reports it (Traceway iOS frame format, symbolicated
// with the uploaded dSYM; system frames stay image+offset).
const pctIOSWatchdogStack = `Watchdog termination: EXC_CRASH (SIGKILL), FRONTBOARD 0x8badf00d scene-create watchdog transgression: exhausted real (wall clock) time allowance of 20.00 seconds
*** *** *** *** *** *** *** *** *** *** *** *** *** *** *** ***
os: ios arch: arm64
#0  libsystem_kernel.dylib+0x1c48
#1  libdispatch.dylib+0x4a24
#2  libdispatch.dylib+0x49d0
#3  ConfigClient.fetchBlocking(timeout:) (ConfigClient.swift:33:23)
#4  RemoteConfig.loadAtLaunch() (RemoteConfig.swift:19:37)
#5  AppDelegate.application(_:didFinishLaunchingWithOptions:) (AppDelegate.swift:17:29)
#6  UIKitCore+0x2f1a84
#7  UIKitCore+0x2f0e2c
#8  UIKitCore+0x2ef9c0
#9  FrontBoardServices+0x1f2c8
#10  libdispatch.dylib+0x3fa0
#11  CoreFoundation+0x77d0c
#12  GraphicsServices+0x1980
#13  UIKitCore+0x3c3ef8
#14  Shop+0x6a4c
#15  dyld+0x5e4c`

// pctSeedXSIOSWatchdog: two marketing pushes (5h and 1h35m ago) each set off
// a wave of app launches. Every launch calls GET /api/config and registers
// its push token, and the mobile API's database pool (8 connections, shared
// with the feed) runs dry: requests wait up to ~30s for a connection before
// their first query. The iOS app fetches the config synchronously on the
// main thread during launch, so launches that wait past ~20s are killed by
// the watchdog. No app release in three weeks. Expected:
// connection_pool_exhaustion. Distractor: a pure client bug (the blocking
// fetch is real, but it only bites since the API started taking 20s).
func pctSeedXSIOSWatchdog(s *seed.Scenario) error {
	app := s.Project("shop-ios", seed.Mobile, "ios")
	apiProj := s.Project("mobile-api", seed.OTel)
	pushes := []time.Time{s.Ago(5*time.Hour + 10*time.Minute), s.Ago(95 * time.Minute)}
	window := 2 * 24 * time.Hour
	api := apiProj.Service("mobile-api", seed.Go("nethttp", "sql")).Versions("2.9.0")

	// wave is how hard a push wave is hitting at t: 1 at the push, decaying
	// over a few minutes, 0 outside the waves.
	decay := func(t time.Time, tau float64) float64 {
		for _, p := range pushes {
			if d := t.Sub(p); d >= 0 && d < 40*time.Minute {
				return math.Exp(-d.Minutes() / tau)
			}
		}
		return 0
	}
	wave := func(t time.Time) float64 { return decay(t, 4) }
	// The connection queue builds up faster than it drains.
	backlog := func(t time.Time) float64 { return decay(t, 9) }
	// A push reaches every user whatever the hour, so the wave adds the
	// same number of launches by day or night (it is divided by the
	// day/night factor the traffic is multiplied by).
	launches := seed.RateFunc(func(t time.Time) float64 { return 1 + 45*wave(t)/pctDiurnal(t) })
	poolWait := func(r *seed.Req) time.Duration {
		w := backlog(r.Start())
		if w < 0.05 {
			return r.Ms(0.02, 0.3)
		}
		return r.LogNormal(time.Duration(12000*w)*time.Millisecond, time.Duration(36000*w)*time.Millisecond)
	}

	config := api.Endpoint("GET", "/api/config")
	config.Traffic(seed.Daily(2400), seed.Diurnal(), launches, seed.Over(window))
	config.Detail(seed.DetailPolicy{Recent: 6 * time.Hour, Rate: 0.01, Max: 20})
	config.Handler(func(r *seed.Req) {
		r.Gap(r.Ms(0.3, 0.8))
		r.Gap(poolWait(r))
		r.DB("SELECT key, value, rollout_pct FROM feature_flags WHERE platform = $1 AND enabled", r.LogNormal(2*time.Millisecond, 6*time.Millisecond), seed.Rows(40+r.IntN(10)))
		r.DB("SELECT min_supported, latest FROM app_versions WHERE platform = $1", r.LogNormal(1*time.Millisecond, 4*time.Millisecond), seed.Rows(1))
		r.Compute("encode response", r.Ms(0.3, 1)).Scope = "mobile-api"
	})
	devices := api.Endpoint("POST", "/api/devices")
	devices.Traffic(seed.Daily(1200), seed.Diurnal(), launches, seed.Over(window))
	devices.Handler(func(r *seed.Req) {
		r.Gap(r.Ms(0.3, 0.8))
		r.Gap(poolWait(r))
		r.DB("INSERT INTO push_tokens (user_id, platform, token, updated_at) VALUES ($1, $2, $3, now()) ON CONFLICT (token) DO UPDATE SET user_id = EXCLUDED.user_id, updated_at = now()", r.LogNormal(3*time.Millisecond, 10*time.Millisecond))
		r.Status(204)
	})
	feed := api.Endpoint("GET", "/api/feed")
	feed.Traffic(seed.Daily(1500), seed.Diurnal(), launches, seed.Over(window))
	feed.Handler(func(r *seed.Req) {
		r.Gap(r.Ms(0.3, 0.8))
		r.Gap(poolWait(r))
		r.DB("SELECT p.id, p.title, p.image_url, p.price_cents, p.badge FROM promotions p WHERE p.active AND p.segment = ANY($1) ORDER BY p.priority DESC LIMIT 30", r.LogNormal(120*time.Millisecond, 380*time.Millisecond), seed.Rows(30))
		r.DB("SELECT product_id, count(*) FROM recent_views WHERE user_id = $1 GROUP BY product_id ORDER BY count(*) DESC LIMIT 20", r.LogNormal(60*time.Millisecond, 220*time.Millisecond), seed.Rows(20))
		r.Compute("assemble feed", r.Ms(2, 6)).Scope = "mobile-api"
	})
	// Everything that touches the database shares the pool, so every route
	// waits in a wave (no unrelated background routes on this service).
	product := api.Endpoint("GET", "/api/products/{id}")
	product.Traffic(seed.Daily(1800), seed.Diurnal(), seed.RateFunc(func(t time.Time) float64 { return 1 + 8*wave(t)/pctDiurnal(t) }), seed.Over(window))
	product.Handler(func(r *seed.Req) {
		r.Gap(r.Ms(0.3, 0.8))
		r.Gap(poolWait(r))
		r.DB("SELECT id, name, description, price_cents, images FROM products WHERE id = $1", r.LogNormal(2*time.Millisecond, 7*time.Millisecond), seed.Rows(1))
		r.Compute("encode response", r.Ms(0.3, 1)).Scope = "mobile-api"
	})
	api.Host().CPU(seed.Flat(0.18, 0.32))
	api.Host().Memory(seed.Flat(0.48, 0.56))

	// The iOS side: a launch whose config fetch had not returned after ~19s
	// (the rest of launch takes about a second) is killed by the watchdog;
	// half the launches are iOS.
	const killAfter = 19500 * time.Millisecond
	models := []string{"iPhone16,2 / iOS 18.0.1", "iPhone15,3 / iOS 18.0", "iPhone14,5 / iOS 17.6.1", "iPhone17,1 / iOS 18.0.1", "iPhone13,2 / iOS 17.5"}
	crashes := 0
	var newest *seed.Request
	// Crashes are derived from the generated backend requests, so they are
	// built lazily, once, when the scenario flushes.
	done := false
	build := func() {
		if done {
			return
		}
		done = true
		for i, r := range config.Requests() {
			if r.Duration() < killAfter || i%2 == 0 || r.Start.After(s.Ago(2*time.Minute)) {
				continue
			}
			r.KeepDetail()
			newest = r
			app.Crash(seed.Crash{
				At: r.Start.Add(20*time.Second + time.Duration(200+i%600)*time.Millisecond), Stack: pctIOSWatchdogStack,
				AppVersion: "4.12.0", Device: models[i%len(models)],
				Attributes: map[string]string{"source": "metrickit", "phase": "launch"},
			})
			crashes++
		}
	}
	s.Expose("watchdogCrashes", func() any { build(); return crashes })
	s.Expose("slowRequest", func() any {
		build()
		if newest == nil {
			return nil
		}
		return newest.Ref()
	})
	s.Expose("push1", pctRFC3339(pushes[0]))
	s.Expose("push2", pctRFC3339(pushes[1]))
	s.Expose("wave2From", pctRFC3339(pushes[1].Add(-5*time.Minute)))
	s.Expose("wave2To", pctRFC3339(pushes[1].Add(20*time.Minute)))
	s.Expose("p50Ms24h", pctLazyMs(config, 0.5, s.Ago(24*time.Hour), s.Now))
	s.Expose("p99Ms24h", pctLazyMs(config, 0.99, s.Ago(24*time.Hour), s.Now))
	s.Expose("p95MsWave2", pctLazyMs(config, 0.95, pushes[1], pushes[1].Add(10*time.Minute)))
	s.Expose("primaryProject", "shop-ios")
	s.AfterFlush(func() error {
		hash, err := app.GroupMatching(24*time.Hour, "0x8badf00d")
		if err != nil {
			return err
		}
		s.Expose("hash", hash)
		return nil
	})
	return nil
}
