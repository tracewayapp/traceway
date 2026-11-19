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

	"github.com/XSAM/otelsql"
	_ "github.com/jackc/pgx/v5/stdlib"
	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/log/global"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
	"go.opentelemetry.io/otel/trace"
)

func init() {
	Setup = setup
	Handler = func(h http.Handler) http.Handler {
		return otelhttp.NewHandler(routeAttr(h), "billing-api", otelhttp.WithSpanNameFormatter(spanName))
	}
	OpenDB = func(driver, dsn string) (*sql.DB, error) {
		return otelsql.Open(driver, dsn,
			otelsql.WithAttributes(semconv.DBSystemNamePostgreSQL),
			otelsql.WithSpanNameFormatter(dbSpanName),
			otelsql.WithSpanOptions(otelsql.SpanOptions{OmitConnResetSession: true, OmitConnPrepare: true, OmitRows: true}))
	}
	Transport = otelhttp.NewTransport(http.DefaultTransport)
	SetAttributes = func(ctx context.Context, attrs ...Attr) {
		span := trace.SpanFromContext(ctx)
		for _, a := range attrs {
			span.SetAttributes(attribute.String(a.Key, fmt.Sprint(a.Value)))
		}
	}
	RecordPanic = func(ctx context.Context, v any, stack []byte) {
		span := trace.SpanFromContext(ctx)
		span.AddEvent("exception", trace.WithAttributes(
			attribute.String("exception.type", fmt.Sprintf("%T", v)),
			attribute.String("exception.message", fmt.Sprint(v)),
			attribute.String("exception.stacktrace", string(stack)),
			attribute.Bool("exception.escaped", true),
		))
		span.SetStatus(codes.Error, fmt.Sprint(v))
	}
	RecordError = func(ctx context.Context, err error) {
		span := trace.SpanFromContext(ctx)
		span.RecordError(err, trace.WithStackTrace(true))
		span.SetStatus(codes.Error, err.Error())
	}
}

// spanName names a server span after the route the ServeMux matched, as in
// "GET /api/invoices/{id}".
func spanName(_ string, r *http.Request) string {
	if r.Pattern != "" {
		return r.Pattern
	}
	return r.Method
}

// routeAttr records the matched route as http.route on the server span.
func routeAttr(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
		if _, route, ok := strings.Cut(r.Pattern, " "); ok {
			trace.SpanFromContext(r.Context()).SetAttributes(semconv.HTTPRoute(route))
		}
	})
}

// setup installs the OTLP trace and log pipelines; endpoint and resource come
// from the OTEL_* environment variables.
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

// querySpanName is "{operation} {table}", e.g. "SELECT invoices".
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

// dbSpanName names a query span after its statement and any other call
// (connect, begin) after the otelsql method, as in "sql.connector.connect".
func dbSpanName(_ context.Context, m otelsql.Method, query string) string {
	if query == "" {
		return string(m)
	}
	return querySpanName(query)
}
