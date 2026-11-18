package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	"example.com/catalog-api/internal/catalog"
	"example.com/catalog-api/internal/telemetry"
)

func main() {
	ctx := context.Background()
	shutdown, err := telemetry.Setup(ctx)
	if err != nil {
		slog.Error("telemetry setup", "err", err)
		os.Exit(1)
	}
	defer shutdown(ctx)

	// DATABASE_URL sets pool_max_conns=10 and pool_min_conns=0: connections
	// are opened on demand.
	db, err := telemetry.OpenDB(os.Getenv("DATABASE_URL"))
	if err != nil {
		slog.Error("open database", "err", err)
		os.Exit(1)
	}
	store := catalog.NewStore(db, envOr("PRICE_LIST", "eu-retail"))
	cache := catalog.NewCache(store, 10*time.Minute)

	// Fill the cache in the background so the instance can take traffic
	// straight away; categories not loaded yet are fetched on demand. One
	// category every 25s, so the three instances a release restarts together
	// do not all load the whole catalog at the same moment.
	go cache.Preload(ctx, catalog.Categories, 25*time.Second)
	go cache.RefreshLoop(ctx, catalog.Categories)

	h := catalog.NewHandlers(store, cache)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/categories/{slug}/products", h.Listing)
	mux.HandleFunc("GET /api/products/{id}", h.Product)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	addr := ":" + envOr("PORT", "8080")
	slog.Info("listening", "addr", addr)
	if err := http.ListenAndServe(addr, telemetry.Handler(telemetry.AccessLog(mux))); err != nil {
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
