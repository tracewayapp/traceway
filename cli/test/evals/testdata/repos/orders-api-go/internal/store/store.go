// Package store reads orders from PostgreSQL.
package store

import (
	"context"
	"database/sql"
	"time"
)

// Order is one customer order.
type Order struct {
	ID         int64     `json:"id"`
	UserID     int64     `json:"userId"`
	Status     string    `json:"status"`
	TotalCents int64     `json:"totalCents"`
	CreatedAt  time.Time `json:"createdAt"`
	Items      []Item    `json:"items"`
}

// Item is one order line.
type Item struct {
	ID        int64  `json:"id"`
	SKU       string `json:"sku"`
	Qty       int    `json:"qty"`
	UnitPrice int64  `json:"unitPrice"`
}

// Store is the data access layer.
type Store interface {
	OrdersByUser(ctx context.Context, userID int64) ([]Order, error)
	OrderByID(ctx context.Context, id int64) (*Order, error)
	ItemsByOrder(ctx context.Context, orderID int64) ([]Item, error)
}

// SQL implements Store on database/sql.
type SQL struct{ db *sql.DB }

// New returns a Store backed by db.
func New(db *sql.DB) *SQL { return &SQL{db: db} }

// OrdersByUser returns a user's latest 100 orders.
func (s *SQL) OrdersByUser(ctx context.Context, userID int64) ([]Order, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT * FROM orders WHERE user_id = $1 ORDER BY created_at DESC LIMIT 100`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Order
	for rows.Next() {
		var o Order
		var shippingAddress, notes sql.NullString
		if err := rows.Scan(&o.ID, &o.UserID, &o.Status, &o.TotalCents, &shippingAddress, &notes, &o.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// OrderByID returns one order.
func (s *SQL) OrderByID(ctx context.Context, id int64) (*Order, error) {
	var o Order
	err := s.db.QueryRowContext(ctx, `SELECT id, user_id, status, total_cents, created_at FROM orders WHERE id = $1`, id).
		Scan(&o.ID, &o.UserID, &o.Status, &o.TotalCents, &o.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &o, err
}

// ItemsByOrder returns one order's line items.
func (s *SQL) ItemsByOrder(ctx context.Context, orderID int64) ([]Item, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, sku, qty, unit_price FROM order_items WHERE order_id = $1`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Item
	for rows.Next() {
		var it Item
		if err := rows.Scan(&it.ID, &it.SKU, &it.Qty, &it.UnitPrice); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}
