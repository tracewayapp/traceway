package main

import (
	"context"
	"errors"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/log/global"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// setupTelemetry installs the OTLP trace and log pipelines. The endpoint and
// resource come from the standard OTEL_* environment variables.
func setupTelemetry(ctx context.Context) (func(context.Context) error, error) {
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

	return func(ctx context.Context) error { return errors.Join(tp.Shutdown(ctx), lp.Shutdown(ctx)) }, nil
}
