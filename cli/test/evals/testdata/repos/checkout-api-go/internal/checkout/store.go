package checkout

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// SQLStore implements Store on PostgreSQL.
type SQLStore struct{ db *sql.DB }

// NewStore returns a Store backed by db.
func NewStore(db *sql.DB) *SQLStore { return &SQLStore{db: db} }

func (s *SQLStore) Order(ctx context.Context, id, customerID int64) (*Order, error) {
	var o Order
	err := s.db.QueryRowContext(ctx, `SELECT id, status, total_cents, currency, payment_method FROM orders WHERE id = $1 AND customer_id = $2`, id, customerID).
		Scan(&o.ID, &o.Status, &o.TotalCents, &o.Currency, &o.PaymentMethod)
	return found(&o, err)
}

func (s *SQLStore) Lines(ctx context.Context, orderID int64) ([]Line, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT sku, qty, unit_price_cents FROM order_lines WHERE order_id = $1`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Line
	for rows.Next() {
		var l Line
		if err := rows.Scan(&l.SKU, &l.Qty, &l.UnitPriceCents); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

func (s *SQLStore) OrderByID(ctx context.Context, id int64) (*Order, error) {
	var o Order
	err := s.db.QueryRowContext(ctx, `SELECT id, status, total_cents, currency, paid_at FROM orders WHERE id = $1`, id).
		Scan(&o.ID, &o.Status, &o.TotalCents, &o.Currency, &o.PaidAt)
	return found(&o, err)
}

func (s *SQLStore) PaidOrder(ctx context.Context, id int64) (*Order, error) {
	var o Order
	var chargeID sql.NullString
	err := s.db.QueryRowContext(ctx, `SELECT id, status, charge_id, total_cents FROM orders WHERE id = $1`, id).
		Scan(&o.ID, &o.Status, &chargeID, &o.TotalCents)
	o.ChargeID = chargeID.String
	return found(&o, err)
}

func (s *SQLStore) MarkPaid(ctx context.Context, id int64, chargeID string) error {
	if _, err := s.db.ExecContext(ctx, `UPDATE orders SET status = 'paid', paid_at = now(), charge_id = $2 WHERE id = $1`, id, chargeID); err != nil {
		return err
	}
	return s.event(ctx, id, "charged", chargeID)
}

func (s *SQLStore) RecordRefund(ctx context.Context, id int64) error {
	return s.event(ctx, id, "refunded", "")
}

func (s *SQLStore) event(ctx context.Context, orderID int64, kind, chargeID string) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO payment_events (order_id, kind, payload) VALUES ($1, $2, $3)`,
		orderID, kind, fmt.Sprintf(`{"charge_id":%q}`, chargeID))
	return err
}

func found(o *Order, err error) (*Order, error) {
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return o, nil
}
