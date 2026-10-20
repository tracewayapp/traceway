// Package invoices serves the invoice CRUD routes.
package invoices

import (
	"context"
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
	Lines        []line    `json:"lines,omitempty"`
	Taxes        []taxLine `json:"taxes,omitempty"`
}

type line struct {
	SKU            string `json:"sku"`
	Description    string `json:"description"`
	Qty            int64  `json:"qty"`
	UnitPriceCents int64  `json:"unitPriceCents"`
}

type taxLine struct {
	RateCode    string `json:"rateCode"`
	RateBps     int64  `json:"rateBps"`
	AmountCents int64  `json:"amountCents"`
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

// Get serves GET /api/invoices/{id}: the invoice with its lines and taxes.
func (h *Handlers) Get(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "bad invoice id", http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	var i invoice
	err = h.db.QueryRowContext(ctx, `SELECT id, number, issued_at, customer_name, total_cents, status FROM invoices WHERE id = $1`, id).
		Scan(&i.ID, &i.Number, &i.IssuedAt, &i.CustomerName, &i.TotalCents, &i.Status)
	if err == sql.ErrNoRows {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if i.Lines, err = queryAll(ctx, h.db, `SELECT sku, description, qty, unit_price_cents FROM invoice_lines WHERE invoice_id = $1`, id,
		func(rows *sql.Rows, l *line) error {
			return rows.Scan(&l.SKU, &l.Description, &l.Qty, &l.UnitPriceCents)
		}); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if i.Taxes, err = queryAll(ctx, h.db, `SELECT rate_code, rate_bps, amount_cents FROM invoice_tax_lines WHERE invoice_id = $1`, id,
		func(rows *sql.Rows, t *taxLine) error { return rows.Scan(&t.RateCode, &t.RateBps, &t.AmountCents) }); err != nil {
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
	ctx := r.Context()
	err := h.db.QueryRowContext(ctx, `INSERT INTO invoices (org_id, number, issued_at, customer_name, total_cents, status) VALUES ($1, $2, $3, $4, $5, 'open') RETURNING id`,
		orgID, in.Number, in.IssuedAt, in.CustomerName, in.TotalCents).Scan(&in.ID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	var skus, descs []string
	var qtys, prices []int64
	for _, l := range in.Lines {
		skus, descs, qtys, prices = append(skus, l.SKU), append(descs, l.Description), append(qtys, l.Qty), append(prices, l.UnitPriceCents)
	}
	if _, err := h.db.ExecContext(ctx, `INSERT INTO invoice_lines (invoice_id, sku, description, qty, unit_price_cents) SELECT $1, * FROM unnest($2::text[], $3::text[], $4::int[], $5::bigint[])`,
		in.ID, skus, descs, qtys, prices); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	var codes []string
	var bps, amounts []int64
	for _, t := range in.Taxes {
		codes, bps, amounts = append(codes, t.RateCode), append(bps, t.RateBps), append(amounts, t.AmountCents)
	}
	if _, err := h.db.ExecContext(ctx, `INSERT INTO invoice_tax_lines (invoice_id, rate_code, rate_bps, amount_cents) SELECT $1, * FROM unnest($2::text[], $3::int[], $4::bigint[])`,
		in.ID, codes, bps, amounts); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	in.Status = "open"
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(in)
}

func queryAll[T any](ctx context.Context, db *sql.DB, query string, id int64, scan func(*sql.Rows, *T) error) ([]T, error) {
	rows, err := db.QueryContext(ctx, query, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []T{}
	for rows.Next() {
		var v T
		if err := scan(rows, &v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
