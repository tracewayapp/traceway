package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/contrib/instrumentation/runtime"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
	"go.opentelemetry.io/otel/trace"

	"github.com/acme/geo-api/internal/places"
)

var version = "dev"

func main() {
	ctx := context.Background()
	shutdown, err := setupTelemetry(ctx)
	if err != nil {
		slog.Error("telemetry setup failed", "err", err)
		os.Exit(1)
	}
	defer shutdown(ctx)
	slog.SetDefault(otelslog.NewLogger("go.opentelemetry.io/contrib/bridges/otelslog"))
	// go.memory.*, go.goroutine.count, go.processor.limit, ...
	if err := runtime.Start(runtime.WithMinimumReadMemStatsInterval(15 * time.Second)); err != nil {
		slog.Error("runtime metrics", "err", err)
	}

	cfg, err := pgxpool.ParseConfig(os.Getenv("DATABASE_URL"))
	if err != nil {
		slog.Error("database config", "err", err)
		os.Exit(1)
	}
	cfg.ConnConfig.Tracer = otelpgx.NewTracer(otelpgx.WithTrimSQLInSpanName(), otelpgx.WithSpanNameFunc(querySpanName), otelpgx.WithDisableQuerySpanNamePrefix())
	db, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		slog.Error("database connect", "err", err)
		os.Exit(1)
	}

	mux := http.NewServeMux()
	places.NewHandler(db).Routes(mux)
	slog.Info("starting geo-api", "version", version, "addr", ":8080", "gomemlimit", os.Getenv("GOMEMLIMIT"))
	srv := &http.Server{Addr: ":8080", Handler: tracing("geo-api", accessLog(mux)), ReadHeaderTimeout: 10 * time.Second}
	if err := srv.ListenAndServe(); err != nil {
		slog.Error("serve", "err", err)
		os.Exit(1)
	}
}

// tracing wraps the mux in otelhttp, naming each server span after the
// pattern the mux matched ("GET /api/places/{id}") and recording it as
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
