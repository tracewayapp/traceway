package perf

import (
	"fmt"
	"time"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/scenarios"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/seed"
)

func init() { scenarios.Register("perf/xs-android-timeouts", pctSeedXSAndroidTimeouts) }

// pctAndroidTimeoutStack is the uncaught SocketTimeoutException the Android
// app crashes with when OkHttp's 10s read timeout fires on POST
// /api/checkout (deobfuscated with the uploaded mapping).
const pctAndroidTimeoutStack = `java.net.SocketTimeoutException: timeout
	at okio.SocketAsyncTimeout.newTimeoutException(JvmOkio.kt:147)
	at okio.AsyncTimeout.access$newTimeoutException(AsyncTimeout.kt:186)
	at okio.AsyncTimeout$source$1.read(AsyncTimeout.kt:390)
	at okio.RealBufferedSource.indexOf(RealBufferedSource.kt:436)
	at okio.RealBufferedSource.readUtf8LineStrict(RealBufferedSource.kt:329)
	at okhttp3.internal.http1.HeadersReader.readLine(HeadersReader.kt:29)
	at okhttp3.internal.http1.Http1ExchangeCodec.readResponseHeaders(Http1ExchangeCodec.kt:180)
	at okhttp3.internal.connection.Exchange.readResponseHeaders(Exchange.kt:110)
	at okhttp3.internal.http.CallServerInterceptor.intercept(CallServerInterceptor.kt:93)
	at okhttp3.internal.http.RealInterceptorChain.proceed(RealInterceptorChain.kt:109)
	at okhttp3.internal.connection.ConnectInterceptor.intercept(ConnectInterceptor.kt:34)
	at okhttp3.internal.http.RealInterceptorChain.proceed(RealInterceptorChain.kt:109)
	at okhttp3.internal.cache.CacheInterceptor.intercept(CacheInterceptor.kt:95)
	at okhttp3.internal.http.RealInterceptorChain.proceed(RealInterceptorChain.kt:109)
	at okhttp3.internal.http.BridgeInterceptor.intercept(BridgeInterceptor.kt:83)
	at okhttp3.internal.http.RealInterceptorChain.proceed(RealInterceptorChain.kt:109)
	at okhttp3.internal.http.RetryAndFollowUpInterceptor.intercept(RetryAndFollowUpInterceptor.kt:76)
	at okhttp3.internal.http.RealInterceptorChain.proceed(RealInterceptorChain.kt:109)
	at com.tracewayapp.traceway.network.TracewayOkHttpInterceptor.intercept(TracewayOkHttpInterceptor.kt:31)
	at okhttp3.internal.http.RealInterceptorChain.proceed(RealInterceptorChain.kt:109)
	at com.example.shop.net.AuthInterceptor.intercept(AuthInterceptor.kt:18)
	at okhttp3.internal.http.RealInterceptorChain.proceed(RealInterceptorChain.kt:109)
	at okhttp3.internal.connection.RealCall.getResponseWithInterceptorChain$okhttp(RealCall.kt:201)
	at okhttp3.internal.connection.RealCall.execute(RealCall.kt:154)
	at retrofit2.OkHttpCall.execute(OkHttpCall.java:207)
	at com.example.shop.checkout.CheckoutRepository$placeOrder$2.invokeSuspend(CheckoutRepository.kt:31)
	at kotlin.coroutines.jvm.internal.BaseContinuationImpl.resumeWith(ContinuationImpl.kt:33)
	at kotlinx.coroutines.DispatchedTask.run(DispatchedTask.kt:101)
	at kotlinx.coroutines.internal.LimitedDispatcher$Worker.run(LimitedDispatcher.kt:113)
	at kotlinx.coroutines.scheduling.TaskImpl.run(Tasks.kt:89)
	at kotlinx.coroutines.scheduling.CoroutineScheduler.runSafely(CoroutineScheduler.kt:586)
	at kotlinx.coroutines.scheduling.CoroutineScheduler$Worker.executeTask(CoroutineScheduler.kt:820)
	at kotlinx.coroutines.scheduling.CoroutineScheduler$Worker.runWorker(CoroutineScheduler.kt:717)
	at kotlinx.coroutines.scheduling.CoroutineScheduler$Worker.run(CoroutineScheduler.kt:704)
	Suppressed: kotlinx.coroutines.internal.DiagnosticCoroutineContextException: [StandaloneCoroutine{Cancelling}@4b1c2a8, Dispatchers.Main.immediate]`

