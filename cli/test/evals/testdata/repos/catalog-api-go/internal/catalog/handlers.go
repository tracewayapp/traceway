// Package catalog serves category listings and product pages.
package catalog

import (
	"encoding/json"
	"net/http"
	"strconv"
)

// Handlers serves the catalog routes.
type Handlers struct {
	store Store
	cache *Cache
}

// NewHandlers returns the catalog handlers.
func NewHandlers(s Store, c *Cache) *Handlers { return &Handlers{store: s, cache: c} }

const pageSize = 48

// Listing serves GET /api/categories/{slug}/products?page=N.
func (h *Handlers) Listing(w http.ResponseWriter, r *http.Request) {
	all, err := h.cache.Get(r.Context(), r.PathValue("slug"))
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	start := min((page-1)*pageSize, len(all))
	end := min(start+pageSize, len(all))
	out := make([]Product, 0, end-start)
	for _, p := range all[start:end] {
		if p.Available > 0 && p.Available < 5 {
			p.Badge = "low-stock"
		}
		out = append(out, p)
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"page": page, "total": len(all), "products": out})
}

// Product serves GET /api/products/{id}.
func (h *Handlers) Product(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "bad product id", http.StatusBadRequest)
		return
	}
	p, err := h.store.Product(r.Context(), id)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if p == nil {
		http.NotFound(w, r)
		return
	}
	_ = json.NewEncoder(w).Encode(p)
}
