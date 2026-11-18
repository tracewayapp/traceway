package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"

	"example.com/store-locator/internal/inventory"
	"example.com/store-locator/internal/maps"
	"example.com/store-locator/internal/stores"
	"example.com/store-locator/internal/telemetry"
)

func main() {
	ctx := context.Background()
	shutdown, err := telemetry.Setup(ctx)
	if err != nil {
		slog.Error("telemetry setup", "err", err)
		os.Exit(1)
	}
	defer shutdown(ctx)

	db, err := telemetry.OpenDB(os.Getenv("DATABASE_URL"))
	if err != nil {
		slog.Error("open database", "err", err)
		os.Exit(1)
	}
	h := &stores.Handler{
		Maps:      maps.New(envOr("MAPS_URL", "https://maps.geo.example"), os.Getenv("MAPS_API_KEY")),
		Inventory: inventory.New(envOr("INVENTORY_URL", "http://inventory.internal:8080"), telemetry.Transport(http.DefaultTransport)),
		Stores:    stores.NewDirectory(db),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/stores/nearby", h.Nearby)
	mux.HandleFunc("GET /api/stores/{id}/availability", h.Availability)
	mux.HandleFunc("GET /api/stores/{id}/hours", h.Hours)
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
