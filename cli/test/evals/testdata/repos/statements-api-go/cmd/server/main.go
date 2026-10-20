package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	"example.com/statements-api/internal/objects"
	"example.com/statements-api/internal/pdf"
	"example.com/statements-api/internal/statements"
	"example.com/statements-api/internal/telemetry"
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
	font, err := os.ReadFile(os.Getenv("STATEMENT_FONT"))
	if err != nil {
		slog.Error("read font", "err", err)
		os.Exit(1)
	}
	bucket := &objects.Bucket{
		Endpoint: os.Getenv("STATEMENTS_BUCKET_URL"),
		Signer:   objects.SidecarSigner(os.Getenv("SIGNER_URL")),
		Client:   telemetry.HTTPClient(30 * time.Second),
	}
	h := statements.New(statements.NewSQLStore(db), pdf.NewRenderer(font), bucket)

	mux := http.NewServeMux()
	h.Routes(mux)
	srv := &http.Server{Addr: ":8080", Handler: telemetry.Handler(telemetry.AccessLog(mux)), ReadHeaderTimeout: 10 * time.Second}
	slog.Info("listening", "addr", srv.Addr)
	if err := srv.ListenAndServe(); err != nil {
		slog.Error("serve", "err", err)
		os.Exit(1)
	}
}