// pctDeadlockStack is what Spring reports when two checkouts lock the same
// inventory rows in opposite order.
const pctDeadlockStack = `org.springframework.dao.CannotAcquireLockException: could not execute statement [ERROR: deadlock detected
  Detail: Process 31877 waits for ShareLock on transaction 9182736; blocked by process 31790.
Process 31790 waits for ShareLock on transaction 9182741; blocked by process 31877.
  Where: while updating tuple (412,7) in relation "inventory"] [update inventory set reserved=reserved+? where sku=? and available-reserved>=?]
	at org.springframework.orm.jpa.vendor.HibernateJpaDialect.convertHibernateAccessException(HibernateJpaDialect.java:299)
	at org.springframework.orm.jpa.vendor.HibernateJpaDialect.translateExceptionIfPossible(HibernateJpaDialect.java:244)
	at org.springframework.dao.support.PersistenceExceptionTranslationInterceptor.invoke(PersistenceExceptionTranslationInterceptor.java:137)
	at com.example.shop.checkout.InventoryRepository.reserve(InventoryRepository.java)
	at com.example.shop.checkout.CheckoutService.placeOrder(CheckoutService.java:39)
	at com.example.shop.checkout.CheckoutController.checkout(CheckoutController.java:27)
Caused by: org.postgresql.util.PSQLException: ERROR: deadlock detected
	at org.postgresql.core.v3.QueryExecutorImpl.receiveErrorResponse(QueryExecutorImpl.java:2733)
	at org.postgresql.jdbc.PgPreparedStatement.executeUpdate(PgPreparedStatement.java:155)
	at com.zaxxer.hikari.pool.HikariProxyPreparedStatement.executeUpdate(HikariProxyPreparedStatement.java)`

