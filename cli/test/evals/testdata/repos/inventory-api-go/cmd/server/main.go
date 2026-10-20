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

	"example.com/inventory-api/internal/db"
	"example.com/inventory-api/internal/reserve"
	"example.com/inventory-api/internal/stock"
)

func main() {
	ctx := context.Background()
	shutdown, err := setupTelemetry(ctx)
	if err != nil {
		slog.Error("telemetry setup", "err", err)
		os.Exit(1)
	}
	defer shutdown(ctx)
	slog.SetDefault(otelslog.NewLogger("go.opentelemetry.io/contrib/bridges/otelslog"))

	cfg, err := pgxpool.ParseConfig(os.Getenv("DATABASE_URL"))
	if err != nil {
		slog.Error("database config", "err", err)
		os.Exit(1)
	}
	cfg.ConnConfig.Tracer = otelpgx.NewTracer()
	cfg.MaxConns = db.Config.MaxConns
	cfg.MinConns = db.Config.MinConns
	cfg.MaxConnIdleTime = time.Duration(db.Config.MaxConnIdleTimeMs) * time.Millisecond
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		slog.Error("open database", "err", err)
		os.Exit(1)
	}

	reservations := &reserve.Handler{Pool: db.PgxPool{Pool: pool}}
	levels := &stock.Handler{DB: pool}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/reservations", reservations.Create)
	mux.HandleFunc("POST /api/reservations/{id}/release", reservations.Release)
	mux.HandleFunc("GET /api/inventory/{sku}", levels.Levels)
	mux.HandleFunc("GET /api/warehouses", levels.Warehouses)

	addr := ":" + envOr("PORT", "8080")
	slog.Info("listening", "addr", addr, "max_conns", db.Config.MaxConns)
	srv := &http.Server{Addr: addr, Handler: tracing("inventory-api", accessLog(mux)), ReadHeaderTimeout: 10 * time.Second}
	if err := srv.ListenAndServe(); err != nil {
		slog.Error("serve", "err", err)
		os.Exit(1)
	}
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
