package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	"example.com/mobile-api/internal/config"
	"example.com/mobile-api/internal/feed"
	"example.com/mobile-api/internal/telemetry"
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
	srv := &http.Server{Addr: addr, Handler: telemetry.Handler(telemetry.AccessLog(mux)), ReadHeaderTimeout: 5 * time.Second}
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
