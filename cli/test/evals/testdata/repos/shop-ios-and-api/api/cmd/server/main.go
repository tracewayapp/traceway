package main

import (
	"database/sql"
	"log/slog"
	"net/http"
	"os"
	"time"

	"example.com/mobile-api/internal/config"
	"example.com/mobile-api/internal/feed"
)

func main() {
	// otelsql.Open("pgx", ...) in production builds.
	db, err := sql.Open("pgx", os.Getenv("DATABASE_URL"))
	if err != nil {
		slog.Error("open database", "err", err)
		os.Exit(1)
	}
	// Sized for the steady state: about 40 requests a second at a few
	// milliseconds each, plus the feed.
	db.SetMaxOpenConns(8)
	db.SetMaxIdleConns(8)
	db.SetConnMaxLifetime(30 * time.Minute)

	mux := http.NewServeMux()
	mux.Handle("GET /api/config", config.Handler(config.NewStore(db)))
	mux.Handle("POST /api/devices", config.RegisterDevice(db))
	mux.Handle("GET /api/feed", feed.Handler(db))
	mux.Handle("GET /api/products/{id}", feed.Product(db))
	addr := ":" + envOr("PORT", "8080")
	slog.Info("listening", "addr", addr)
	srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	if err := srv.ListenAndServe(); err != nil {
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
