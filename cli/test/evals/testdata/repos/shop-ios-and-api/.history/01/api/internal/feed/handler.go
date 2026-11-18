// Package feed serves the home feed: promotions for the user's segments and
// their recently viewed products.
package feed

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"example.com/mobile-api/internal/telemetry"
)

type promotion struct {
	ID         int64  `json:"id"`
	Title      string `json:"title"`
	ImageURL   string `json:"imageUrl"`
	PriceCents int64  `json:"priceCents"`
}

// Handler serves GET /api/feed.
func Handler(db *sql.DB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		userID, _ := strconv.ParseInt(r.Header.Get("X-User-Id"), 10, 64)
		segments := strings.Split(r.Header.Get("X-Segments"), ",")
		rows, err := db.QueryContext(ctx, `SELECT p.id, p.title, p.image_url, p.price_cents FROM promotions p WHERE p.active AND p.segment = ANY($1) ORDER BY p.priority DESC LIMIT 30`, segments)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		promos := []promotion{}
		for rows.Next() {
			var p promotion
			if err := rows.Scan(&p.ID, &p.Title, &p.ImageURL, &p.PriceCents); err != nil {
				rows.Close()
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
			promos = append(promos, p)
		}
		rows.Close()
		viewed := map[int64]int{}
		vrows, err := db.QueryContext(ctx, `SELECT product_id, count(*) FROM recent_views WHERE user_id = $1 GROUP BY product_id ORDER BY count(*) DESC LIMIT 20`, userID)
		if err == nil {
			for vrows.Next() {
				var id int64
				var n int
				if vrows.Scan(&id, &n) == nil {
					viewed[id] = n
				}
			}
			vrows.Close()
		}
		_, end := telemetry.Start(ctx, "assemble feed")
		defer end()
		_ = json.NewEncoder(w).Encode(map[string]any{"promotions": promos, "recentlyViewed": viewed})
	})
}

// Product serves GET /api/products/{id}.
func Product(db *sql.DB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.Error(w, "bad product id", http.StatusBadRequest)
			return
		}
		var p struct {
			ID          int64  `json:"id"`
			Name        string `json:"name"`
			Description string `json:"description"`
			PriceCents  int64  `json:"priceCents"`
			Images      string `json:"images"`
		}
		err = db.QueryRowContext(r.Context(), `SELECT id, name, description, price_cents, images FROM products WHERE id = $1`, id).
			Scan(&p.ID, &p.Name, &p.Description, &p.PriceCents, &p.Images)
		if err == sql.ErrNoRows {
			http.NotFound(w, r)
			return
		}
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		_, end := telemetry.Start(r.Context(), "encode response")
		defer end()
		_ = json.NewEncoder(w).Encode(p)
	})
}
