package seed

import (
	"fmt"
	"math"
	"regexp"
	"time"
)

// TrafficOpts configures background noise.
type TrafficOpts struct {
	Endpoints int
	// PerDay is the request rate per background endpoint (capped so the
	// background stays under ~5000 requests per service).
	PerDay int
	// Window defaults to the service's longest scenario window.
	Window time.Duration
	// NoErrors suppresses the low-rate unrelated client error.
	NoErrors bool
	// Hosts sets default host metrics (CPU and memory) when the scenario
	// did not set its own.
	NoHostMetrics bool
}

// TypicalTraffic is n healthy endpoints, a low-rate unrelated error and flat
// host metrics.
func TypicalTraffic(n int) TrafficOpts { return TrafficOpts{Endpoints: n, PerDay: 150} }

// backgroundRoutes is a neutral pool of routes for healthy traffic.
var backgroundRoutes = [][2]string{
	{"GET", "/api/me"}, {"GET", "/api/products"}, {"GET", "/api/products/:id"}, {"POST", "/api/cart/items"},
	{"GET", "/api/cart"}, {"GET", "/api/categories"}, {"POST", "/api/auth/session"}, {"GET", "/api/notifications"},
	{"PUT", "/api/users/:id/preferences"}, {"POST", "/api/events"}, {"GET", "/api/addresses"}, {"DELETE", "/api/cart/items/:id"},
	{"GET", "/api/inventory/:sku"}, {"GET", "/api/recommendations"}, {"POST", "/api/checkout/quote"}, {"GET", "/api/stores/:id/hours"},
	{"GET", "/api/promotions"}, {"POST", "/api/newsletter/subscriptions"}, {"GET", "/api/shipping/rates"}, {"GET", "/api/help/articles"},
}

var backgroundTables = []string{"users", "products", "categories", "carts", "cart_items", "notifications", "addresses", "inventory", "stores", "promotions"}

// Background adds healthy traffic around the scenario so the signal has to
// be found, not handed over.
func (s *Scenario) Background(svc *Service, o TrafficOpts) {
	taken := map[string]bool{}
	for _, e := range svc.endpoints {
		taken[e.Name()] = true
	}
	win := o.Window
	if win == 0 {
		win = svc.metricsWindow()
	}
	days := math.Max(1, win.Hours()/24)
	rate := o.PerDay
	if o.Endpoints > 0 && float64(rate)*float64(o.Endpoints)*days > 5000 {
		rate = int(5000 / (float64(o.Endpoints) * days))
		if rate < 8 {
			rate = 8
		}
	}
	added := 0
	for i := 0; added < o.Endpoints && i < len(backgroundRoutes)*2; i++ {
		rt := backgroundRoutes[(i*7+len(svc.Name))%len(backgroundRoutes)]
		route := routeSyntax(svc.Stack, rt[1])
		name := rt[0] + " " + route
		if taken[name] {
			continue
		}
		taken[name] = true
		added++
		tables := []string{backgroundTables[(i*3)%len(backgroundTables)], backgroundTables[(i*5+1)%len(backgroundTables)]}
		method := rt[0]
		medianDB := time.Duration(2+(i%5)) * time.Millisecond
		withError := !o.NoErrors && method == "POST" && added <= 6
		e := svc.Endpoint(rt[0], route)
		e.Traffic(Daily(rate), Diurnal(), Over(win))
		e.Detail(DetailPolicy{Recent: 6 * time.Hour, Rate: 0.01, Max: 8})
		e.Handler(func(r *Req) {
			r.Gap(r.Ms(0.3, 1.2))
			if withError && r.Chance(0.004) {
				r.Compute("decode request", r.Ms(0.1, 0.4))
				r.Error(clientDecodeError(svc.Stack))
				r.Status(400)
				r.Gap(r.Ms(0.2, 0.6))
				return
			}
			r.DB(placeholders(svc.Stack, selectFor(method, tables[0])), r.LogNormal(medianDB, medianDB*6))
			if r.Chance(0.6) {
				r.DB(placeholders(svc.Stack, selectFor("GET", tables[1])), r.LogNormal(medianDB, medianDB*5))
			}
			r.Compute("encode response", r.Ms(0.4, 2.5))
			r.Gap(r.Ms(0.2, 0.8))
		})
	}
	if !o.NoHostMetrics {
		svc.defaultHost = true
	}
}

