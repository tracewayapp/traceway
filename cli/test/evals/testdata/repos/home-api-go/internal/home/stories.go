package home

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"example.com/home-api/internal/telemetry"
)

// Stories serves the seasonal stories linked from the home feed.
type Stories struct{ DB *sql.DB }

type story struct {
	ID          int64     `json:"id"`
	Title       string    `json:"title"`
	Body        string    `json:"body"`
	ImageURL    string    `json:"imageUrl"`
	PublishedAt time.Time `json:"publishedAt"`
}

// Handler serves GET /api/home/stories/{id}.
func (s *Stories) Handler(w http.ResponseWriter, r *http.Request) {
	var st story
	err := s.DB.QueryRowContext(r.Context(), `SELECT id, title, body, image_url, published_at FROM seasonal_stories WHERE id = $1`, r.PathValue("id")).
		Scan(&st.ID, &st.Title, &st.Body, &st.ImageURL, &st.PublishedAt)
	if errors.Is(err, sql.ErrNoRows) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	_, end := telemetry.Start(r.Context(), "encode response")
	defer end()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(st)
}
