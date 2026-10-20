package debug

import (
	"fmt"
	"time"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/scenarios"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/seed"
)

func init() {
	scenarios.Register("debug/go-nil-regression", seedGoNilRegression)
}

// billingPanicStack is what Recover records for a customer without a billing
// address: debug.Stack() in the deferred recover of the production build
// (Go 1.26, -tags otel), where main inlines Recover and AccessLog.
const billingPanicStack = `goroutine 1183 [running]:
runtime/debug.Stack()
	/usr/local/go/src/runtime/debug/stack.go:26 +0x5e
main.main.Recover.func2.1()
	/app/internal/invoices/handler.go:48 +0x6f
panic({0xcc6220?, 0x15f8ca0?})
	/usr/local/go/src/runtime/panic.go:860 +0x13a
example.com/billing-api/internal/invoices.formatAddress(0x0)
	/app/internal/invoices/render.go:37 +0x1a
example.com/billing-api/internal/invoices.render(0x2ba2cdcdf260)
	/app/internal/invoices/render.go:27 +0x91
example.com/billing-api/internal/invoices.(*Handler).Get(0x2ba2cdbff080, {0xe5a010, 0x2ba2cdcaa7c8}, 0x2ba2ce086780)
	/app/internal/invoices/handler.go:37 +0x22b
net/http.HandlerFunc.ServeHTTP(0x2ba2cde6a240?, {0xe5a010?, 0x2ba2cdcaa7c8?}, 0x426d05?)
	/usr/local/go/src/net/http/server.go:2306 +0x29
net/http.(*ServeMux).ServeHTTP(0x7f6235edbbf0?, {0xe5a010, 0x2ba2cdcaa7c8}, 0x2ba2ce086780)
	/usr/local/go/src/net/http/server.go:2848 +0x1c7
main.main.Recover.func2({0xe5a010?, 0x2ba2cdcaa7c8?}, 0x2ba2cdcaa7c8?)
	/app/internal/invoices/handler.go:53 +0x6c
net/http.HandlerFunc.ServeHTTP(0x2ba2cdc84b40?, {0xe5a010?, 0x2ba2cdcaa7c8?}, 0x7f61ee077d18?)
	/usr/local/go/src/net/http/server.go:2306 +0x29
main.main.AccessLog.func3({0xe5be60, 0x2ba2cdcad3e0}, 0x2ba2ce086780)
	/app/internal/telemetry/access.go:26 +0xd5
net/http.HandlerFunc.ServeHTTP(0x2ba2cde07630?, {0xe5be60?, 0x2ba2cdcad3e0?}, 0x48f972?)
	/usr/local/go/src/net/http/server.go:2306 +0x29
example.com/billing-api/internal/telemetry.init.0.func1.routeAttr.1({0xe5be60?, 0x2ba2cdcad3e0?}, 0x2ba2ce086780)
	/app/internal/telemetry/otel.go:77 +0x4c
net/http.HandlerFunc.ServeHTTP(0xe5c930?, {0xe5be60?, 0x2ba2cdcad3e0?}, 0xe4b620?)
	/usr/local/go/src/net/http/server.go:2306 +0x29
go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp.(*middleware).serveHTTP(0x2ba2cde69c70, {0xe5a100, 0x2ba2ce0780f0}, 0x2ba2ce086640, {0xe4fe60, 0x2ba2cdbdaca8})
	/go/pkg/mod/go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp@v0.63.0/handler.go:180 +0x1258
go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp.NewMiddleware.func1.1({0xe5a100?, 0x2ba2ce0780f0?}, 0x4bbc33?)
	/go/pkg/mod/go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp@v0.63.0/handler.go:67 +0x35
net/http.HandlerFunc.ServeHTTP(0x486c19?, {0xe5a100?, 0x2ba2ce0780f0?}, 0x2ba2cdc41b30?)
	/usr/local/go/src/net/http/server.go:2306 +0x29
net/http.serverHandler.ServeHTTP({0x2ba2cdcae940?}, {0xe5a100?, 0x2ba2ce0780f0?}, 0x1?)
	/usr/local/go/src/net/http/server.go:3331 +0x8e
net/http.(*conn).serve(0x2ba2ce07cfc0, {0xe5c930, 0x2ba2cde3c600})
	/usr/local/go/src/net/http/server.go:2087 +0x690
created by net/http.(*Server).Serve in goroutine 1
	/usr/local/go/src/net/http/server.go:3484 +0x485
`

