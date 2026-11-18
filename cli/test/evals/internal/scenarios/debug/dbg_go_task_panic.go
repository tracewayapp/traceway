package debug

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/scenarios"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/seed"
)

func init() { scenarios.Register("debug/go-task-panic", dbgSeedGoTaskPanic) }

// dbgLabelPanicStack is debug.Stack() in the worker's deferred recover
// (testdata/repos/fulfillment-worker-go built with -tags otel on Go 1.26):
// the consumer loop runs on the main goroutine.
const dbgLabelPanicStack = `goroutine 1 [running]:
runtime/debug.Stack()
	/usr/local/go/src/runtime/debug/stack.go:26 +0x5e
example.com/fulfillment/internal/worker.(*Worker).handle.func1()
	/app/internal/worker/worker.go:61 +0x58
panic({0xd58f00?, 0x334008750570?})
	/usr/local/go/src/runtime/panic.go:860 +0x13a
example.com/fulfillment/internal/labels.(*Service).Handle(0x334008822510, {0xe09030, 0x3340087f11d0}, {{0x33400871d120?, 0x334008727060?}})
	/app/internal/labels/labels.go:41 +0x46c
example.com/fulfillment/internal/worker.(*Worker).handle(0x3340087dbe98, {0xe092b0, 0x33400872f200}, 0x33400878c870)
	/app/internal/worker/worker.go:70 +0x585
example.com/fulfillment/internal/worker.(*Worker).Run(0x3340087dbe98, {0xe092b0, 0x33400872f200})
	/app/internal/worker/worker.go:42 +0x131
main.main()
	/app/cmd/worker/main.go:32 +0x44c
`

// dbgLabelErrorStack is span.RecordError(err, trace.WithStackTrace(true)) in
// Worker.handle for a failed label purchase.
const dbgLabelErrorStack = `goroutine 1 [running]:
go.opentelemetry.io/otel/sdk/trace.recordStackTrace(...)
	/go/pkg/mod/go.opentelemetry.io/otel/sdk@v1.38.0/trace/span.go:570
go.opentelemetry.io/otel/sdk/trace.(*recordingSpan).RecordError(0x33b83e84780, {0xdfd9a0, 0x33b84144040}, {0x33b8411a110, 0x1, 0x1})
	/go/pkg/mod/go.opentelemetry.io/otel/sdk@v1.38.0/trace/span.go:552 +0x585
example.com/fulfillment/internal/telemetry.init.0.func3({0xe09030?, 0x33b83ea0300?}, {0xdfd9a0, 0x33b84144040})
	/app/internal/telemetry/otel.go:57 +0xe5
example.com/fulfillment/internal/worker.(*Worker).handle(0x33b83f6fe98, {0xe092b0, 0x33b83e37240}, 0x33b83eb8000)
	/app/internal/worker/worker.go:71 +0x5b7
example.com/fulfillment/internal/worker.(*Worker).Run(0x33b83f6fe98, {0xe092b0, 0x33b83e37240})
	/app/internal/worker/worker.go:42 +0x131
main.main()
	/app/cmd/worker/main.go:32 +0x44c
`

