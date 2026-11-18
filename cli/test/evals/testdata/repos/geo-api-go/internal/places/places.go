// Package places serves place search.
package places

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/acme/geo-api/internal/memo"
)

// Place is one searchable place.
type Place struct {
	ID       int64   `json:"id"`
	Name     string  `json:"name"`
	Kind     string  `json:"kind"`
	Country  string  `json:"country"`
	Lat      float64 `json:"lat"`
	Lon      float64 `json:"lon"`
	Address  string  `json:"address,omitempty"`
	Timezone string  `json:"timezone,omitempty"`
}

// Handler serves the place routes.
type Handler struct {
	db *pgxpool.Pool
	// suggestions memoizes autocomplete results per locale and prefix for
	// ten minutes; most keystrokes repeat a prefix someone typed recently.
	suggestions *memo.Memo[[]Place]
}

// NewHandler returns the handler.
func NewHandler(db *pgxpool.Pool) *Handler {
	return &Handler{db: db, suggestions: memo.New[[]Place](10 * time.Minute)}
}

// Routes registers the routes.
func (h *Handler) Routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/autocomplete", h.autocomplete)
	mux.HandleFunc("GET /api/places/{id}", h.place)
	mux.HandleFunc("GET /api/reverse", h.reverse)
	mux.HandleFunc("POST /api/geocode/batch", h.batch)
}

func (h *Handler) autocomplete(w http.ResponseWriter, r *http.Request) {
	q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	if len(q) < 2 {
		writeJSON(w, []Place{})
		return
	}
	key := r.URL.Query().Get("locale") + "|" + q
	if hit, ok := h.suggestions.Get(key); ok {
		writeJSON(w, hit)
		return
	}
	out, err := h.query(r.Context(), `SELECT id, name, kind, country, lat, lon FROM places WHERE name_prefix @@ to_tsquery($1) ORDER BY popularity DESC LIMIT 10`, q+":*")
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	h.suggestions.Put(key, out)
	writeJSON(w, out)
}

func (h *Handler) place(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	var p Place
	err = h.db.QueryRow(r.Context(), `SELECT id, name, kind, country, lat, lon, address, timezone FROM places WHERE id = $1`, id).
		Scan(&p.ID, &p.Name, &p.Kind, &p.Country, &p.Lat, &p.Lon, &p.Address, &p.Timezone)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	writeJSON(w, p)
}

func (h *Handler) reverse(w http.ResponseWriter, r *http.Request) {
	lat, err1 := strconv.ParseFloat(r.URL.Query().Get("lat"), 64)
	lon, err2 := strconv.ParseFloat(r.URL.Query().Get("lon"), 64)
	if err1 != nil || err2 != nil {
		http.Error(w, "lat and lon are required", http.StatusBadRequest)
		return
	}
	rows, err := h.db.Query(r.Context(), `SELECT id, name, kind, country FROM places WHERE ST_DWithin(geom, ST_SetSRID(ST_MakePoint($1, $2), 4326), 200) ORDER BY geom <-> ST_SetSRID(ST_MakePoint($1, $2), 4326) LIMIT 5`, lon, lat)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	var out []Place
	for rows.Next() {
		var p Place
		if err := rows.Scan(&p.ID, &p.Name, &p.Kind, &p.Country); err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		out = append(out, p)
	}
	writeJSON(w, out)
}

func (h *Handler) query(ctx context.Context, sql string, args ...any) ([]Place, error) {
	rows, err := h.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Place
	for rows.Next() {
		var p Place
		if err := rows.Scan(&p.ID, &p.Name, &p.Kind, &p.Country, &p.Lat, &p.Lon); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