func selectFor(method, table string) string {
	switch method {
	case "POST":
		return fmt.Sprintf("INSERT INTO %s (id, payload, created_at) VALUES ($1, $2, now())", table)
	case "PUT":
		return fmt.Sprintf("UPDATE %s SET payload = $2, updated_at = now() WHERE id = $1", table)
	case "DELETE":
		return fmt.Sprintf("DELETE FROM %s WHERE id = $1", table)
	default:
		return fmt.Sprintf("SELECT id, name, updated_at FROM %s WHERE id = $1", table)
	}
}

var (
	routeParamRe = regexp.MustCompile(`:(\w+)`)
	sqlParamRe   = regexp.MustCompile(`\$\d+`)
)

// routeSyntax writes a background route's parameters the way the service's
// router reports http.route: Express and gin keep ":id", Flask and Django
// "<id>", every other framework "{id}".
func routeSyntax(st Stack, route string) string {
	switch {
	case st.Runtime == "node" || st.Framework == "go/gin":
		return route
	case st.Framework == "python/flask" || st.Framework == "python/django":
		return routeParamRe.ReplaceAllString(route, "<$1>")
	default:
		return routeParamRe.ReplaceAllString(route, "{$1}")
	}
}

// placeholders writes a background query's parameters the way the driver
// reports db.query.text: "$1" for Postgres drivers that number them, "%s"
// for the Python drivers, "?" for JDBC, PDO and MySQL.
func placeholders(st Stack, q string) string {
	switch {
	case st.Runtime == "python":
		return sqlParamRe.ReplaceAllString(q, "%s")
	case st.DBSystem == "mysql" || st.Runtime == "jvm" || st.Runtime == "php":
		return sqlParamRe.ReplaceAllString(q, "?")
	default:
		return q
	}
}

// clientDecodeError is a benign 400: a client sent malformed JSON.
func clientDecodeError(st Stack) Exc {
	switch st.StackFormat {
	case "python":
		return Exc{Type: "json.decoder.JSONDecodeError", Message: "Expecting property name enclosed in double quotes: line 1 column 2 (char 1)",
			Stack: "Traceback (most recent call last):\n  File \"/usr/local/lib/python3.12/site-packages/starlette/requests.py\", line 252, in json\n    self._json = json.loads(body)\n  File \"/usr/local/lib/python3.12/json/__init__.py\", line 346, in loads\n    return _default_decoder.decode(s)\njson.decoder.JSONDecodeError: Expecting property name enclosed in double quotes: line 1 column 2 (char 1)"}
	case "node":
		return Exc{Type: "SyntaxError", Message: "Expected property name or '}' in JSON at position 1",
			Stack: "SyntaxError: Expected property name or '}' in JSON at position 1\n    at JSON.parse (<anonymous>)\n    at parse (/app/node_modules/body-parser/lib/types/json.js:92:19)\n    at /app/node_modules/body-parser/lib/read.js:128:18"}
	case "java":
		return Exc{Type: "com.fasterxml.jackson.core.JsonParseException", Message: "Unexpected character ('}' (code 125)): was expecting double-quote to start field name",
			Stack: "com.fasterxml.jackson.core.JsonParseException: Unexpected character ('}' (code 125)): was expecting double-quote to start field name\n\tat com.fasterxml.jackson.core.JsonParser._constructError(JsonParser.java:2481)\n\tat org.springframework.http.converter.json.AbstractJackson2HttpMessageConverter.readJavaType(AbstractJackson2HttpMessageConverter.java:406)"}
	case "php":
		return Exc{Type: "JsonException", Message: "Syntax error", Stack: "JsonException: Syntax error in /var/www/vendor/laravel/framework/src/Illuminate/Http/Request.php:671\nStack trace:\n#0 /var/www/vendor/laravel/framework/src/Illuminate/Http/Request.php(671): json_decode()"}
	case "dotnet":
		return Exc{Type: "System.Text.Json.JsonException", Message: "'}' is an invalid start of a property name. Path: $ | LineNumber: 0 | BytePositionInLine: 1.",
			Stack: "System.Text.Json.JsonException: '}' is an invalid start of a property name.\n   at System.Text.Json.ThrowHelper.ReThrowWithPath(ReadStack& state, JsonReaderException ex)\n   at Microsoft.AspNetCore.Http.Json.HttpRequestJsonExtensions.ReadFromJsonAsync[TValue](HttpRequest request)"}
	default:
		return Exc{Type: "*json.SyntaxError", Message: "invalid character '}' looking for beginning of object key string", Stack: goDecodeStack(st)}
	}
}

