// Package quote serves the cart page's price summary.
package quote

import (
	"context"
	"encoding/json"
	"net/http"

	"example.com/cart-api/internal/pricing"
	"example.com/cart-api/internal/telemetry"
)

// Pricer prices a cart.
type Pricer interface {
	Quote(ctx context.Context, cartID string) (*pricing.Quote, error)
}

// Handler serves GET /api/cart/quote.
type Handler struct{ pricing Pricer }

// NewHandler returns a Handler.
func NewHandler(p Pricer) *Handler { return &Handler{pricing: p} }

// ServeHTTP answers the price summary of a cart.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	cartID := r.URL.Query().Get("cart")
	if cartID == "" {
		http.Error(w, `{"error":"cart is required"}`, http.StatusBadRequest)
		return
	}
	q, err := h.pricing.Quote(r.Context(), cartID)
	if err != nil {
		telemetry.RecordError(r.Context(), err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "We couldn't calculate your total. Please try again."})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(q)
}
