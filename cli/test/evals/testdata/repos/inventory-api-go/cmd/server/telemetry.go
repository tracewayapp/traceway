package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

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

// tracing wraps the mux in otelhttp, naming each server span after the
// pattern the mux matched ("POST /api/reservations") and recording it as
// http.route, which otelhttp cannot see before the mux has run.
func tracing(service string, mux http.Handler) http.Handler {
	routed := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mux.ServeHTTP(w, r)
		if _, route, ok := strings.Cut(r.Pattern, " "); ok {
			trace.SpanFromContext(r.Context()).SetAttributes(semconv.HTTPRoute(route))
		}
	})
	return otelhttp.NewHandler(routed, service, otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string {
		if r.Pattern != "" {
			return r.Pattern
		}
		return r.Method
	}))
}

// accessLog writes one line per request, inside the request's trace.
func accessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		ms := time.Since(start).Milliseconds()
		_, route, _ := strings.Cut(r.Pattern, " ")
		slog.InfoContext(r.Context(), fmt.Sprintf("%s %s %d %dms", r.Method, r.URL.Path, rec.status, ms),
			"http.request.method", r.Method, "url.path", r.URL.Path, "http.route", route,
			"http.response.status_code", rec.status, "duration_ms", ms)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}