// goDecodeStack is where a Go handler records a JSON decode error: the SDK's
// RecordError frames, then the router that called the handler.
func goDecodeStack(st Stack) string {
	s := "goroutine 4113 [running]:\n"
	if st.Framework == "go/gin" {
		return s + "github.com/gin-gonic/gin.(*Context).ShouldBindJSON(0xc000521100, {0x1a2b3c0, 0xc0004b2000})\n\t/go/pkg/mod/github.com/gin-gonic/gin@v1.11.0/context.go:825 +0x45\n" +
			"github.com/gin-gonic/gin.(*Context).Next(0xc000521100)\n\t/go/pkg/mod/github.com/gin-gonic/gin@v1.11.0/context.go:192 +0x2b"
	}
	s += "go.opentelemetry.io/otel/sdk/trace.recordStackTrace(...)\n\t/go/pkg/mod/go.opentelemetry.io/otel/sdk@v1.38.0/trace/span.go:570\n" +
		"go.opentelemetry.io/otel/sdk/trace.(*recordingSpan).RecordError(0xc0004a2f00, {0x1b6e0a0, 0xc0003f81e0}, {0xc000519b98, 0x1, 0x1})\n\t/go/pkg/mod/go.opentelemetry.io/otel/sdk@v1.38.0/trace/span.go:552 +0x585\n" +
		"net/http.HandlerFunc.ServeHTTP(0xc0001c8a80?, {0x1d4f2c0?, 0xc0004d2380?}, 0xc0004c8780?)\n\t/usr/local/go/src/net/http/server.go:2306 +0x29\n"
	if st.Framework == "go/chi" {
		return s + "github.com/go-chi/chi/v5.(*Mux).routeHTTP(0xc00012a0e0, {0x1d4f2c0, 0xc0004d2380}, 0xc0004c8780)\n\t/go/pkg/mod/github.com/go-chi/chi/v5@v5.2.2/mux.go:478 +0x2a5"
	}
	return s + "net/http.(*ServeMux).ServeHTTP(0xc00012a0e0, {0x1d4f2c0, 0xc0004d2380}, 0xc0004c8780)\n\t/usr/local/go/src/net/http/server.go:2848 +0x1c4"
}

// applyDefaults fills host metrics a scenario did not set itself.
func applyDefaults(svc *Service) {
	if !svc.defaultHost {
		return
	}
	has := map[string]bool{}
	for _, m := range svc.metrics {
		has[m.name+"@"+m.host] = true
	}
	for _, h := range svc.Hosts {
		host := svc.HostNamed(h)
		if !has["system.cpu.utilization@"+h] {
			host.CPU(Flat(0.18, 0.32))
		}
		if !has["system.memory.utilization@"+h] {
			host.Memory(Flat(0.52, 0.6))
		}
	}
}
