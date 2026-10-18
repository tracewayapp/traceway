// Package statements serves account statements: a PDF of every invoice an
// account was issued in a period.
package statements

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Account is the billed customer.
type Account struct {
	ID           int64
	LegalName    string
	Currency     string
	BillingEmail string
}

// Invoice is one issued invoice.
type Invoice struct {
	ID         int64
	Number     string
	IssuedAt   time.Time
	DueAt      time.Time
	TotalCents int64
	Status     string
}

// Statement is a rendered (or rendering) statement.
type Statement struct {
	ID          string     `json:"id"`
	AccountID   int64      `json:"accountId"`
	PeriodStart time.Time  `json:"periodStart"`
	PeriodEnd   time.Time  `json:"periodEnd"`
	Status      string     `json:"status"`
	ObjectKey   string     `json:"objectKey,omitempty"`
	RenderedAt  *time.Time `json:"renderedAt,omitempty"`
}

// Store is the statements database.
type Store interface {
	Account(ctx context.Context, id int64) (*Account, error)
	InvoicesForPeriod(ctx context.Context, accountID int64, from, to time.Time) ([]Invoice, error)
	CreateStatement(ctx context.Context, st *Statement) error
	MarkReady(ctx context.Context, id, objectKey string) error
	MarkFailed(ctx context.Context, id string) error
	Statement(ctx context.Context, id string) (*Statement, error)
}

// Renderer turns an account's invoices into a PDF.
type Renderer interface {
	Render(ctx context.Context, acct *Account, invoices []Invoice) ([]byte, error)
}

// Uploader stores rendered PDFs.
type Uploader interface {
	Put(ctx context.Context, key string, body []byte) error
	PresignGet(ctx context.Context, key string, ttl time.Duration) (string, error)
}

// Handler holds the statement routes.
type Handler struct {
	store    Store
	renderer Renderer
	uploader Uploader
}

// New returns the handler.
func New(store Store, renderer Renderer, uploader Uploader) *Handler {
	return &Handler{store: store, renderer: renderer, uploader: uploader}
}

// Routes registers the statement routes on mux.
func (h *Handler) Routes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/statements", h.Create)
	mux.HandleFunc("GET /api/statements/{id}", h.Get)
	mux.HandleFunc("GET /api/statements/{id}/download", h.Download)
}

type createRequest struct {
	AccountID   int64     `json:"accountId"`
	PeriodStart time.Time `json:"periodStart"`
	PeriodEnd   time.Time `json:"periodEnd"`
}

// Create serves POST /api/statements.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if req.AccountID == 0 || !req.PeriodEnd.After(req.PeriodStart) {
		http.Error(w, "accountId and a valid period are required", http.StatusUnprocessableEntity)
		return
	}
	ctx := r.Context()
	acct, err := h.store.Account(ctx, req.AccountID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if acct == nil {
		http.NotFound(w, r)
		return
	}
	invoices, err := h.store.InvoicesForPeriod(ctx, acct.ID, req.PeriodStart, req.PeriodEnd)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	st := &Statement{ID: newID(), AccountID: acct.ID, PeriodStart: req.PeriodStart, PeriodEnd: req.PeriodEnd, Status: "rendering"}
	if err := h.store.CreateStatement(ctx, st); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	pdf, err := h.renderer.Render(ctx, acct, invoices)
	if err != nil {
		_ = h.store.MarkFailed(ctx, st.ID)
		http.Error(w, "render failed", http.StatusInternalServerError)
		return
	}
	key := fmt.Sprintf("statements/%d/%s.pdf", st.PeriodStart.Year(), st.ID)
	if err := h.uploader.Put(ctx, key, pdf); err != nil {
		_ = h.store.MarkFailed(ctx, st.ID)
		http.Error(w, "upload failed", http.StatusInternalServerError)
		return
	}
	if err := h.store.MarkReady(ctx, st.ID, key); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	st.Status, st.ObjectKey = "ready", key
	writeJSON(w, http.StatusCreated, st)
}

// Get serves GET /api/statements/{id}; the dashboard polls it.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	st, err := h.store.Statement(r.Context(), r.PathValue("id"))
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if st == nil {
		http.NotFound(w, r)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

// Download serves GET /api/statements/{id}/download.
func (h *Handler) Download(w http.ResponseWriter, r *http.Request) {
	st, err := h.store.Statement(r.Context(), r.PathValue("id"))
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if st == nil || st.Status != "ready" {
		http.NotFound(w, r)
		return
	}
	url, err := h.uploader.PresignGet(r.Context(), st.ObjectKey, 15*time.Minute)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, url, http.StatusFound)
}

func newID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
