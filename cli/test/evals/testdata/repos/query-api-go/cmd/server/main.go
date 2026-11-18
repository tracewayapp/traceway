package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"

	"example.com/query-api/internal/search"
	"example.com/query-api/internal/telemetry"
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
	h := search.NewHandlers(search.NewStore(db))
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/query", h.Query)
	mux.HandleFunc("GET /api/suggest", h.Suggest)
	mux.HandleFunc("GET /api/documents/{id}", h.Document)
	mux.HandleFunc("POST /api/feedback", h.Feedback)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
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
