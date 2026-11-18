package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"

	"example.com/shipping-api/internal/rates"
	"example.com/shipping-api/internal/shipments"
	"example.com/shipping-api/internal/telemetry"
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
	h := shipments.NewHandler(shipments.NewService(rates.New(db), shipments.NewSQLRepo(db)))
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/shipments", h.Create)
	mux.HandleFunc("GET /api/shipments/{id}", h.Get)
	addr := ":" + os.Getenv("PORT")
	if addr == ":" {
		addr = ":8080"
	}
	slog.Info("listening", "addr", addr)
	if err := http.ListenAndServe(addr, telemetry.Handler(telemetry.AccessLog(mux))); err != nil {
		slog.Error("serve", "err", err)
		os.Exit(1)
	}
}