// billingMailerStack is span.RecordError(err, trace.WithStackTrace(true)) in
// Sender.Send when the mailer call hits its 10s deadline; the SDK captures at
// most 2048 bytes of stack.
const billingMailerStack = `goroutine 2271 [running]:
go.opentelemetry.io/otel/sdk/trace.recordStackTrace(...)
	/go/pkg/mod/go.opentelemetry.io/otel/sdk@v1.38.0/trace/span.go:570
go.opentelemetry.io/otel/sdk/trace.(*recordingSpan).RecordError(0x2ba2ce09e3c0, {0xe50760, 0x2ba2cde3c270}, {0x2ba2cdd82240, 0x1, 0x1})
	/go/pkg/mod/go.opentelemetry.io/otel/sdk@v1.38.0/trace/span.go:552 +0x585
example.com/billing-api/internal/telemetry.init.0.func5({0xe5c9d8?, 0x2ba2cdcde380?}, {0xe50760, 0x2ba2cde3c270})
	/app/internal/telemetry/otel.go:60 +0xe5
example.com/billing-api/internal/invoices.(*Sender).Send(0x2ba2cdc1aec0, {0xe5a010, 0x2ba2cdd92168}, 0x2ba2ce086140)
	/app/internal/invoices/send.go:47 +0x43d
net/http.HandlerFunc.ServeHTTP(0x2ba2cde6a240?, {0xe5a010?, 0x2ba2cdd92168?}, 0x426cd4?)
	/usr/local/go/src/net/http/server.go:2306 +0x29
net/http.(*ServeMux).ServeHTTP(0x7f6235edbbf0?, {0xe5a010, 0x2ba2cdd92168}, 0x2ba2ce086140)
	/usr/local/go/src/net/http/server.go:2848 +0x1c7
main.main.Recover.func2({0xe5a010?, 0x2ba2cdd92168?}, 0x4bbc01?)
	/app/internal/invoices/handler.go:53 +0x6c
net/http.HandlerFunc.ServeHTTP(0x2ba2cdc15430?, {0xe5a010?, 0x2ba2cdd92168?}, 0xd98ac0?)
	/usr/local/go/src/net/http/server.go:2306 +0x29
main.main.AccessLog.func3({0xe5be60, 0x2ba2cdd000c0}, 0x2ba2ce086140)
	/app/internal/telemetry/access.go:26 +0xd5
net/http.HandlerFunc.ServeHTTP(0x2ba2cdc15630?, {0xe5be60?, 0x2ba2cdd000c0?}, 0x2ba2cddd0140?)
	/usr/local/go/src/net/http/server.go:2306 +0x29
example.com/billing-api/internal/telemetry.init.0.func1.routeAttr.1({0xe5be60?, 0x2ba2cdcad3e0?}, 0x2ba2ce086780)
	/app/internal/telemetry/otel.go:77 +0x4c
net/http.HandlerFunc.ServeHTTP(0xe5c930?, {0xe5be60?, 0x2ba2cdcad3e0?}, 0xe4b620?)
	/usr/local/go/src/net/http/server.go:2306 +0x29
go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp.(*middleware).serveHTTP(0x2ba2cde69c70, {0xe5a100, 0x2ba2ce0780f0}, 0x2ba2ce086640, {0xe4fe60, 0x2ba2cdbdaca8})
	/go/pkg/mod/go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp@v0.63.0/handler.go:180 +0x1258
go.opentelemetry.i`

const (
	billingInvoiceQuery = "SELECT i.id, i.number, i.issued_at, i.total_cents, i.currency, c.id, c.name, c.email FROM invoices i JOIN customers c ON c.id = i.customer_id WHERE i.id = $1"
	billingAddressQuery = "SELECT line1, line2, city, postal_code, country FROM billing_addresses WHERE customer_id = $1"
	billingListQuery    = "SELECT id, number, issued_at, total_cents, currency FROM invoices WHERE customer_id = $1 ORDER BY issued_at DESC LIMIT 50"
	billingMailerURL    = "https://api.mailer.example/v3/messages"
)

// billingLoad is SQLStore.Invoice: the invoice with its customer, then the
// customer's billing address, which has no row for customers who never
// entered one.
func billingLoad(r *seed.Req, noAddress bool) {
	r.DB(billingInvoiceQuery, r.Ms(2, 5))
	r.DB(billingAddressQuery, r.Ms(1, 3), seed.Rows(map[bool]int{true: 0, false: 1}[noAddress]))
}

