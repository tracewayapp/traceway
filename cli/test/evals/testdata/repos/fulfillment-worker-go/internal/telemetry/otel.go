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

var tracer = otel.Tracer("example.com/fulfillment/internal/worker")

func init() {
	StartConsumer = func(ctx context.Context, name string, attrs ...Attr) (context.Context, func()) {
		ctx, span := tracer.Start(ctx, name, trace.WithSpanKind(trace.SpanKindConsumer))
		for _, a := range attrs {
			switch v := a.Value.(type) {
			case int64:
				span.SetAttributes(attribute.Int64(a.Key, v))
			default:
				span.SetAttributes(attribute.String(a.Key, fmt.Sprint(v)))
			}
		}
		return ctx, func() { span.End() }
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
	OpenDB = func(driver, dsn string) (*sql.DB, error) {
		return otelsql.Open(driver, dsn,
			otelsql.WithAttributes(semconv.DBSystemNamePostgreSQL),
			otelsql.WithSpanNameFormatter(func(_ context.Context, _ otelsql.Method, query string) string { return querySpanName(query) }),
			otelsql.WithSpanOptions(otelsql.SpanOptions{OmitConnResetSession: true, OmitConnPrepare: true, OmitRows: true}))
	}
	Transport = otelhttp.NewTransport(http.DefaultTransport)
	Setup = setup
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

// querySpanName is "{operation} {table}", e.g. "SELECT packages".
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
