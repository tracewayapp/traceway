package listings

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
)

// Handler holds the listing routes.
type Handler struct {
	repo *Repo
	log  *slog.Logger
}

// NewHandler returns the handler.
func NewHandler(repo *Repo, log *slog.Logger) *Handler { return &Handler{repo: repo, log: log} }

// Routes registers the routes.
func (h *Handler) Routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/listings", h.search)
	mux.HandleFunc("GET /api/listings/{id}", h.show)
	mux.HandleFunc("POST /api/listings/{id}/views", h.view)
	mux.HandleFunc("GET /api/categories", h.categories)
}

func (h *Handler) categories(w http.ResponseWriter, r *http.Request) {
	tree, err := h.repo.Categories(r.Context())
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(tree)
}

func (h *Handler) search(w http.ResponseWriter, r *http.Request) {
	category, err := strconv.ParseInt(r.URL.Query().Get("category"), 10, 64)
	if err != nil {
		http.Error(w, "category is required", http.StatusBadRequest)
		return
	}
	out, err := h.repo.Search(r.Context(), category, r.URL.Query().Get("cursor"))
	if err != nil {
		h.log.ErrorContext(r.Context(), "search failed", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	h.log.InfoContext(r.Context(), "search served", "category.id", category, "results", len(out))
	writeJSON(w, out)
}

func (h *Handler) show(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	l, err := h.repo.Get(r.Context(), id)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, l)
}

func (h *Handler) view(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	viewer, _ := strconv.ParseInt(r.Header.Get("X-Viewer-Id"), 10, 64)
	if err := h.repo.RecordView(r.Context(), id, viewer); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
