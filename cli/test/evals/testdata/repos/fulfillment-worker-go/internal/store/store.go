// Package store reads orders and records labels in PostgreSQL.
package store

import (
	"context"
	"database/sql"

	"example.com/fulfillment/internal/labels"
)

// Orders loads orders with their packages.
type Orders struct{ db *sql.DB }

// NewOrders returns an Orders store.
func NewOrders(db *sql.DB) *Orders { return &Orders{db: db} }

// Get loads one order and its packages.
func (o *Orders) Get(ctx context.Context, id string) (*labels.Order, error) {
	ord := &labels.Order{ID: id}
	err := o.db.QueryRowContext(ctx, `SELECT ship_name, ship_line1, ship_city, ship_postal_code, ship_country FROM orders WHERE id = $1`, id).
		Scan(&ord.ShipTo.Name, &ord.ShipTo.Line1, &ord.ShipTo.City, &ord.ShipTo.PostalCode, &ord.ShipTo.Country)
	if err != nil {
		return nil, err
	}
	rows, err := o.db.QueryContext(ctx, `SELECT weight_grams, service FROM packages WHERE order_id = $1 ORDER BY position`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var p labels.Package
		if err := rows.Scan(&p.WeightGrams, &p.Service); err != nil {
			return nil, err
		}
		ord.Packages = append(ord.Packages, p)
	}
	return ord, rows.Err()
}

// Labels records bought labels.
type Labels struct{ db *sql.DB }

// NewLabels returns a Labels store.
func NewLabels(db *sql.DB) *Labels { return &Labels{db: db} }

// SaveLabel stores the label of an order.
func (l *Labels) SaveLabel(ctx context.Context, orderID string, lb *labels.Label) error {
	_, err := l.db.ExecContext(ctx, `INSERT INTO labels (order_id, tracking_number, label_url, created_at) VALUES ($1, $2, $3, now())`, orderID, lb.TrackingNumber, lb.URL)
	return err
}
