package statements

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// SQLStore is the PostgreSQL Store.
type SQLStore struct{ db *sql.DB }

// NewSQLStore returns a Store backed by db.
func NewSQLStore(db *sql.DB) *SQLStore { return &SQLStore{db: db} }

func (s *SQLStore) Account(ctx context.Context, id int64) (*Account, error) {
	var a Account
	err := s.db.QueryRowContext(ctx, `SELECT id, legal_name, currency, billing_email FROM accounts WHERE id = $1`, id).
		Scan(&a.ID, &a.LegalName, &a.Currency, &a.BillingEmail)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return &a, err
}

func (s *SQLStore) InvoicesForPeriod(ctx context.Context, accountID int64, from, to time.Time) ([]Invoice, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, number, issued_at, due_at, total_cents, status FROM invoices WHERE account_id = $1 AND issued_at >= $2 AND issued_at < $3 ORDER BY issued_at`, accountID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Invoice
	for rows.Next() {
		var inv Invoice
		if err := rows.Scan(&inv.ID, &inv.Number, &inv.IssuedAt, &inv.DueAt, &inv.TotalCents, &inv.Status); err != nil {
			return nil, err
		}
		out = append(out, inv)
	}
	return out, rows.Err()
}

func (s *SQLStore) LatestInvoices(ctx context.Context, accountID int64) ([]Invoice, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, number, issued_at, total_cents, status FROM invoices WHERE account_id = $1 ORDER BY issued_at DESC LIMIT 50`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Invoice
	for rows.Next() {
		var inv Invoice
		if err := rows.Scan(&inv.ID, &inv.Number, &inv.IssuedAt, &inv.TotalCents, &inv.Status); err != nil {
			return nil, err
		}
		out = append(out, inv)
	}
	return out, rows.Err()
}

func (s *SQLStore) CreateStatement(ctx context.Context, st *Statement) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO statements (id, account_id, period_start, period_end, status, created_at) VALUES ($1, $2, $3, $4, 'rendering', now())`,
		st.ID, st.AccountID, st.PeriodStart, st.PeriodEnd)
	return err
}

func (s *SQLStore) MarkReady(ctx context.Context, id, objectKey string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE statements SET status = 'ready', object_key = $2, rendered_at = now() WHERE id = $1`, id, objectKey)
	return err
}

func (s *SQLStore) MarkFailed(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE statements SET status = 'failed' WHERE id = $1`, id)
	return err
}

func (s *SQLStore) Statement(ctx context.Context, id string) (*Statement, error) {
	var st Statement
	var key sql.NullString
	var rendered sql.NullTime
	err := s.db.QueryRowContext(ctx, `SELECT id, account_id, status, object_key, rendered_at FROM statements WHERE id = $1`, id).
		Scan(&st.ID, &st.AccountID, &st.Status, &key, &rendered)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	st.ObjectKey = key.String
	if rendered.Valid {
		st.RenderedAt = &rendered.Time
	}
	return &st, nil
}