// dbgSeedGoTaskPanic: the label worker consumes paid orders. v2.4.0 (46h
// ago) builds one multi-parcel label request from order.Packages[0], so an
// order of only digital items (no packages) panics inside the consumer.
// v2.3.2 bought one label per package. Distractor: carrier 429s during a
// sale peak five days ago, same task.
func dbgSeedGoTaskPanic(s *seed.Scenario) error {
	deployAt := s.Ago(46 * time.Hour).Truncate(time.Minute).Add(-11 * time.Minute)
	peakFrom := s.Ago(5*24*time.Hour + 6*time.Hour).Truncate(time.Hour)
	peakTo := peakFrom.Add(40 * time.Minute)
	p := s.Project("fulfillment", seed.OTel)
	worker := p.Service("label-worker", seed.Go("nethttp", "sql")).Versions("2.3.2", seed.Deploy{At: deployAt, Version: "2.4.0"})
	const labelsURL = "https://api.carrier.example/v2/labels"
	db := []seed.KV{seed.A("db.namespace", "fulfillment"), seed.A("server.address", "fulfillment-db.internal")}

	task := worker.Task("process fulfillment.orders")
	task.Traffic(seed.Daily(1400), seed.Diurnal(), seed.Burst(peakFrom, peakTo, 6), seed.Over(7*24*time.Hour))
	task.Detail(seed.DetailPolicy{Recent: 8 * time.Hour, Rate: 0.03, Max: 30})
	offset := int64(8_812_000)
	task.Handler(func(r *seed.Req) {
		r.Request().Root.Scope = "example.com/fulfillment/internal/worker"
		offset += int64(1 + r.IntN(3))
		orderID := fmt.Sprintf("ord_%07d", 3_100_000+r.IntN(900_000))
		r.RootAttr("messaging.system", "kafka")
		r.RootAttr("messaging.destination.name", "fulfillment.orders")
		r.RootAttr("messaging.consumer.group.name", "label-printer")
		r.RootAttr("messaging.kafka.message.key", orderID)
		r.RootAttr("messaging.kafka.offset", offset)
		r.Gap(r.Ms(0.2, 0.6))
		r.DB("SELECT ship_name, ship_line1, ship_city, ship_postal_code, ship_country FROM orders WHERE id = $1", r.Ms(1, 4), db...)
		digital := r.Chance(0.016)
		packages := 1 + r.IntN(3)
		if digital {
			packages = 0
		}
		r.DB("SELECT weight_grams, service FROM packages WHERE order_id = $1 ORDER BY position", r.Ms(1, 3), append(db, seed.Rows(packages))...)
		if digital && r.Version() == "2.4.0" {
			r.Error(seed.Exc{Type: "runtime.boundsError", Message: "runtime error: index out of range [0] with length 0", Stack: dbgLabelPanicStack, Escaped: true})
			r.Log(seed.SevError, "panic handling message", seed.A("topic", "fulfillment.orders"), seed.A("offset", offset), seed.A("panic", "runtime error: index out of range [0] with length 0"))
			r.Gap(r.Ms(0.1, 0.3))
			return
		}
		at := r.Start()
		peak := !at.Before(peakFrom) && at.Before(peakTo)
		labels := 1
		if r.Version() == "2.3.2" {
			labels = packages
		}
		for i := 1; i <= labels; i++ {
			if peak && r.Chance(0.25) {
				r.HTTP("POST", labelsURL, r.Ms(40, 120), 429).Name = "HTTP POST"
				msg := fmt.Sprintf("create label %d for order %s: POST %s: 429 Too Many Requests", i, orderID, labelsURL)
				r.Error(seed.Exc{Type: "*fmt.wrapError", Message: msg, Stack: dbgLabelErrorStack})
				r.Log(seed.SevError, "message failed", seed.A("topic", "fulfillment.orders"), seed.A("offset", offset), seed.A("err", msg))
				return
			}
			r.HTTP("POST", labelsURL, r.LogNormal(320*time.Millisecond, 1400*time.Millisecond), 201).Name = "HTTP POST"
			r.DB("INSERT INTO labels (order_id, tracking_number, label_url, created_at) VALUES ($1, $2, $3, now())", r.Ms(1, 3), db...)
		}
		r.Gap(r.Ms(0.1, 0.3))
	})

	api := p.Service("fulfillment-api", seed.Go("chi", "pgx")).Versions("1.9.0")
	ship := api.Endpoint("GET", "/api/shipments/{id}")
	ship.Traffic(seed.Daily(600), seed.Diurnal(), seed.Over(7*24*time.Hour))
	ship.Handler(func(r *seed.Req) {
		r.Gap(r.Ms(0.3, 0.8))
		r.DB("SELECT s.id, s.order_id, s.status, l.tracking_number FROM shipments s LEFT JOIN labels l ON l.order_id = s.order_id WHERE s.id = $1", r.Ms(2, 5))
		r.Gap(r.Ms(0.2, 0.8))
	})
	byOrder := api.Endpoint("GET", "/api/orders/{orderId}/shipments")
	byOrder.Traffic(seed.Daily(350), seed.Diurnal(), seed.Over(7*24*time.Hour))
	byOrder.Handler(func(r *seed.Req) {
		r.Gap(r.Ms(0.3, 0.8))
		r.DB("SELECT s.id, s.status, l.tracking_number, l.label_url FROM shipments s LEFT JOIN labels l ON l.order_id = s.order_id WHERE s.order_id = $1", r.Ms(2, 6), seed.Rows(1+r.IntN(3)))
		r.Gap(r.Ms(0.2, 0.8))
	})
	s.Background(api, seed.TrafficOpts{})
	worker.Host().CPU(seed.Flat(0.12, 0.24))
	worker.Host().Memory(seed.Flat(0.41, 0.47))

	s.Expose("deployAt", deployAt.Format(time.RFC3339))
	s.AfterFlush(func() error {
		hash, err := p.GroupMatching(8*24*time.Hour, "labels.(*Service).Handle")
		if err != nil {
			return err
		}
		s.Expose("hash", hash)
		other, err := p.GroupMatching(8*24*time.Hour, "429 Too Many Requests")
		if err != nil {
			return err
		}
		s.Expose("otherHash", other)
		occs, err := dbgOccurrences(s, p, hash, 100)
		if err != nil {
			return err
		}
		if len(occs) == 0 || len(occs) >= 100 {
			return fmt.Errorf("want 1-99 occurrences of the panic, got %d", len(occs))
		}
		// The new-error notification fires for the group's first occurrence.
		first := occs[len(occs)-1]
		ref := dbgOccRef(first)
		ref["stack"] = strings.TrimRight(first.StackTrace, "\n")
		ref["server"] = first.ServerName
		keys := make([]string, 0, len(first.Attributes))
		for k := range first.Attributes {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		var attrs []string
		for _, k := range keys {
			attrs = append(attrs, "  "+k+": "+first.Attributes[k])
		}
		ref["attrs"] = strings.Join(attrs, "\n")
		s.Expose("occ", ref)
		s.Expose("count", len(occs))
		return nil
	})
	return nil
}
