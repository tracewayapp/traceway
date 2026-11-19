// Package accounts serves balances and journal entries.
package accounts

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"time"
)

// Handlers serves the account routes.
type Handlers struct{ db *sql.DB }

// New returns the account handlers.
func New(db *sql.DB) *Handlers { return &Handlers{db: db} }

// Balance serves GET /api/accounts/{id}/balance.
func (h *Handlers) Balance(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "bad account id", http.StatusBadRequest)
		return
	}
	var out struct {
		BalanceCents int64     `json:"balanceCents"`
		Currency     string    `json:"currency"`
		UpdatedAt    time.Time `json:"updatedAt"`
	}
	err = h.db.QueryRowContext(r.Context(), `SELECT balance_cents, currency, updated_at FROM account_balances WHERE account_id = $1`, id).
		Scan(&out.BalanceCents, &out.Currency, &out.UpdatedAt)
	if err == sql.ErrNoRows {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	_ = json.NewEncoder(w).Encode(out)
}

// Entries serves GET /api/accounts/{id}/entries: the latest 50.
func (h *Handlers) Entries(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "bad account id", http.StatusBadRequest)
		return
	}
	rows, err := h.db.QueryContext(r.Context(), `SELECT id, posted_at, description, amount_cents FROM ledger_entries WHERE account_id = $1 ORDER BY posted_at DESC LIMIT 50`, id)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	type entry struct {
		ID          int64     `json:"id"`
		PostedAt    time.Time `json:"postedAt"`
		Description string    `json:"description"`
		AmountCents int64     `json:"amountCents"`
	}
	out := []entry{}
	for rows.Next() {
		var e entry
		if err := rows.Scan(&e.ID, &e.PostedAt, &e.Description, &e.AmountCents); err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		out = append(out, e)
	}
	_ = json.NewEncoder(w).Encode(out)
}

// Post serves POST /api/journal-entries.
func (h *Handlers) Post(w http.ResponseWriter, r *http.Request) {
	var in struct {
		OrgID       int64     `json:"orgId"`
		AccountID   int64     `json:"accountId"`
		PostedAt    time.Time `json:"postedAt"`
		Description string    `json:"description"`
		AmountCents int64     `json:"amountCents"`
		Currency    string    `json:"currency"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	if _, err := h.db.ExecContext(ctx, `INSERT INTO ledger_entries (org_id, account_id, posted_at, description, amount_cents, currency) VALUES ($1, $2, $3, $4, $5, $6)`,
		in.OrgID, in.AccountID, in.PostedAt, in.Description, in.AmountCents, in.Currency); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if _, err := h.db.ExecContext(ctx, `UPDATE account_balances SET balance_cents = balance_cents + $2, updated_at = now() WHERE account_id = $1`, in.AccountID, in.AmountCents); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
}
