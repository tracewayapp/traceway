package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/acme/listings-api/internal/listings"
	"github.com/acme/listings-api/internal/logging"
)

func main() {
	ctx := context.Background()
	log := logging.New(os.Getenv("LOG_LEVEL"))
	slog.SetDefault(log)

	shutdown, err := setupTelemetry(ctx)
	if err != nil {
		log.Error("telemetry setup failed", "err", err)
		os.Exit(1)
	}
	defer shutdown(ctx)

	cfg, err := pgxpool.ParseConfig(os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Error("database config", "err", err)
		os.Exit(1)
	}
	cfg.ConnConfig.Tracer = otelpgx.NewTracer()
	cfg.MaxConns = int32(envInt("DATABASE_MAX_CONNS", 20))
	db, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		log.Error("database connect", "err", err)
		os.Exit(1)
	}
	cache := redis.NewClient(&redis.Options{Addr: os.Getenv("REDIS_ADDR")})
	_ = redisotel.InstrumentTracing(cache)

	ttl, _ := time.ParseDuration(os.Getenv("SEARCH_CACHE_TTL"))
	repo := listings.NewRepo(db, cache, log, envInt("SEARCH_PAGE_SIZE", 44), ttl)
	mux := http.NewServeMux()
	listings.NewHandler(repo, log).Routes(mux)

	addr := os.Getenv("LISTEN_ADDR")
	log.Info("starting listings-api", "version", version, "log.level", os.Getenv("LOG_LEVEL"), "addr", addr)
	srv := &http.Server{Addr: addr, Handler: otelhttp.NewHandler(mux, "listings-api"), ReadHeaderTimeout: 10 * time.Second}
	if err := srv.ListenAndServe(); err != nil {
		log.Error("serve", "err", err)
		os.Exit(1)
	}
}

var version = "dev"

func envInt(name string, def int) int {
	if v, err := strconv.Atoi(os.Getenv(name)); err == nil {
		return v
	}
	return def
}
