package main

import (
	"database/sql"
	"log/slog"
	"net/http"
	"os"

	"example.com/invoicing-api/internal/exports"
	"example.com/invoicing-api/internal/invoices"
)

func main() {
	db, err := sql.Open("pgx", os.Getenv("DATABASE_URL"))
	if err != nil {
		slog.Error("open database", "err", err)
		os.Exit(1)
	}
	inv := invoices.New(db)
	exp := exports.New(exports.NewStore(db))
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/invoices", inv.List)
	mux.HandleFunc("GET /api/invoices/{id}", inv.Get)
	mux.HandleFunc("POST /api/invoices", inv.Create)
	mux.HandleFunc("GET /api/exports/invoices", exp.Invoices)
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