// pctSeedXSAndroidTimeouts: a flash sale started three hours ago. Every
// checkout reserves stock with UPDATE inventory inside a transaction that
// also calls the payment provider, so checkouts of the sale item queue on
// that row's lock for seconds; some run past OkHttp's 10s read timeout and
// the Android app, which does not catch the SocketTimeoutException, crashes.
// Expected: lock_transaction_contention. Distractors: an app bug (the crash
// is unhandled) and connection-pool exhaustion (there is no wait before the
// first query; the time is inside the UPDATE).
func pctSeedXSAndroidTimeouts(s *seed.Scenario) error {
	app := s.Project("shop-android", seed.Mobile, "android")
	apiProj := s.Project("commerce-api", seed.OTel)
	saleAt := s.Ago(3*time.Hour + 10*time.Minute)
	window := 2 * 24 * time.Hour
	api := apiProj.Service("checkout-api", seed.Java("spring")).Versions("7.4.2")

	checkout := api.Endpoint("POST", "/api/checkout")
	checkout.Traffic(seed.Daily(600), seed.Diurnal(), seed.Burst(saleAt, s.Now, 18), seed.Over(window))
	checkout.Detail(seed.DetailPolicy{Recent: 3 * time.Hour, Rate: 0.01, Max: 20})
	checkout.Handler(func(r *seed.Req) {
		sale := !r.Start().Before(saleAt)
		hot := sale && r.Chance(0.75)
		items := 1 + r.IntN(3)
		r.Gap(r.Ms(0.6, 1.5))
		r.DB("select c.id, ci.sku, ci.qty, ci.unit_price_cents from carts c join cart_items ci on ci.cart_id=c.id where c.id=?", r.LogNormal(3*time.Millisecond, 9*time.Millisecond), seed.Rows(items))
		for i := 0; i < items; i++ {
			wait := r.LogNormal(2*time.Millisecond, 7*time.Millisecond)
			if hot && i == 0 {
				wait = r.LogNormal(1800*time.Millisecond, 14*time.Second)
			}
			r.DB("update inventory set reserved=reserved+? where sku=? and available-reserved>=?", wait, seed.Rows(1))
			if hot && i == 1 && r.Chance(0.012) {
				r.Error(seed.Exc{Type: "org.springframework.dao.CannotAcquireLockException", Message: "could not execute statement [ERROR: deadlock detected]", Stack: pctDeadlockStack, Escaped: true})
				r.Log(seed.SevWarn, "SQL Error: 0, SQLState: 40P01", seed.A("logger.name", "org.hibernate.engine.jdbc.spi.SqlExceptionHelper"))
				r.Log(seed.SevError, "ERROR: deadlock detected\n  Detail: Process 31877 waits for ShareLock on transaction 9182736; blocked by process 31790.", seed.A("logger.name", "org.hibernate.engine.jdbc.spi.SqlExceptionHelper"))
				r.Status(500)
				return
			}
		}
		r.HTTP("POST", "https://api.payments.example/v1/charges", r.LogNormal(420*time.Millisecond, 1100*time.Millisecond), 201)
		r.DB("insert into orders (id, cart_id, total_cents, currency, payment_ref, status, created_at) values (?, ?, ?, ?, ?, ?, ?)", r.LogNormal(3*time.Millisecond, 9*time.Millisecond))
		r.DB("insert into order_lines (order_id, sku, qty, unit_price_cents) values (?, ?, ?, ?)", r.LogNormal(2*time.Millisecond, 8*time.Millisecond))
		r.Gap(r.Ms(0.4, 1.2))
	})
	cart := api.Endpoint("GET", "/api/cart")
	cart.Traffic(seed.Daily(900), seed.Diurnal(), seed.Burst(saleAt, s.Now, 6), seed.Over(window))
	cart.Handler(func(r *seed.Req) {
		r.Gap(r.Ms(0.5, 1.2))
		r.DB("select c.id, ci.sku, ci.qty, ci.unit_price_cents from carts c join cart_items ci on ci.cart_id=c.id where c.id=?", r.LogNormal(3*time.Millisecond, 9*time.Millisecond), seed.Rows(1+r.IntN(4)))
		r.Gap(r.Ms(1, 4)) // map to JSON
	})
	addItem := api.Endpoint("POST", "/api/cart/items")
	addItem.Traffic(seed.Daily(500), seed.Diurnal(), seed.Burst(saleAt, s.Now, 6), seed.Over(window))
	addItem.Handler(func(r *seed.Req) {
		r.Gap(r.Ms(0.5, 1.2))
		r.DB("select available, reserved from inventory where sku=?", r.LogNormal(2*time.Millisecond, 7*time.Millisecond), seed.Rows(1))
		r.DB("insert into cart_items (cart_id, sku, qty, unit_price_cents) values (?, ?, ?, ?) on conflict (cart_id, sku) do update set qty=cart_items.qty+excluded.qty", r.LogNormal(3*time.Millisecond, 9*time.Millisecond))
	})
	s.Background(api, seed.TypicalTraffic(4))
	api.Host().CPU(seed.Step(saleAt, seed.Flat(0.2, 0.3), seed.Flat(0.34, 0.46)))
	api.Host().Memory(seed.Flat(0.55, 0.63))

	// The Android side: checkouts that ran past 10s, for two in three (the
	// rest are web and iOS customers), crashed the app.
	const readTimeout = 10 * time.Second
	devices := []string{"Pixel 8 / Android 15", "Samsung SM-S921B / Android 14", "Pixel 7a / Android 15", "Xiaomi 23129RAA4G / Android 14", "Samsung SM-A546B / Android 14"}
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
		for i, r := range checkout.Requests() {
			if r.Start.Before(saleAt) || r.Duration() < readTimeout+200*time.Millisecond || r.Start.After(s.Ago(2*time.Minute)) || i%3 == 0 {
				continue
			}
			r.KeepDetail()
			newest = r
			lines := 0
			for _, sp := range r.Root.Children {
				if sp.Name == "UPDATE inventory" {
					lines++
				}
			}
			at := r.Start.Add(readTimeout + 15*time.Millisecond)
			cartAt := r.Start.Add(-time.Duration(25+i%50) * time.Second)
			app.Crash(seed.Crash{
				At: at, Stack: pctAndroidTimeoutStack, AppVersion: "5.18.0", Device: devices[i%len(devices)],
				Attributes: map[string]string{"screen": "CheckoutActivity"},
				Actions: []seed.Action{
					{Type: "navigation", At: cartAt.Add(-3 * time.Second), From: "ProductActivity", To: "CartActivity"},
					{Type: "network", At: cartAt, Method: "GET", URL: "https://api.shop.example/api/cart", DurationMs: float64(90 + i%120), Status: 200, RespBytes: 1800 + i%900},
					{Type: "navigation", At: cartAt.Add(4 * time.Second), From: "CartActivity", To: "CheckoutActivity"},
					{Type: "network", At: at, Method: "POST", URL: "https://api.shop.example/api/checkout", DurationMs: float64(readTimeout.Milliseconds()) + 4},
				},
				Logs: []seed.ActionLog{{At: r.Start.Add(-20 * time.Millisecond), Level: "info", Message: fmt.Sprintf("CheckoutViewModel: placing order (%d items)", lines)}},
			})
			crashes++
		}
	}
	s.Expose("androidCrashes", func() any { build(); return crashes })
	s.Expose("slowRequest", func() any {
		build()
		if newest == nil {
			return nil
		}
		return newest.Ref()
	})
	s.Expose("saleAt", pctRFC3339(saleAt))
	s.Expose("p95MsBefore", pctLazyMs(checkout, 0.95, s.Ago(window), saleAt))
	s.Expose("p95MsSale", pctLazyMs(checkout, 0.95, saleAt, s.Now))
	s.Expose("p50MsSale", pctLazyMs(checkout, 0.5, saleAt, s.Now))
	s.Expose("primaryProject", "shop-android")
	s.AfterFlush(func() error {
		hash, err := app.GroupMatching(24*time.Hour, "SocketTimeoutException")
		if err != nil {
			return err
		}
		s.Expose("hash", hash)
		return nil
	})
	return nil
}
