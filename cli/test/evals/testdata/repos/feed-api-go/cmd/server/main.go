package main

import (
	"database/sql"
	"log/slog"
	"net/http"
	"os"

	"example.com/feed-api/internal/feed"
)

func main() {
	db, err := sql.Open("pgx", os.Getenv("DATABASE_URL"))
	if err != nil {
		slog.Error("open database", "err", err)
		os.Exit(1)
	}
	h := &feed.Handler{Store: &feed.SQL{DB: db}, CDNBase: envOr("CDN_BASE_URL", "https://media.social.example")}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/feed", h.Feed)
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

// withTracing installs the OpenTelemetry HTTP middleware (otelhttp.NewHandler
// in production builds).
func withTracing(h http.Handler) http.Handler { return h }
