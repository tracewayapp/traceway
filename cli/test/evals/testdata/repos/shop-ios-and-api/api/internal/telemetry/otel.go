//go:build otel

package telemetry

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/XSAM/otelsql"
	_ "github.com/jackc/pgx/v5/stdlib"
	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/log/global"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
	"go.opentelemetry.io/otel/trace"
)

const service = "mobile-api"

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
	tracer := otel.Tracer(service)
	Start = func(ctx context.Context, name string) (context.Context, func()) {
		ctx, span := tracer.Start(ctx, name)
		return ctx, func() { span.End() }
	}
}

// setup installs the OTLP trace and log pipelines; endpoint and resource come from
// the OTEL_* environment variables.
func setup(ctx context.Context) (func(context.Context) error, error) {
	te, err := otlptracehttp.New(ctx)
	if err != nil {
		return nil, err
	}
	tp := sdktrace.NewTracerProvider(sdktrace.WithBatcher(te))
	otel.SetTracerProvider(tp)
	le, err := otlploghttp.New(ctx)
	if err != nil {
		return nil, err
	}
	lp := sdklog.NewLoggerProvider(sdklog.WithProcessor(sdklog.NewBatchProcessor(le)))
	global.SetLoggerProvider(lp)
	slog.SetDefault(otelslog.NewLogger("go.opentelemetry.io/contrib/bridges/otelslog"))
	return func(ctx context.Context) error { return errors.Join(tp.Shutdown(ctx), lp.Shutdown(ctx)) }, nil
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
