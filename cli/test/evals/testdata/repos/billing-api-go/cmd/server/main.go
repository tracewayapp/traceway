package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	"example.com/billing-api/internal/invoices"
	"example.com/billing-api/internal/mail"
	"example.com/billing-api/internal/telemetry"
)

func main() {
	ctx := context.Background()
	shutdown, err := telemetry.Setup(ctx)
	if err != nil {
		slog.Error("telemetry setup", "err", err)
		os.Exit(1)
	}
	defer shutdown(ctx)
	db, err := telemetry.OpenDB("pgx", os.Getenv("DATABASE_URL"))
	if err != nil {
		slog.Error("open database", "err", err)
		os.Exit(1)
	}
	store := invoices.NewStore(db)
	mailer := mail.New(os.Getenv("MAILER_API_URL"), os.Getenv("MAILER_API_KEY"))
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/invoices/{id}", invoices.NewHandler(store).Get)
	mux.HandleFunc("POST /api/invoices/{id}/send", invoices.NewSender(store, mailer).Send)
	mux.HandleFunc("GET /api/customers/{id}/invoices", invoices.List(store))
	addr := ":" + os.Getenv("PORT")
	if addr == ":" {
		addr = ":8080"
	}
	srv := &http.Server{Addr: addr, Handler: telemetry.Handler(telemetry.AccessLog(invoices.Recover(mux))), ReadHeaderTimeout: 10 * time.Second}
	slog.Info("listening", "addr", addr)
	if err := srv.ListenAndServe(); err != nil {
		slog.Error("serve", "err", err)
		os.Exit(1)
	}
}
