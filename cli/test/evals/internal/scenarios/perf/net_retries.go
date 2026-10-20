package perf

import (
	"fmt"
	"time"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/scenarios"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/seed"
)

func init() { scenarios.Register("perf/net-retries", pncSeedRetries) }

const (
	pncRatesURL     = "https://rates.carrier.example/v2/rates"
	pncRetryFn      = "app.carriers.client.CarrierClient.fetch_rates"
	pncReadTimeout  = "The read operation timed out"
	pnc503Message   = "Server error '503 Service Unavailable' for url 'https://rates.carrier.example/v2/rates'\nFor more information check: https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/503"
	pncRetryErrType = "tenacity.RetryError"
)

const pncRetryErrorStack = `Traceback (most recent call last):
  File "/usr/local/lib/python3.12/site-packages/tenacity/asyncio/__init__.py", line 114, in __call__
    result = await fn(*args, **kwargs)
  File "/app/app/carriers/client.py", line 36, in fetch_rates
    response = await self._http.post(RATES_URL, json=payload)
  File "/usr/local/lib/python3.12/site-packages/httpx/_client.py", line 1859, in post
    return await self.request(
  File "/usr/local/lib/python3.12/site-packages/httpx/_transports/default.py", line 118, in map_httpcore_exceptions
    raise mapped_exc(message) from exc
httpx.ReadTimeout: The read operation timed out

The above exception was the direct cause of the following exception:

Traceback (most recent call last):
  File "/usr/local/lib/python3.12/site-packages/starlette/routing.py", line 76, in app
    response = await f(request)
  File "/app/app/routes/quotes.py", line 27, in create_quote
    rates = await carrier.fetch_rates(parcel)
  File "/usr/local/lib/python3.12/site-packages/tenacity/asyncio/__init__.py", line 189, in async_wrapped
    return await copy(fn, *args, **kwargs)
  File "/usr/local/lib/python3.12/site-packages/tenacity/asyncio/__init__.py", line 153, in iter
    raise retry_exc from fut.exception()
tenacity.RetryError: RetryError[<Future at 0x7f3a2c1b9d50 state=finished raised ReadTimeout>]`

// pncSeedRetries: the shipping quote endpoint asks a carrier's rate API
// through httpx with no explicit timeout (httpx's default 5s) inside a
// tenacity retry (3 attempts, exponential wait 1s then 2s). A few percent of
// attempts hang until the read timeout or get a quick 503; each failure is
// waited out and retried, so a minority of requests take 6-13s with the same
// carrier span two or three times. Distractor: the carrier looks slow, but
// its successful answers take ~150ms; the tail is the client's own timeout
// and backoff.
func pncSeedRetries(s *seed.Scenario) error {
	window := 2 * 24 * time.Hour
	p := s.Project("logistics", seed.OTel)
	api := p.Service("quotes-api", seed.Python("fastapi", "psycopg")).Versions("2.3.0")

	var forced *seed.Request
	forceFrom, forceTo := s.Ago(110*time.Minute), s.Ago(95*time.Minute)
	quotes := api.Endpoint("POST", "/api/shipping/quotes")
	quotes.Traffic(seed.Daily(2000), seed.Diurnal(), seed.Over(window))
	quotes.Detail(seed.DetailPolicy{Recent: 6 * time.Hour, Rate: 0.02, Max: 12})
	quotes.Handler(func(r *seed.Req) {
		route := "POST /api/shipping/quotes"
		r.Compute(route+" http receive", r.Ms(0.05, 0.3))
		r.DB("SELECT id, weight_grams, length_mm, width_mm, height_mm, origin_postcode, destination_postcode, destination_country FROM parcels WHERE id = %s", r.LogNormal(2*time.Millisecond, 6*time.Millisecond))
		ok := false
		force := false
		if forced == nil && !r.Start().Before(forceFrom) && r.Start().Before(forceTo) {
			forced, force = r.Request(), true
		}
		for attempt := 1; attempt <= 3; attempt++ {
			x := r.Float()
			if force {
				x = map[bool]float64{true: 0.01, false: 0.5}[attempt < 3]
			}
			var cause string
			switch {
			case x < 0.035:
				sp := r.HTTP("POST", pncRatesURL, 5*time.Second+r.Ms(1, 8), 0)
				delete(sp.Attrs, "http.response.status_code")
				sp.Attrs["error.type"] = "ReadTimeout"
				sp.Status, sp.StatusMsg = seed.StatusError, pncReadTimeout
				cause = "ReadTimeout: " + pncReadTimeout
			case x < 0.05:
				r.HTTP("POST", pncRatesURL, r.LogNormal(35*time.Millisecond, 90*time.Millisecond), 503, seed.A("error.type", "503"))
				cause = "HTTPStatusError: " + pnc503Message
			default:
				r.HTTP("POST", pncRatesURL, r.LogNormal(150*time.Millisecond, 400*time.Millisecond), 200)
				ok = true
			}
			if ok {
				break
			}
			if attempt < 3 {
				wait := float64(int(1) << (attempt - 1))
				r.Log(seed.SevWarn, fmt.Sprintf("Retrying %s in %.1f seconds as it raised %s.", pncRetryFn, wait, cause),
					seed.A("code.filepath", "/usr/local/lib/python3.12/site-packages/tenacity/before_sleep.py"), seed.A("code.function", "log_it"), seed.A("code.lineno", 65))
				r.Gap(time.Duration(wait * float64(time.Second)))
			}
		}
		if !ok {
			r.Error(seed.Exc{Type: pncRetryErrType, Message: "RetryError[<Future at 0x7f3a2c1b9d50 state=finished raised ReadTimeout>]", Stack: pncRetryErrorStack, Escaped: true})
			r.Status(500)
			r.Compute(route+" http send", r.Ms(0.05, 0.2))
			return
		}
		r.DB("INSERT INTO quotes (parcel_id, carrier, service_level, amount_minor, currency, expires_at) VALUES (%s, %s, %s, %s, %s, %s)", r.LogNormal(2500*time.Microsecond, 6*time.Millisecond))
		r.Compute(route+" http send", r.Ms(0.05, 0.3))
	})

	track := api.Endpoint("GET", "/api/shipments/:id/tracking")
	track.Traffic(seed.Daily(1100), seed.Diurnal(), seed.Over(window))
	track.Handler(func(r *seed.Req) {
		r.DB("SELECT status, location, occurred_at FROM tracking_events WHERE shipment_id = %s ORDER BY occurred_at DESC LIMIT 20", r.LogNormal(3*time.Millisecond, 8*time.Millisecond), seed.Rows(1+r.IntN(12)))
		r.Compute("GET /api/shipments/:id/tracking http send", r.Ms(0.05, 0.3))
	})

	s.Background(api, seed.TrafficOpts{Endpoints: 4, PerDay: 150})
	api.Host().CPU(seed.Flat(0.15, 0.24))
	api.Host().Memory(seed.Flat(0.5, 0.57))

	s.Expose("retriedRequest", quotes.Pick(func(r *seed.Request) bool { return r == forced }))
	s.Expose("p50Ms24h", func() any { return quotes.Percentile(0.5, 24*time.Hour).Seconds() * 1000 })
	s.Expose("p95Ms24h", func() any { return quotes.Percentile(0.95, 24*time.Hour).Seconds() * 1000 })
	s.Expose("p99Ms24h", func() any { return quotes.Percentile(0.99, 24*time.Hour).Seconds() * 1000 })
	return nil
}
