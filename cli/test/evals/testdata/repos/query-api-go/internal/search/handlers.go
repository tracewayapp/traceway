package search

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"example.com/query-api/internal/telemetry"
)

// Store runs the database side of search.
type Store interface {
	Candidates(ctx context.Context, tenantID int64, q string) ([]Hit, error)
	Suggest(ctx context.Context, tenantID int64, prefix string) ([]Suggestion, error)
	Document(ctx context.Context, tenantID, id int64) (*Document, error)
	SaveFeedback(ctx context.Context, tenantID int64, f Feedback) error
}

// Suggestion is one typeahead term.
type Suggestion struct {
	Term string `json:"term"`
	Hits int64  `json:"hits"`
}

// Document is one workspace document.
type Document struct {
	ID        int64     `json:"id"`
	Title     string    `json:"title"`
	Body      string    `json:"-"`
	HTML      string    `json:"html"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Feedback is a user's verdict on one result of a query.
type Feedback struct {
	Query      string `json:"query"`
	DocumentID int64  `json:"documentId"`
	Verdict    string `json:"verdict"`
}

// Handlers serves the search routes.
type Handlers struct{ store Store }

// NewHandlers returns the handlers.
func NewHandlers(s Store) *Handlers { return &Handlers{store: s} }

func tenantOf(r *http.Request) int64 {
	tenant, _ := strconv.ParseInt(r.Header.Get("X-Tenant-Id"), 10, 64)
	return tenant
}

// Query serves GET /api/query?q=.
func (h *Handlers) Query(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	_, end := telemetry.Start(ctx, "parse query")
	q := r.URL.Query().Get("q")
	terms := Terms(q)
	end()
	if len(terms) == 0 {
		writeJSON(w, []Hit{})
		return
	}
	hits, err := h.store.Candidates(ctx, tenantOf(r), q)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	_, end = telemetry.Start(ctx, "rank results")
	top := Rank(terms, hits)
	end()
	_, end = telemetry.Start(ctx, "render highlights")
	for i := range top {
		top[i].Highlight = Highlight(top[i].Snippet, terms)
	}
	end()
	writeJSON(w, top)
}

// Suggest serves GET /api/suggest?prefix=.
func (h *Handlers) Suggest(w http.ResponseWriter, r *http.Request) {
	out, err := h.store.Suggest(r.Context(), tenantOf(r), r.URL.Query().Get("prefix"))
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, out)
}

// Document serves GET /api/documents/{id}.
func (h *Handlers) Document(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "bad document id", http.StatusBadRequest)
		return
	}
	doc, err := h.store.Document(r.Context(), tenantOf(r), id)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if doc == nil {
		http.NotFound(w, r)
		return
	}
	_, end := telemetry.Start(r.Context(), "render document")
	doc.HTML = RenderHTML(doc.Body)
	end()
	writeJSON(w, doc)
}

// Feedback serves POST /api/feedback.
func (h *Handlers) Feedback(w http.ResponseWriter, r *http.Request) {
	var f Feedback
	if err := json.NewDecoder(r.Body).Decode(&f); err != nil || f.DocumentID == 0 {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if err := h.store.SaveFeedback(r.Context(), tenantOf(r), f); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
