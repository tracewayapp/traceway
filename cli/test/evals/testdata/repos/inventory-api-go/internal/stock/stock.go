// Package stock serves stock levels and the warehouse list.
package stock

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Level is the stock of one SKU in one warehouse.
type Level struct {
	SKU         string    `json:"sku"`
	WarehouseID int64     `json:"warehouseId"`
	OnHand      int       `json:"onHand"`
	Reserved    int       `json:"reserved"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// Warehouse is one fulfillment warehouse.
type Warehouse struct {
	ID       int64  `json:"id"`
	Code     string `json:"code"`
	Name     string `json:"name"`
	Timezone string `json:"timezone"`
}

// Handler serves the stock routes.
type Handler struct{ DB *pgxpool.Pool }

// Levels serves GET /api/inventory/{sku}: the SKU's stock per warehouse.
func (h *Handler) Levels(w http.ResponseWriter, r *http.Request) {
	rows, err := h.DB.Query(r.Context(), "SELECT sku, warehouse_id, on_hand, reserved, updated_at FROM inventory WHERE sku = $1", r.PathValue("sku"))
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	levels, err := pgx.CollectRows(rows, pgx.RowToStructByPos[Level])
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if len(levels) == 0 {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	writeJSON(w, levels)
}

// Warehouses serves GET /api/warehouses.
func (h *Handler) Warehouses(w http.ResponseWriter, r *http.Request) {
	rows, err := h.DB.Query(r.Context(), "SELECT id, code, name, timezone FROM warehouses ORDER BY code")
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	out, err := pgx.CollectRows(rows, pgx.RowToStructByPos[Warehouse])
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, out)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
