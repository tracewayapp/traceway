package main

import (
	"database/sql"
	"log/slog"
	"net/http"
	"os"
	"time"

	"example.com/statements-api/internal/objects"
	"example.com/statements-api/internal/pdf"
	"example.com/statements-api/internal/statements"
)

func main() {
	db, err := sql.Open("pgx", os.Getenv("DATABASE_URL"))
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
		Client:   &http.Client{Timeout: 30 * time.Second},
	}
	h := statements.New(statements.NewSQLStore(db), pdf.NewRenderer(font), bucket)

	mux := http.NewServeMux()
	h.Routes(mux)
	srv := &http.Server{Addr: ":8080", Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	slog.Info("listening", "addr", srv.Addr)
	if err := srv.ListenAndServe(); err != nil {
		slog.Error("serve", "err", err)
		os.Exit(1)
	}
}
