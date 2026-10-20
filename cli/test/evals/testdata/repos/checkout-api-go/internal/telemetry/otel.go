//go:build otel

package telemetry

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/log/global"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
	"go.opentelemetry.io/otel/trace"
)

const service = "checkout-api"

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
	Transport = func(rt http.RoundTripper) http.RoundTripper {
		return otelhttp.NewTransport(rt, otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string { return r.Method }))
	}
	RecordError = func(ctx context.Context, err error) { trace.SpanFromContext(ctx).RecordError(err) }
	OpenDB = func(dsn string) (*sql.DB, error) {
		cfg, err := pgxpool.ParseConfig(dsn)
		if err != nil {
			return nil, err
		}
		cfg.ConnConfig.Tracer = otelpgx.NewTracer(otelpgx.WithTrimSQLInSpanName(), otelpgx.WithSpanNameFunc(querySpanName), otelpgx.WithDisableQuerySpanNamePrefix())
		pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
		if err != nil {
			return nil, err
		}
		return stdlib.OpenDBFromPool(pool), nil
	}
}

// setup installs the OTLP trace, metric and log pipelines and starts the Go
// runtime metrics (go.goroutine.count, go.memory.*); endpoint and resource
// come from the OTEL_* environment variables plus the host name.
func setup(ctx context.Context) (func(context.Context) error, error) {
	res, err := resource.New(ctx, resource.WithFromEnv(), resource.WithHost(), resource.WithTelemetrySDK())
	if err != nil {
		return nil, err
	}
	te, err := otlptracehttp.New(ctx)
	if err != nil {
		return nil, err
	}
	tp := sdktrace.NewTracerProvider(sdktrace.WithBatcher(te), sdktrace.WithResource(res))
	otel.SetTracerProvider(tp)
	me, err := otlpmetrichttp.New(ctx)
	if err != nil {
		return nil, err
	}
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(sdkmetric.NewPeriodicReader(me)), sdkmetric.WithResource(res))
	otel.SetMeterProvider(mp)
	if err := runtime.Start(); err != nil {
		return nil, err
	}
	le, err := otlploghttp.New(ctx)
	if err != nil {
		return nil, err
	}
	lp := sdklog.NewLoggerProvider(sdklog.WithProcessor(sdklog.NewBatchProcessor(le)), sdklog.WithResource(res))
	global.SetLoggerProvider(lp)
	slog.SetDefault(otelslog.NewLogger("go.opentelemetry.io/contrib/bridges/otelslog"))
	return func(ctx context.Context) error {
		return errors.Join(tp.Shutdown(ctx), mp.Shutdown(ctx), lp.Shutdown(ctx))
	}, nil
}

// withRoute records the matched ServeMux pattern as http.route.
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
