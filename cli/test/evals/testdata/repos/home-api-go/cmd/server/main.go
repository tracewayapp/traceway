package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"

	"example.com/home-api/internal/home"
	"example.com/home-api/internal/telemetry"
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
	svc := &home.Service{Cache: newRedisCache(os.Getenv("REDIS_URL")), Builder: &home.SQLBuilder{DB: db, Season: envOr("SEASON", "autumn")}}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/home", svc.Handler)
	mux.HandleFunc("GET /api/home/stories/{id}", (&home.Stories{DB: db}).Handler)
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
