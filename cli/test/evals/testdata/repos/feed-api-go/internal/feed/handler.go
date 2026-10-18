// Package feed serves the home feed.
package feed

import (
	"encoding/json"
	"net/http"
	"sort"
	"time"
)

// Handler serves the feed routes.
type Handler struct {
	Store   Store
	CDNBase string
}

type cardJSON struct {
	ID          int64      `json:"id"`
	Kind        string     `json:"kind"`
	Title       string     `json:"title"`
	Body        string     `json:"body"`
	Image       string     `json:"image"`
	PublishedAt time.Time  `json:"publishedAt"`
	Author      authorJSON `json:"author"`
}

type authorJSON struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Avatar string `json:"avatar"`
}

// Feed serves GET /api/feed: the latest 50 cards, ranked.
func (h *Handler) Feed(w http.ResponseWriter, r *http.Request) {
	cards, err := h.Store.Latest(r.Context(), audiences(r), 50)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	rank(cards)
	out := make([]cardJSON, 0, len(cards))
	for _, c := range cards {
		out = append(out, cardJSON{
			ID: c.ID, Kind: c.Kind, Title: c.Title, Body: c.Body, Image: c.ImageData, PublishedAt: c.PublishedAt,
			Author: authorJSON{ID: c.Author.ID, Name: c.Author.DisplayName, Avatar: c.Author.AvatarData},
		})
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"cards": out})
}

func audiences(r *http.Request) []string {
	if a := r.Header.Get("X-Audience"); a != "" {
		return []string{"all", a}
	}
	return []string{"all"}
}

// rank orders cards by recency, pinned announcements first.
func rank(cards []Card) {
	sort.SliceStable(cards, func(i, j int) bool {
		pi, pj := cards[i].Kind == "announcement", cards[j].Kind == "announcement"
		if pi != pj {
			return pi
		}
		return cards[i].PublishedAt.After(cards[j].PublishedAt)
	})
}
