package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"

	"example.com/checkout-api/internal/checkout"
	"example.com/checkout-api/internal/payments"
	"example.com/checkout-api/internal/telemetry"
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
	h := &checkout.Handler{
		Store:    checkout.NewStore(db),
		Payments: payments.New(envOr("PAYMENTS_URL", "https://api.payments.example"), os.Getenv("PAYMENTS_API_KEY"), telemetry.Transport(http.DefaultTransport)),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/checkout", h.Checkout)
	mux.HandleFunc("POST /api/refunds", h.Refund)
	mux.HandleFunc("GET /api/orders/{id}", h.Get)
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
