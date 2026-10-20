package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/contrib/instrumentation/runtime"

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
	slog.SetDefault(otelslog.NewLogger("geo-api"))
	// go.memory.*, go.goroutine.count, go.processor.limit, ...
	if err := runtime.Start(runtime.WithMinimumReadMemStatsInterval(15 * time.Second)); err != nil {
		slog.Error("runtime metrics", "err", err)
	}

	cfg, err := pgxpool.ParseConfig(os.Getenv("DATABASE_URL"))
	if err != nil {
		slog.Error("database config", "err", err)
		os.Exit(1)
	}
	cfg.ConnConfig.Tracer = otelpgx.NewTracer()
	db, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		slog.Error("database connect", "err", err)
		os.Exit(1)
	}

	mux := http.NewServeMux()
	places.NewHandler(db).Routes(mux)
	slog.Info("starting geo-api", "version", version, "addr", ":8080", "gomemlimit", os.Getenv("GOMEMLIMIT"))
	srv := &http.Server{Addr: ":8080", Handler: otelhttp.NewHandler(mux, "geo-api"), ReadHeaderTimeout: 10 * time.Second}
	if err := srv.ListenAndServe(); err != nil {
		slog.Error("serve", "err", err)
		os.Exit(1)
	}
}
