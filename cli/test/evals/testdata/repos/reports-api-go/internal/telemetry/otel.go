//go:build otel

package telemetry

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

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
	"go.opentelemetry.io/otel/trace"
)

func init() {
	RecordError = func(ctx context.Context, err error) {
		span := trace.SpanFromContext(ctx)
		span.RecordError(err, trace.WithStackTrace(true))
		span.SetStatus(codes.Error, err.Error())
	}
	SetAttributes = func(ctx context.Context, kv ...any) {
		span := trace.SpanFromContext(ctx)
		for i := 0; i+1 < len(kv); i += 2 {
			span.SetAttributes(attribute.String(fmt.Sprint(kv[i]), fmt.Sprint(kv[i+1])))
		}
	}
	Handler = func(h http.Handler) http.Handler {
		return otelhttp.NewHandler(routeAttr(h), "reports-api", otelhttp.WithSpanNameFormatter(spanName))
	}
	HTTPClient = func() *http.Client {
		return &http.Client{Transport: otelhttp.NewTransport(http.DefaultTransport, otelhttp.WithSpanNameFormatter(clientSpanName))}
	}
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
