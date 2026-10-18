// Package handlers serves the orders API.
package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"example.com/orders-api/internal/store"
)

// Handlers holds the HTTP handlers.
type Handlers struct{ store store.Store }

// New returns the handlers.
func New(s store.Store) *Handlers { return &Handlers{store: s} }

// ListOrders serves GET /api/orders: the signed-in customer's orders.
func (h *Handlers) ListOrders(w http.ResponseWriter, r *http.Request) {
	userID, err := strconv.ParseInt(r.Header.Get("X-User-Id"), 10, 64)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	orders, err := h.store.OrdersByUser(r.Context(), userID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, orders)
}

// GetOrder serves GET /api/orders/{id}.
func (h *Handlers) GetOrder(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	o, err := h.store.OrderByID(r.Context(), id)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if o == nil {
		http.NotFound(w, r)
		return
	}
	if o.Items, err = h.store.ItemsByOrder(r.Context(), o.ID); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, o)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
