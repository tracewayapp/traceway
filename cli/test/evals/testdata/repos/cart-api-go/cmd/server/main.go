package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"

	"example.com/cart-api/internal/config"
	"example.com/cart-api/internal/pricing"
	"example.com/cart-api/internal/quote"
	"example.com/cart-api/internal/telemetry"
)

func main() {
	ctx := context.Background()
	shutdown, err := telemetry.Setup(ctx)
	if err != nil {
		slog.Error("telemetry setup", "err", err)
		os.Exit(1)
	}
	defer shutdown(ctx)
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		slog.Error("config", "err", err)
		os.Exit(1)
	}
	mux := http.NewServeMux()
	mux.Handle("GET /api/cart/quote", quote.NewHandler(pricing.New(cfg)))
	slog.Info("listening", "port", cfg.Port, "pricing_timeout", cfg.PricingTimeout)
	if err := http.ListenAndServe(":"+cfg.Port, telemetry.Handler(telemetry.AccessLog(mux))); err != nil {
		slog.Error("serve", "err", err)
		os.Exit(1)
	}
}
