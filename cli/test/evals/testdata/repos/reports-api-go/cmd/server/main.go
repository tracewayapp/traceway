package main

import (
	"log/slog"
	"net/http"
	"os"

	"example.com/reports-api/internal/blob"
	"example.com/reports-api/internal/export"
	"example.com/reports-api/internal/reports"
	"example.com/reports-api/internal/telemetry"
)

func main() {
	db, err := telemetry.OpenDB(os.Getenv("DATABASE_URL"))
	if err != nil {
		slog.Error("open database", "err", err)
		os.Exit(1)
	}
	blobs := blob.New(os.Getenv("STORAGE_ENDPOINT"), "reports-exports", os.Getenv("STORAGE_CREDENTIALS_URL"))
	h := reports.NewHandler(export.New(export.NewSQLRows(db), blobs))
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/reports", reports.NewCatalog(db).List)
	mux.HandleFunc("POST /api/reports/{id}/export", h.Export)
	addr := ":" + os.Getenv("PORT")
	if addr == ":" {
		addr = ":8080"
	}
	slog.Info("listening", "addr", addr)
	if err := http.ListenAndServe(addr, telemetry.Handler(mux)); err != nil {
		slog.Error("serve", "err", err)
		os.Exit(1)
	}
}
