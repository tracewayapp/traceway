package main

import (
	"log/slog"
	"net/http"
	"os"

	"example.com/inventory-api/internal/db"
	"example.com/inventory-api/internal/reserve"
)

func main() {
	pool, err := openPool(os.Getenv("DATABASE_URL"), db.Config.MaxConns)
	if err != nil {
		slog.Error("open database", "err", err)
		os.Exit(1)
	}
	h := &reserve.Handler{Pool: pool}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/reservations", h.Create)
	mux.HandleFunc("POST /api/reservations/{id}/release", h.Release)
	addr := ":" + envOr("PORT", "8080")
	slog.Info("listening", "addr", addr, "max_conns", db.Config.MaxConns)
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
