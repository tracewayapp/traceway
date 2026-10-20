package main

import (
	"log/slog"
	"net/http"
	"os"

	"example.com/store-locator/internal/inventory"
	"example.com/store-locator/internal/maps"
	"example.com/store-locator/internal/stores"
)

func main() {
	h := &stores.Handler{
		Maps:      maps.New(envOr("MAPS_URL", "https://maps.geo.example"), os.Getenv("MAPS_API_KEY")),
		Inventory: inventory.New(envOr("INVENTORY_URL", "http://inventory.internal:8080"), maps.Instrument(http.DefaultTransport)),
		Stores:    newFinder(os.Getenv("DATABASE_URL")),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/stores/nearby", h.Nearby)
	mux.HandleFunc("GET /api/stores/{id}/availability", h.Availability)
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
