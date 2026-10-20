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
	pnc503Message   = "Server error '503 Service Unavailable' for url 'https://rates.carrier.example/v2/rates'\nFor more information check: https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/503"
	pncRetryErrType = "tenacity.RetryError"
)

// What the FastAPI instrumentation records on the server span when all three
// attempts fail: the last attempt's exception, then the RetryError raised
// through the access-log middleware.
const pncReadTimeoutCause = `Traceback (most recent call last):
  File "/usr/local/lib/python3.12/site-packages/httpx/_transports/default.py", line 101, in map_httpcore_exceptions
    yield
  File "/usr/local/lib/python3.12/site-packages/httpx/_transports/default.py", line 394, in handle_async_request
    resp = await self._pool.handle_async_request(req)
           ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
  File "/usr/local/lib/python3.12/site-packages/httpcore/_async/connection_pool.py", line 256, in handle_async_request
    raise exc from None
  File "/usr/local/lib/python3.12/site-packages/httpcore/_async/connection_pool.py", line 236, in handle_async_request
    response = await connection.handle_async_request(
               ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
  File "/usr/local/lib/python3.12/site-packages/httpcore/_async/connection.py", line 103, in handle_async_request
    return await self._connection.handle_async_request(request)
           ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
  File "/usr/local/lib/python3.12/site-packages/httpcore/_async/http11.py", line 136, in handle_async_request
    raise exc
  File "/usr/local/lib/python3.12/site-packages/httpcore/_async/http11.py", line 106, in handle_async_request
    ) = await self._receive_response_headers(**kwargs)
        ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
  File "/usr/local/lib/python3.12/site-packages/httpcore/_async/http11.py", line 177, in _receive_response_headers
    event = await self._receive_event(timeout=timeout)
            ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
  File "/usr/local/lib/python3.12/site-packages/httpcore/_async/http11.py", line 217, in _receive_event
    data = await self._network_stream.read(
           ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
  File "/usr/local/lib/python3.12/site-packages/httpcore/_backends/anyio.py", line 32, in read
    with map_exceptions(exc_map):
         ^^^^^^^^^^^^^^^^^^^^^^^
  File "/usr/local/lib/python3.12/contextlib.py", line 158, in __exit__
    self.gen.throw(value)
  File "/usr/local/lib/python3.12/site-packages/httpcore/_exceptions.py", line 14, in map_exceptions
    raise to_exc(exc) from exc
httpcore.ReadTimeout

The above exception was the direct cause of the following exception:

Traceback (most recent call last):
  File "/usr/local/lib/python3.12/site-packages/tenacity/asyncio/__init__.py", line 114, in __call__
    result = await fn(*args, **kwargs)
             ^^^^^^^^^^^^^^^^^^^^^^^^^
  File "/app/app/carriers/client.py", line 36, in fetch_rates
    response = await self._http.post(RATES_URL, json=payload)
               ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
  File "/usr/local/lib/python3.12/site-packages/httpx/_client.py", line 1859, in post
    return await self.request(
           ^^^^^^^^^^^^^^^^^^^
  File "/usr/local/lib/python3.12/site-packages/httpx/_client.py", line 1540, in request
    return await self.send(request, auth=auth, follow_redirects=follow_redirects)
           ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
  File "/usr/local/lib/python3.12/site-packages/httpx/_client.py", line 1629, in send
    response = await self._send_handling_auth(
               ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
  File "/usr/local/lib/python3.12/site-packages/httpx/_client.py", line 1657, in _send_handling_auth
    response = await self._send_handling_redirects(
               ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
  File "/usr/local/lib/python3.12/site-packages/httpx/_client.py", line 1694, in _send_handling_redirects
    response = await self._send_single_request(request)
               ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
  File "/usr/local/lib/python3.12/site-packages/httpx/_client.py", line 1730, in _send_single_request
    response = await transport.handle_async_request(request)
               ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
  File "/usr/local/lib/python3.12/site-packages/opentelemetry/instrumentation/httpx/__init__.py", line 1175, in _handle_async_request_wrapper
    raise exception.with_traceback(exception.__traceback__)
  File "/usr/local/lib/python3.12/site-packages/opentelemetry/instrumentation/httpx/__init__.py", line 1133, in _handle_async_request_wrapper
    response = await wrapped(*args, **kwargs)
               ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
  File "/usr/local/lib/python3.12/site-packages/httpx/_transports/default.py", line 393, in handle_async_request
    with map_httpcore_exceptions():
         ^^^^^^^^^^^^^^^^^^^^^^^^^
  File "/usr/local/lib/python3.12/contextlib.py", line 158, in __exit__
    self.gen.throw(value)
  File "/usr/local/lib/python3.12/site-packages/httpx/_transports/default.py", line 118, in map_httpcore_exceptions
    raise mapped_exc(message) from exc
httpx.ReadTimeout`

