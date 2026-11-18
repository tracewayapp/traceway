package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"

	"example.com/ledger-api/internal/accounts"
	"example.com/ledger-api/internal/reports"
	"example.com/ledger-api/internal/telemetry"
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
	acc := accounts.New(db)
	exp := reports.NewExporter(reports.NewStore(db))
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/accounts/{id}/balance", acc.Balance)
	mux.HandleFunc("GET /api/accounts/{id}/entries", acc.Entries)
	mux.HandleFunc("POST /api/journal-entries", acc.Post)
	mux.HandleFunc("GET /api/reports/export", exp.Export)
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
