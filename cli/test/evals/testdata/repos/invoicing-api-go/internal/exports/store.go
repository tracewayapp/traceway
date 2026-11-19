package exports

import (
	"context"
	"database/sql"
	"time"
)

// Invoice is one exported invoice with its lines and tax breakdown.
type Invoice struct {
	ID           int64
	Number       string
	IssuedAt     time.Time
	CustomerName string
	TotalCents   int64
	Lines        []Line
	Taxes        []TaxLine
}

// Line is one invoice line.
type Line struct {
	SKU            string
	Description    string
	Qty            int64
	UnitPriceCents int64
}

// TaxLine is one tax amount of an invoice.
type TaxLine struct {
	RateCode    string
	RateBps     int64
	AmountCents int64
}

// Store is the data the export reads.
type Store interface {
	// InvoicesWithLines returns the organization's invoices issued in
	// [from, to), each with its lines, in issue order.
	InvoicesWithLines(ctx context.Context, orgID int64, from, to time.Time) ([]Invoice, error)
	// TaxLines returns one invoice's tax breakdown.
	TaxLines(ctx context.Context, invoiceID int64) ([]TaxLine, error)
}

// SQLStore implements Store on PostgreSQL.
type SQLStore struct{ db *sql.DB }

// NewStore returns a Store backed by db.
func NewStore(db *sql.DB) *SQLStore { return &SQLStore{db: db} }

// InvoicesWithLines loads the month in one JOIN.
func (s *SQLStore) InvoicesWithLines(ctx context.Context, orgID int64, from, to time.Time) ([]Invoice, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT i.id, i.number, i.issued_at, i.customer_name, i.total_cents, l.sku, l.description, l.qty, l.unit_price_cents FROM invoices i JOIN invoice_lines l ON l.invoice_id = i.id WHERE i.org_id = $1 AND i.issued_at >= $2 AND i.issued_at < $3 ORDER BY i.issued_at, i.id`, orgID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Invoice
	for rows.Next() {
		var inv Invoice
		var l Line
		if err := rows.Scan(&inv.ID, &inv.Number, &inv.IssuedAt, &inv.CustomerName, &inv.TotalCents, &l.SKU, &l.Description, &l.Qty, &l.UnitPriceCents); err != nil {
			return nil, err
		}
		if n := len(out); n > 0 && out[n-1].ID == inv.ID {
			out[n-1].Lines = append(out[n-1].Lines, l)
			continue
		}
		inv.Lines = []Line{l}
		out = append(out, inv)
	}
	return out, rows.Err()
}

// TaxLines loads one invoice's tax breakdown.
func (s *SQLStore) TaxLines(ctx context.Context, invoiceID int64) ([]TaxLine, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT rate_code, rate_bps, amount_cents FROM invoice_tax_lines WHERE invoice_id = $1`, invoiceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TaxLine
	for rows.Next() {
		var t TaxLine
		if err := rows.Scan(&t.RateCode, &t.RateBps, &t.AmountCents); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
