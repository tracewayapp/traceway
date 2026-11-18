package shipments

import (
	"context"
	"database/sql"
	"errors"
)

// SQLRepo stores shipments in PostgreSQL.
type SQLRepo struct{ db *sql.DB }

// NewSQLRepo returns a SQLRepo.
func NewSQLRepo(db *sql.DB) *SQLRepo { return &SQLRepo{db: db} }

// Insert stores a shipment and sets its id.
func (r *SQLRepo) Insert(ctx context.Context, s *Shipment) error {
	return r.db.QueryRowContext(ctx, `INSERT INTO shipments (order_id, carrier, price_cents, created_at) VALUES ($1, $2, $3, now()) RETURNING id`,
		s.OrderID, s.Carrier, s.PriceCents).Scan(&s.ID)
}

// Get returns the shipment with id, or nil when there is none.
func (r *SQLRepo) Get(ctx context.Context, id string) (*Shipment, error) {
	var s Shipment
	err := r.db.QueryRowContext(ctx, `SELECT id, order_id, carrier, price_cents FROM shipments WHERE id = $1`, id).
		Scan(&s.ID, &s.OrderID, &s.Carrier, &s.PriceCents)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}
