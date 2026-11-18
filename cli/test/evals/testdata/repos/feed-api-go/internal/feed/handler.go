// Package feed serves the home feed.
package feed

import (
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"time"

	"example.com/feed-api/internal/telemetry"
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
	_, end := telemetry.Start(r.Context(), "rank cards")
	rank(cards)
	end()
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

// Card serves GET /api/feed/cards/{id}: one card.
func (h *Handler) Card(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "bad card id", http.StatusBadRequest)
		return
	}
	c, err := h.Store.Card(r.Context(), id)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if c == nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(cardJSON{
		ID: c.ID, Kind: c.Kind, Title: c.Title, Body: c.Body, Image: c.ImageData, PublishedAt: c.PublishedAt,
		Author: authorJSON{ID: c.Author.ID, Name: c.Author.DisplayName},
	})
}

// Author serves GET /api/authors/{id}: an author's profile.
func (h *Handler) Author(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "bad author id", http.StatusBadRequest)
		return
	}
	p, err := h.Store.Profile(r.Context(), id)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if p == nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(p)
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
