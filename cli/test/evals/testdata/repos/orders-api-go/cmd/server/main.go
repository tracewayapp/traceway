package main

import (
	"database/sql"
	"log/slog"
	"net/http"
	"os"

	"example.com/orders-api/internal/handlers"
	"example.com/orders-api/internal/store"
)

func main() {
	db, err := sql.Open("pgx", os.Getenv("DATABASE_URL"))
	if err != nil {
		slog.Error("open database", "err", err)
		os.Exit(1)
	}
	h := handlers.New(store.New(db))
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/orders", h.ListOrders)
	mux.HandleFunc("GET /api/orders/{id}", h.GetOrder)
	addr := ":" + envOr("PORT", "8080")
	slog.Info("listening", "addr", addr)
	if err := http.ListenAndServe(addr, withTracing(mux)); err != nil {
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

// withTracing is where the OpenTelemetry HTTP middleware is installed
// (otelhttp.NewHandler in production builds).
func withTracing(h http.Handler) http.Handler { return h }
