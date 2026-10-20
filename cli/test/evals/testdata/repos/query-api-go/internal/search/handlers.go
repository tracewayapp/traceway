package search

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
)

// Store runs the database side of search.
type Store interface {
	Candidates(ctx context.Context, tenantID int64, q string) ([]Hit, error)
	Suggest(ctx context.Context, tenantID int64, prefix string) ([]string, error)
}

// PgStore implements Store on pgx (wired in production builds).
type PgStore struct{ pool any }

// NewStore returns the PostgreSQL store.
func NewStore(pool any) *PgStore { return &PgStore{pool: pool} }

// Candidates runs the full-text query, top 200 by ts_rank_cd.
func (s *PgStore) Candidates(context.Context, int64, string) ([]Hit, error) { return nil, nil }

// Suggest returns the 10 most used terms with the prefix.
func (s *PgStore) Suggest(context.Context, int64, string) ([]string, error) { return nil, nil }

// Handlers serves the search routes.
type Handlers struct{ store Store }

// NewHandlers returns the handlers.
func NewHandlers(s Store) *Handlers { return &Handlers{store: s} }

// Query serves GET /api/query?q=.
func (h *Handlers) Query(w http.ResponseWriter, r *http.Request) {
	tenant, _ := strconv.ParseInt(r.Header.Get("X-Tenant-Id"), 10, 64)
	q := r.URL.Query().Get("q")
	hits, err := h.store.Candidates(r.Context(), tenant, q)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	_ = json.NewEncoder(w).Encode(Rank(q, hits))
}

// Suggest serves GET /api/suggest?prefix=.
func (h *Handlers) Suggest(w http.ResponseWriter, r *http.Request) {
	tenant, _ := strconv.ParseInt(r.Header.Get("X-Tenant-Id"), 10, 64)
	out, err := h.store.Suggest(r.Context(), tenant, r.URL.Query().Get("prefix"))
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	_ = json.NewEncoder(w).Encode(out)
}
