package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	"example.com/catalog-api/internal/catalog"
)

func main() {
	ctx := context.Background()
	// pgxpool.New(ctx, os.Getenv("DATABASE_URL")) in production builds:
	// pool_max_conns=10, pool_min_conns=0, connections are opened on demand.
	store := catalog.NewStore(nil)
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
	if err := http.ListenAndServe(addr, mux); err != nil {
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
