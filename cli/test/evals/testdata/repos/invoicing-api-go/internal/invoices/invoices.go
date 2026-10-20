// Package invoices serves the invoice CRUD routes.
package invoices

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"time"
)

// Handlers serves /api/invoices.
type Handlers struct{ db *sql.DB }

// New returns the invoice handlers.
func New(db *sql.DB) *Handlers { return &Handlers{db: db} }

type invoice struct {
	ID           int64     `json:"id"`
	Number       string    `json:"number"`
	IssuedAt     time.Time `json:"issuedAt"`
	CustomerName string    `json:"customerName"`
	TotalCents   int64     `json:"totalCents"`
	Status       string    `json:"status"`
}

// List serves GET /api/invoices: the latest 50 of the organization.
func (h *Handlers) List(w http.ResponseWriter, r *http.Request) {
	orgID, err := strconv.ParseInt(r.Header.Get("X-Org-Id"), 10, 64)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	rows, err := h.db.QueryContext(r.Context(), `SELECT id, number, issued_at, customer_name, total_cents, status FROM invoices WHERE org_id = $1 ORDER BY issued_at DESC LIMIT 50`, orgID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	out := []invoice{}
	for rows.Next() {
		var i invoice
		if err := rows.Scan(&i.ID, &i.Number, &i.IssuedAt, &i.CustomerName, &i.TotalCents, &i.Status); err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		out = append(out, i)
	}
	_ = json.NewEncoder(w).Encode(out)
}

// Get serves GET /api/invoices/{id}.
func (h *Handlers) Get(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "bad invoice id", http.StatusBadRequest)
		return
	}
	var i invoice
	err = h.db.QueryRowContext(r.Context(), `SELECT id, number, issued_at, customer_name, total_cents, status FROM invoices WHERE id = $1`, id).
		Scan(&i.ID, &i.Number, &i.IssuedAt, &i.CustomerName, &i.TotalCents, &i.Status)
	if err == sql.ErrNoRows {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	_ = json.NewEncoder(w).Encode(i)
}

// Create serves POST /api/invoices.
func (h *Handlers) Create(w http.ResponseWriter, r *http.Request) {
	var in invoice
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	orgID, _ := strconv.ParseInt(r.Header.Get("X-Org-Id"), 10, 64)
	err := h.db.QueryRowContext(r.Context(), `INSERT INTO invoices (org_id, number, issued_at, customer_name, total_cents, status) VALUES ($1, $2, $3, $4, $5, 'open') RETURNING id`,
		orgID, in.Number, in.IssuedAt, in.CustomerName, in.TotalCents).Scan(&in.ID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(in)
}
