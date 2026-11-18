package invoices

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strconv"

	"example.com/billing-api/internal/telemetry"
)

// Handler serves invoices.
type Handler struct{ store Store }

// NewHandler returns a Handler.
func NewHandler(s Store) *Handler { return &Handler{store: s} }

// Get serves GET /api/invoices/{id}.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	inv, err := h.store.Invoice(r.Context(), id)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if inv == nil {
		http.NotFound(w, r)
		return
	}
	telemetry.SetAttributes(r.Context(), telemetry.Attr{Key: "enduser.id", Value: inv.Customer.ID})
	view := render(inv)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(view)
}

// Recover turns a panic into a 500 and records it, with its stack, on the
// request span.
func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				telemetry.RecordPanic(r.Context(), v, debug.Stack())
				slog.ErrorContext(r.Context(), "panic serving request", "path", r.URL.Path, "panic", fmt.Sprint(v))
				http.Error(w, "internal error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