const pnc503Cause = `Traceback (most recent call last):
  File "/usr/local/lib/python3.12/site-packages/tenacity/asyncio/__init__.py", line 114, in __call__
    result = await fn(*args, **kwargs)
             ^^^^^^^^^^^^^^^^^^^^^^^^^
  File "/app/app/carriers/client.py", line 37, in fetch_rates
    response.raise_for_status()
  File "/usr/local/lib/python3.12/site-packages/httpx/_models.py", line 829, in raise_for_status
    raise HTTPStatusError(message, request=request, response=self)
httpx.HTTPStatusError: Server error '503 Service Unavailable' for url 'https://rates.carrier.example/v2/rates'
For more information check: https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/503`

const pncRetryErrorFrames = `

The above exception was the direct cause of the following exception:

Traceback (most recent call last):
  File "/usr/local/lib/python3.12/site-packages/opentelemetry/instrumentation/fastapi/__init__.py", line 307, in __call__
    await self.app(scope, receive, send)
  File "/usr/local/lib/python3.12/site-packages/starlette/middleware/base.py", line 182, in __call__
    with recv_stream, send_stream, collapse_excgroups():
                                   ^^^^^^^^^^^^^^^^^^^^
  File "/usr/local/lib/python3.12/contextlib.py", line 158, in __exit__
    self.gen.throw(value)
  File "/usr/local/lib/python3.12/site-packages/starlette/_utils.py", line 85, in collapse_excgroups
    raise exc
  File "/usr/local/lib/python3.12/site-packages/starlette/middleware/base.py", line 184, in __call__
    response = await self.dispatch_func(request, call_next)
               ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
  File "/app/app/access_log.py", line 17, in access_log
    response = await call_next(request)
               ^^^^^^^^^^^^^^^^^^^^^^^^
  File "/usr/local/lib/python3.12/site-packages/starlette/middleware/base.py", line 159, in call_next
    raise app_exc
  File "/usr/local/lib/python3.12/site-packages/starlette/middleware/base.py", line 144, in coro
    await self.app(scope, receive_or_disconnect, send_no_error)
  File "/usr/local/lib/python3.12/site-packages/starlette/middleware/exceptions.py", line 63, in __call__
    await wrap_app_handling_exceptions(self.app, conn)(scope, receive, send)
  File "/usr/local/lib/python3.12/site-packages/starlette/_exception_handler.py", line 53, in wrapped_app
    raise exc
  File "/usr/local/lib/python3.12/site-packages/starlette/_exception_handler.py", line 42, in wrapped_app
    await app(scope, receive, sender)
  File "/usr/local/lib/python3.12/site-packages/fastapi/middleware/asyncexitstack.py", line 18, in __call__
    await self.app(scope, receive, send)
  File "/usr/local/lib/python3.12/site-packages/starlette/routing.py", line 716, in __call__
    await self.middleware_stack(scope, receive, send)
  File "/usr/local/lib/python3.12/site-packages/starlette/routing.py", line 736, in app
    await route.handle(scope, receive, send)
  File "/usr/local/lib/python3.12/site-packages/starlette/routing.py", line 290, in handle
    await self.app(scope, receive, send)
  File "/usr/local/lib/python3.12/site-packages/fastapi/routing.py", line 123, in app
    await wrap_app_handling_exceptions(app, request)(scope, receive, send)
  File "/usr/local/lib/python3.12/site-packages/starlette/_exception_handler.py", line 53, in wrapped_app
    raise exc
  File "/usr/local/lib/python3.12/site-packages/starlette/_exception_handler.py", line 42, in wrapped_app
    await app(scope, receive, sender)
  File "/usr/local/lib/python3.12/site-packages/fastapi/routing.py", line 109, in app
    response = await f(request)
               ^^^^^^^^^^^^^^^^
  File "/usr/local/lib/python3.12/site-packages/fastapi/routing.py", line 387, in app
    raw_response = await run_endpoint_function(
                   ^^^^^^^^^^^^^^^^^^^^^^^^^^^^
  File "/usr/local/lib/python3.12/site-packages/fastapi/routing.py", line 288, in run_endpoint_function
    return await dependant.call(**values)
           ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
  File "/app/app/routes/quotes.py", line 27, in create_quote
    rates = await carrier.fetch_rates(parcel)
            ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
  File "/usr/local/lib/python3.12/site-packages/tenacity/asyncio/__init__.py", line 189, in async_wrapped
    return await copy(fn, *args, **kwargs)
           ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
  File "/usr/local/lib/python3.12/site-packages/tenacity/asyncio/__init__.py", line 111, in __call__
    do = await self.iter(retry_state=retry_state)
         ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
  File "/usr/local/lib/python3.12/site-packages/tenacity/asyncio/__init__.py", line 153, in iter
    result = await action(retry_state)
             ^^^^^^^^^^^^^^^^^^^^^^^^^
  File "/usr/local/lib/python3.12/site-packages/tenacity/_utils.py", line 99, in inner
    return call(*args, **kwargs)
           ^^^^^^^^^^^^^^^^^^^^^
  File "/usr/local/lib/python3.12/site-packages/tenacity/__init__.py", line 421, in exc_check
    raise retry_exc from fut.exception()
`

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
		failure := ""
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
				sp.Status, sp.StatusMsg = seed.StatusError, "ReadTimeout: "
				cause, failure = "ReadTimeout: ", "ReadTimeout"
			case x < 0.05:
				r.HTTP("POST", pncRatesURL, r.LogNormal(35*time.Millisecond, 90*time.Millisecond), 503, seed.A("error.type", "503"))
				cause, failure = "HTTPStatusError: "+pnc503Message, "HTTPStatusError"
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
					seed.A("code.file.path", "/usr/local/lib/python3.12/site-packages/tenacity/before_sleep.py"), seed.A("code.function.name", "log_it"), seed.A("code.line.number", 65))
				r.Gap(time.Duration(wait * float64(time.Second)))
			}
		}
		if !ok {
			stack := pncReadTimeoutCause
			if failure == "HTTPStatusError" {
				stack = pnc503Cause
			}
			msg := "RetryError[<Future at 0x7f3a2c1b9d50 state=finished raised " + failure + ">]"
			r.Error(seed.Exc{Type: pncRetryErrType, Message: msg, Stack: stack + pncRetryErrorFrames + pncRetryErrType + ": " + msg, Escaped: true})
			r.Status(500)
			r.Compute(route+" http send", r.Ms(0.05, 0.2))
			return
		}
		r.DB("INSERT INTO quotes (parcel_id, carrier, service_level, amount_minor, currency, expires_at) VALUES (%s, %s, %s, %s, %s, %s)", r.LogNormal(2500*time.Microsecond, 6*time.Millisecond))
		r.Compute(route+" http send", r.Ms(0.05, 0.3))
	})

	track := api.Endpoint("GET", "/api/shipments/{shipment_id}/tracking")
	track.Traffic(seed.Daily(1100), seed.Diurnal(), seed.Over(window))
	track.Handler(func(r *seed.Req) {
		r.DB("SELECT status, location, occurred_at FROM tracking_events WHERE shipment_id = %s ORDER BY occurred_at DESC LIMIT 20", r.LogNormal(3*time.Millisecond, 8*time.Millisecond), seed.Rows(1+r.IntN(12)))
		r.Compute("GET /api/shipments/{shipment_id}/tracking http send", r.Ms(0.05, 0.3))
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
