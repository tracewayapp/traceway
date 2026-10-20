package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/acme/tickets-api/internal/tickets"
)

func main() {
	ctx := context.Background()
	cfg, err := pgxpool.ParseConfig(os.Getenv("DATABASE_URL"))
	if err != nil {
		slog.Error("database config", "err", err)
		os.Exit(1)
	}
	cfg.ConnConfig.Tracer = otelpgx.NewTracer()
	db, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		slog.Error("database connect", "err", err)
		os.Exit(1)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	mux.HandleFunc("GET /api/events/{id}", func(w http.ResponseWriter, r *http.Request) {
		var ev tickets.Event
		err := db.QueryRow(r.Context(), `SELECT id, title, description, venue_id, starts_at FROM events WHERE id = $1`, r.PathValue("id")).
			Scan(&ev.ID, &ev.Title, &ev.Description, &ev.VenueID, &ev.StartsAt)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		rows, err := db.Query(r.Context(), `SELECT section, row_label, seat_count, price_cents, available FROM seat_blocks WHERE event_id = $1`, ev.ID)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		defer rows.Close()
		var blocks []tickets.SeatBlock
		for rows.Next() {
			var b tickets.SeatBlock
			if err := rows.Scan(&b.Section, &b.Row, &b.SeatCount, &b.PriceCents, &b.Available); err != nil {
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
			blocks = append(blocks, b)
		}
		ev.Sections = tickets.SeatMap(blocks)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(ev)
	})
	// The other routes are registered in internal/tickets (omitted here).

	// otelhttp records spans and the http.server.request.duration histogram.
	srv := &http.Server{Addr: ":8080", Handler: otelhttp.NewHandler(mux, "tickets-api"), ReadHeaderTimeout: 10 * time.Second}
	slog.Info("listening", "addr", srv.Addr, "host", hostname())
	if err := srv.ListenAndServe(); err != nil {
		slog.Error("serve", "err", err)
		os.Exit(1)
	}
}

func hostname() string {
	h, _ := os.Hostname()
	return h
}
