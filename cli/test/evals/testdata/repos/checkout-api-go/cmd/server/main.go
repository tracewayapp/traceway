package main

import (
	"log/slog"
	"net/http"
	"os"

	"example.com/checkout-api/internal/checkout"
	"example.com/checkout-api/internal/payments"
)

func main() {
	h := &checkout.Handler{
		Store:    newStore(os.Getenv("DATABASE_URL")),
		Payments: payments.New(envOr("PAYMENTS_URL", "https://api.payments.example"), os.Getenv("PAYMENTS_API_KEY")),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/checkout", h.Checkout)
	mux.HandleFunc("POST /api/refunds", h.Refund)
	mux.HandleFunc("GET /api/orders/{id}", h.Get)
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
