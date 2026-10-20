package main

import (
	"log/slog"
	"net/http"
	"os"

	"example.com/query-api/internal/search"
)

func main() {
	h := search.NewHandlers(search.NewStore(nil))
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/query", h.Query)
	mux.HandleFunc("GET /api/suggest", h.Suggest)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	addr := ":" + envOr("PORT", "8080")
	slog.Info("listening", "addr", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
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
