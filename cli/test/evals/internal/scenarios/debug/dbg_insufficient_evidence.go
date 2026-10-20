package debug

import (
	"fmt"
	"math"
	"time"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/scenarios"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/seed"
)

func init() { scenarios.Register("debug/insufficient-evidence", dbgSeedInsufficientEvidence) }

// dbgSeedInsufficientEvidence: one export in 170 of every format fails with the bare
// sentinel "report export failed". Export returns that sentinel from four
// steps and drops the cause; the failing traces show the rows query
// succeeding and the upload never starting, which leaves render and the
// uninstrumented credential fetch inside blob.Put (signing comes after the
// upload), and nothing records which one failed or why. Failures are spread
// evenly over formats and report sizes, so the tempting hypothesis (big xlsx
// renders) does not hold. The right deliverable is instrumentation.
func dbgSeedInsufficientEvidence(s *seed.Scenario) error {
	p := s.Project("analytics", seed.OTel)
	api := p.Service("reports-api", seed.Go("nethttp", "pgx")).Versions("2.11.0")
	const mod = "example.com/reports-api/internal"
	stack := dbgOtelStack(9022,
		mod+"/telemetry.init.0.func1({0x1d4c3a8, 0xc0004e60e0}, {0x1b6e0a0, 0xc000112f80})\n\t/app/internal/telemetry/otel.go:21 +0x8e",
		mod+"/httpx.Error({0x1d4f2c0, 0xc0004d2380}, 0xc0004c8780, {0x1b6e0a0, 0xc000112f80})\n\t/app/internal/httpx/errors.go:22 +0x9c",
		mod+"/reports.(*Handler).Export(0xc0001a4b40, {0x1d4f2c0, 0xc0004d2380}, 0xc0004c8780)\n\t/app/internal/reports/handler.go:44 +0x3c5",
		"net/http.HandlerFunc.ServeHTTP(0xc000189480, {0x1d4f2c0, 0xc0004d2380}, 0xc0004c8780)\n\t/usr/local/go/src/net/http/server.go:2306 +0x29",
		"net/http.(*ServeMux).ServeHTTP(0xc00012a0e0, {0x1d4f2c0, 0xc0004d2380}, 0xc0004c8500)\n\t/usr/local/go/src/net/http/server.go:2848 +0x1c4",
		mod+"/telemetry.init.0.func3.routeAttr.1({0x1d4f2c0?, 0xc0004d2380?}, 0xc0004c8500)\n\t/app/internal/telemetry/otel_http.go:29 +0x4c",
		"net/http.HandlerFunc.ServeHTTP(0xc0001c8a80?, {0x1d4f2c0?, 0xc0004d2380?}, 0x1b2a4e0?)\n\t/usr/local/go/src/net/http/server.go:2306 +0x29",
		"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp.(*middleware).serveHTTP(0xc0001c6000, {0x1d4e1a0, 0xc0004c6000}, 0xc0004c8280, {0x1d3b5a0, 0xc00012a0e0})\n\t/go/pkg/mod/go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp@v0.63.0/handler.go:180 +0x1a4c",
		"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp.NewMiddleware.func1.1({0x1d4e1a0?, 0xc0004c6000?}, 0x0?)\n\t/go/pkg/mod/go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp@v0.63.0/handler.go:67 +0x35",
		"net/http.HandlerFunc.ServeHTTP(0xc0001c8000?, {0x1d4e1a0?, 0xc0004c6000?}, 0xc0004c6001?)\n\t/usr/local/go/src/net/http/server.go:2306 +0x29",
		"net/http.serverHandler.ServeHTTP({0xc000180d20?}, {0x1d4e1a0?, 0xc0004c6000?}, 0x6?)\n\t/usr/local/go/src/net/http/server.go:3331 +0x8e",
	)
	formats := []string{"csv", "csv", "xlsx", "xlsx", "json"}
	perRow := map[string]float64{"csv": 0.004, "json": 0.006, "xlsx": 0.02} // ms per row

	// Failures are spread evenly: every 170th export of each format, so no
	// format (or report size) stands out.
	seen := map[string]int{}
	exp := api.Endpoint("POST", "/api/reports/{id}/export")
	exp.Traffic(seed.Daily(1100), seed.Diurnal(), seed.Over(7*24*time.Hour))
	exp.Detail(seed.DetailPolicy{Recent: 8 * time.Hour, Rate: 0.03, Max: 25})
	exp.Handler(func(r *seed.Req) {
		var id int
		path, _ := r.Request().Root.Attrs["url.path"].(string)
		fmt.Sscanf(path, "/api/reports/%d/export", &id)
		format := formats[r.IntN(len(formats))]
		rows := int(math.Min(80000, math.Max(3, float64(r.LogNormal(400*time.Millisecond, 30000*time.Millisecond))/float64(time.Millisecond))))
		r.RootAttr("report.id", id)
		r.RootAttr("report.format", format)
		r.Gap(r.Ms(0.3, 0.8))
		r.DB("SELECT columns FROM reports WHERE id = $1", r.Ms(1, 3))
		r.DB("SELECT values FROM report_rows WHERE report_id = $1 ORDER BY position", r.Ms(2, 6)+time.Duration(float64(rows)*0.003*float64(time.Millisecond)), seed.Rows(rows))
		renderMs := float64(rows)*perRow[format]*(0.8+0.4*r.Float()) + 0.3
		seen[format]++
		if seen[format]%170 == 97 {
			// Somewhere between the rows and the upload; nothing records where.
			r.Gap(time.Duration(renderMs * r.Float() * float64(time.Millisecond)))
			r.Error(seed.Exc{Type: "*errors.errorString", Message: "report export failed", Stack: stack})
			r.Status(500)
			r.Gap(r.Ms(0.1, 0.3))
			return
		}
		r.Gap(time.Duration(renderMs * float64(time.Millisecond)))
		bytes := rows*48 + 600
		if format == "xlsx" {
			bytes = rows*22 + 4800
		}
		r.HTTP("PUT", fmt.Sprintf("https://storage.example/reports-exports/exports/%d/%s.%s", id, r.Start().UTC().Format("20060102T150405Z"), format),
			r.Ms(25, 60)+time.Duration(float64(bytes)/40000*float64(time.Millisecond)), 200, seed.A("http.request.body.size", bytes))
		r.Gap(r.Ms(0.1, 0.4))
	})

	list := api.Endpoint("GET", "/api/reports")
	list.Traffic(seed.Daily(400), seed.Diurnal(), seed.Over(7*24*time.Hour))
	list.Handler(func(r *seed.Req) {
		r.Gap(r.Ms(0.3, 0.8))
		r.DB("SELECT id, name, owner_id, updated_at FROM reports WHERE workspace_id = $1 ORDER BY updated_at DESC LIMIT 50", r.Ms(2, 7), seed.Rows(1+r.IntN(50)))
		r.Gap(r.Ms(0.3, 1.2))
	})
	s.Background(api, seed.TypicalTraffic(4))
	api.Host().CPU(seed.Flat(0.2, 0.34))
	api.Host().Memory(seed.Flat(0.44, 0.51))

	s.Expose("failures", func() any {
		byFormat := map[string]int{}
		for _, rq := range exp.Requests() {
			if rq.Status == 500 {
				byFormat[rq.Root.Attrs["report.format"].(string)]++
			}
		}
		return byFormat
	})
	s.AfterFlush(func() error {
		hash, err := p.GroupMatching(8*24*time.Hour, "reports.(*Handler).Export")
		if err != nil {
			return err
		}
		s.Expose("hash", hash)
		return nil
	})
	return nil
}
