package home

import (
	"encoding/json"
	"net/http"
)

// Handler serves GET /api/home.
func (s *Service) Handler(w http.ResponseWriter, r *http.Request) {
	f, err := s.Feed(r.Context())
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(f)
}
