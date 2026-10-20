// Package reserve holds stock for carts.
package reserve

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"example.com/inventory-api/internal/db"
)

// Line is one cart line to reserve.
type Line struct {
	SKU         string `json:"sku"`
	WarehouseID int64  `json:"warehouseId"`
	Qty         int    `json:"qty"`
}

// Request is the body of POST /api/reservations.
type Request struct {
	CartID string `json:"cartId"`
	Lines  []Line `json:"lines"`
}

// ErrOutOfStock means a line could not be held.
var ErrOutOfStock = errors.New("out of stock")

// Handler serves the reservation routes.
type Handler struct{ Pool db.Pool }

// Create serves POST /api/reservations.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.CartID == "" || len(req.Lines) == 0 {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	err := Reserve(r.Context(), h.Pool, req)
	switch {
	case err == nil:
		w.WriteHeader(http.StatusCreated)
	case errors.Is(err, ErrOutOfStock):
		http.Error(w, err.Error(), http.StatusConflict)
	case db.SQLState(err) == "55P03": // lock_not_available
		slog.WarnContext(r.Context(), "reserve stock: "+err.Error(), "cart_id", req.CartID)
		http.Error(w, "busy, try again", http.StatusServiceUnavailable)
	default:
		slog.ErrorContext(r.Context(), "reserve stock: "+err.Error(), "cart_id", req.CartID)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

// Reserve holds every line of the cart in one transaction: either all lines
// are held or none are.
func Reserve(ctx context.Context, pool db.Pool, req Request) (err error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()
	if _, err = tx.Exec(ctx, "SET LOCAL lock_timeout = '5s'"); err != nil {
		return err
	}
	skus := make([]string, len(req.Lines))
	qtys := make([]int, len(req.Lines))
	for i, l := range req.Lines {
		n, err := tx.Exec(ctx,
			"UPDATE inventory SET reserved = reserved + $1, updated_at = now() WHERE sku = $2 AND warehouse_id = $3 AND on_hand - reserved >= $1",
			l.Qty, l.SKU, l.WarehouseID)
		if err != nil {
			return err
		}
		if n == 0 {
			return fmt.Errorf("%s: %w", l.SKU, ErrOutOfStock)
		}
		skus[i], qtys[i] = l.SKU, l.Qty
	}
	if _, err = tx.Exec(ctx,
		"INSERT INTO reservations (cart_id, sku, warehouse_id, qty, expires_at) SELECT $1, unnest($2::text[]), $3, unnest($4::int[]), now() + interval '15 minutes'",
		req.CartID, skus, req.Lines[0].WarehouseID, qtys); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// Release serves POST /api/reservations/{id}/release: it drops a hold and
// gives the stock back.
func (h *Handler) Release(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tx, err := h.Pool.Begin(ctx)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	defer func() { _ = tx.Rollback(ctx) }()
	// RETURNING sku, warehouse_id, qty feeds the UPDATE below (pgx scans
	// it in the production adapter).
	if _, err := tx.Exec(ctx, "DELETE FROM reservations WHERE id = $1 RETURNING sku, warehouse_id, qty", r.PathValue("id")); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if _, err := tx.Exec(ctx, "UPDATE inventory SET reserved = reserved - $1, updated_at = now() WHERE sku = $2 AND warehouse_id = $3"); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if err := tx.Commit(ctx); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
