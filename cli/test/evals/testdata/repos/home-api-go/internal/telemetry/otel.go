//go:build otel

package telemetry

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"

	"github.com/XSAM/otelsql"
	_ "github.com/jackc/pgx/v5/stdlib"
	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/metric"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
	"go.opentelemetry.io/otel/trace"
)

const service = "home-api"

func init() {
	Setup = setup
	Handler = func(h http.Handler) http.Handler {
		return otelhttp.NewHandler(withRoute(h), service, otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string {
			if r.Pattern != "" {
				return r.Pattern
			}
			return r.Method
		}))
	}
	OpenDB = func(dsn string) (*sql.DB, error) {
		return otelsql.Open("pgx", dsn,
			otelsql.WithAttributes(semconv.DBSystemNamePostgreSQL),
			otelsql.WithSpanNameFormatter(func(_ context.Context, _ otelsql.Method, query string) string { return querySpanName(query) }),
			otelsql.WithSpanOptions(otelsql.SpanOptions{OmitConnResetSession: true, OmitConnPrepare: true, OmitRows: true, OmitConnectorConnect: true}))
	}
	CacheLookup = cacheHits.record
	tracer := otel.Tracer(service)
	Start = func(ctx context.Context, name string, kv ...any) (context.Context, func()) {
		ctx, span := tracer.Start(ctx, name, trace.WithAttributes(attrs(kv)...))
		return ctx, func() { span.End() }
	}
}

// cacheHits counts lookups per cache between two collections of the gauge.
var cacheHits = &hitCounter{counts: map[string]*[2]int{}}

type hitCounter struct {
	mu     sync.Mutex
	counts map[string]*[2]int
}

func (h *hitCounter) record(cache string, hit bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	c := h.counts[cache]
	if c == nil {
		c = &[2]int{}
		h.counts[cache] = c
	}
	if hit {
		c[0]++
	} else {
		c[1]++
	}
}

func (h *hitCounter) observe(_ context.Context, o metric.Float64Observer) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	for cache, c := range h.counts {
		if n := c[0] + c[1]; n > 0 {
			o.Observe(float64(c[0])/float64(n), metric.WithAttributes(attribute.String("cache", cache)))
		}
		*c = [2]int{}
	}
	return nil
}

// setup installs the OTLP trace, metric and log pipelines; endpoint and resource come from
// the OTEL_* environment variables.
func setup(ctx context.Context) (func(context.Context) error, error) {
	te, err := otlptracehttp.New(ctx)
	if err != nil {
		return nil, err
	}
	tp := sdktrace.NewTracerProvider(sdktrace.WithBatcher(te))
	otel.SetTracerProvider(tp)
	me, err := otlpmetrichttp.New(ctx)
	if err != nil {
		return nil, err
	}
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(sdkmetric.NewPeriodicReader(me)))
	otel.SetMeterProvider(mp)
	if _, err := mp.Meter(service).Float64ObservableGauge("cache.hit_ratio",
		metric.WithUnit("1"), metric.WithDescription("Share of cache lookups that hit"),
		metric.WithFloat64Callback(cacheHits.observe)); err != nil {
		return nil, err
	}
	le, err := otlploghttp.New(ctx)
	if err != nil {
		return nil, err
	}
	lp := sdklog.NewLoggerProvider(sdklog.WithProcessor(sdklog.NewBatchProcessor(le)))
	global.SetLoggerProvider(lp)
	slog.SetDefault(otelslog.NewLogger("go.opentelemetry.io/contrib/bridges/otelslog"))
	return func(ctx context.Context) error {
		return errors.Join(tp.Shutdown(ctx), mp.Shutdown(ctx), lp.Shutdown(ctx))
	}, nil
}

// withRoute names the server span after the matched ServeMux pattern.
func withRoute(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.ServeHTTP(w, r)
		if _, route, ok := strings.Cut(r.Pattern, " "); ok {
			trace.SpanFromContext(r.Context()).SetAttributes(semconv.HTTPRoute(route))
		}
	})
}

// querySpanName is "{operation} {table}", e.g. "SELECT orders".
func querySpanName(query string) string {
	f := strings.Fields(query)
	if len(f) == 0 {
		return "QUERY"
	}
	op := strings.ToUpper(f[0])
	for i := 0; i+1 < len(f); i++ {
		if u := strings.ToUpper(f[i]); u == "FROM" || u == "INTO" || (u == "UPDATE" && i == 0) {
			return op + " " + strings.ToLower(strings.Trim(f[i+1], "(),;"))
		}
	}
	return op
}

func attrs(kv []any) []attribute.KeyValue {
	out := make([]attribute.KeyValue, 0, len(kv)/2)
	for i := 0; i+1 < len(kv); i += 2 {
		k := fmt.Sprint(kv[i])
		switch v := kv[i+1].(type) {
		case int:
			out = append(out, attribute.Int(k, v))
		default:
			out = append(out, attribute.String(k, fmt.Sprint(v)))
		}
	}
	return out
}
