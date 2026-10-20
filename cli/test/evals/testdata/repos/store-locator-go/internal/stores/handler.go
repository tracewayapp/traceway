// Package stores serves the store finder.
package stores

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"example.com/store-locator/internal/inventory"
	"example.com/store-locator/internal/maps"
)

// Store is one shop location.
type Store struct {
	ID      int     `json:"id"`
	Name    string  `json:"name"`
	Address string  `json:"address"`
	Lat     float64 `json:"lat"`
	Lng     float64 `json:"lng"`
	Meters  int     `json:"meters"`
}

// Finder looks up stores near a point (PostGIS in production).
type Finder interface {
	Near(ctx context.Context, p maps.Point, limit int) ([]Store, error)
}

// Handler serves /api/stores.
type Handler struct {
	Maps      *maps.Client
	Inventory *inventory.Client
	Stores    Finder
}

// Availability serves GET /api/stores/{id}/availability.
func (h *Handler) Availability(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "bad store id", http.StatusBadRequest)
		return
	}
	stock, err := h.Inventory.Stock(r.Context(), id)
	if err != nil {
		http.Error(w, "inventory unavailable", http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(stock)
}

// Nearby serves GET /api/stores/nearby?q=...
func (h *Handler) Nearby(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		http.Error(w, "q is required", http.StatusBadRequest)
		return
	}
	origin, err := h.Maps.Geocode(r.Context(), q)
	if err != nil {
		http.Error(w, "could not resolve the location", http.StatusBadGateway)
		return
	}
	stores, err := h.Stores.Near(r.Context(), origin, 10)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	dest := make([]string, len(stores))
	for i, s := range stores {
		dest[i] = fmt.Sprintf("store:%d", s.ID)
	}
	if len(dest) > 0 {
		meters, err := h.Maps.Distances(r.Context(), origin, dest)
		if err == nil {
			for i := range stores {
				if i < len(meters) {
					stores[i].Meters = meters[i]
				}
			}
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(stores)
}