// billingService is the billing-api of testdata/repos/billing-api-go, shared
// by the billing debug cases: v1.8.2, deployed at deployAt, renders the
// billing address without a nil check, so customers who never entered one
// get a panic. A non-zero failStart adds a database failover of 18 minutes
// (connections refused, then reads cancelled on the recovering standby).
func billingService(s *seed.Scenario, deployAt, failStart time.Time) (*seed.Project, *seed.Service, *seed.Endpoint) {
	p := s.Project("billing", seed.OTel)
	api := p.Service("billing-api", seed.Go("nethttp", "sql")).Versions("1.8.1", seed.Deploy{At: deployAt, Version: "1.8.2"})

	get := api.Endpoint("GET", "/api/invoices/{id}")
	get.Traffic(seed.Daily(700), seed.Diurnal(), seed.Over(7*24*time.Hour))
	get.Detail(seed.DetailPolicy{Recent: 6 * time.Hour, Rate: 0.03, Max: 10})
	get.Handler(func(r *seed.Req) {
		r.RootAttr("enduser.id", fmt.Sprint(4000+r.IntN(9000)))
		r.Gap(r.Ms(0.3, 0.9))
		noAddress := r.Chance(0.14)
		billingLoad(r, noAddress)
		if r.Version() == "1.8.2" && noAddress {
			r.Error(seed.Exc{Type: "runtime.errorString", Message: "runtime error: invalid memory address or nil pointer dereference", Stack: billingPanicStack, Escaped: true})
			r.Log(seed.SevError, "panic serving request", seed.A("path", r.Request().Root.Attrs["url.path"]), seed.A("panic", "runtime error: invalid memory address or nil pointer dereference"))
			r.Status(500)
			r.Gap(r.Ms(0.2, 0.5))
			return
		}
		r.Gap(r.Ms(0.6, 1.7))
	})

	// Distractor: invoice e-mails occasionally time out at the mailer, since
	// long before the release.
	send := api.Endpoint("POST", "/api/invoices/{id}/send")
	send.Traffic(seed.Daily(120), seed.Diurnal(), seed.Over(7*24*time.Hour))
	send.Handler(func(r *seed.Req) {
		r.Gap(r.Ms(0.3, 0.9))
		billingLoad(r, r.Chance(0.14))
		r.Gap(r.Ms(0.1, 0.3))
		if r.Chance(0.02) {
			sp := r.HTTP("POST", billingMailerURL, 10*time.Second, 0)
			sp.Name = "HTTP POST"
			delete(sp.Attrs, "http.response.status_code")
			sp.Attrs["error.type"] = "context.deadlineExceededError"
			sp.Status, sp.StatusMsg = seed.StatusError, "context deadline exceeded"
			r.Error(seed.Exc{Type: "*url.Error", Message: `Post "` + billingMailerURL + `": context deadline exceeded`, Stack: billingMailerStack})
			r.Status(502)
			return
		}
		r.HTTP("POST", billingMailerURL, r.LogNormal(140*time.Millisecond, 500*time.Millisecond), 202).Name = "HTTP POST"
		r.Status(202)
	})

	failEnd := failStart.Add(18 * time.Minute)
	list := api.Endpoint("GET", "/api/customers/{id}/invoices")
	shapes := []seed.Shape{seed.Daily(900), seed.Diurnal(), seed.Over(7 * 24 * time.Hour)}
	if !failStart.IsZero() {
		shapes = append(shapes, seed.Burst(failStart, failEnd, 6))
	}
	list.Traffic(shapes...)
	list.Detail(seed.DetailPolicy{Recent: 6 * time.Hour, Rate: 0.02, Max: 8})
	list.Handler(func(r *seed.Req) {
		r.Gap(r.Ms(0.3, 0.9))
		at := r.Start()
		inFailover := !failStart.IsZero() && !at.Before(failStart) && at.Before(failEnd)
		switch {
		case inFailover && at.Before(failStart.Add(6*time.Minute)):
			// database/sql dials before the query runs, so otelsql records
			// the refused dial on its connect span.
			sp := r.DB(billingListQuery, r.Ms(1, 3))
			sp.Name = "sql.connector.connect"
			for _, k := range []string{"db.query.text", "db.operation.name", "db.collection.name"} {
				delete(sp.Attrs, k)
			}
			dbgSpanError(r, sp, seed.Exc{Type: "*pgconn.ConnectError",
				Message: "failed to connect to `user=billing database=billing`: 10.0.4.12:5432 (billing-db.internal): dial error: dial tcp 10.0.4.12:5432: connect: connection refused"})
			r.Status(500)
			return
		case inFailover && r.Chance(0.4):
			dbgSpanError(r, r.DB(billingListQuery, r.Ms(20, 60)), seed.Exc{Type: "*pgconn.PgError", Message: "ERROR: canceling statement due to conflict with recovery (SQLSTATE 40001)"})
			r.Status(500)
			return
		}
		r.DB(billingListQuery, r.Ms(2, 6), seed.Rows(1+r.IntN(40)))
		r.Gap(r.Ms(0.3, 1.2))
	})
	s.Background(api, seed.TrafficOpts{})
	return p, api, get
}

// seedGoNilRegression: a nil-pointer panic in invoice rendering that starts
// with the v1.8.2 deploy 30 hours ago.
func seedGoNilRegression(s *seed.Scenario) error {
	deployAt := s.Ago(30 * time.Hour).Truncate(time.Minute).Add(-7 * time.Minute)
	p, _, get := billingService(s, deployAt, time.Time{})
	s.Expose("deployAt", deployAt.Format(time.RFC3339))
	s.Expose("lastFailure", get.Pick(func(r *seed.Request) bool { return r.Status == 500 }))
	s.AfterFlush(func() error {
		hash, err := p.GroupMatching(8*24*time.Hour, "formatAddress")
		if err != nil {
			return err
		}
		s.Expose("hash", hash)
		other, err := p.GroupMatching(8*24*time.Hour, "mailer.example")
		if err != nil {
			return err
		}
		s.Expose("otherHash", other)
		return nil
	})
	return nil
}
