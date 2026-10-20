// Package rates prices parcels from the carrier rate tables.
package rates

import (
	"context"
	"database/sql"
	"fmt"
)

// Table reads rate tables from PostgreSQL.
type Table struct{ db *sql.DB }

// New returns a Table.
func New(db *sql.DB) *Table { return &Table{db: db} }

// Quote returns the price in cents of a parcel for a carrier and zone.
func (t *Table) Quote(ctx context.Context, carrier string, zone, weightGrams int) (int64, error) {
	var cents int64
	err := t.db.QueryRowContext(ctx, `SELECT price_cents FROM rate_tables WHERE carrier = $1 AND zone = $2 AND max_weight_grams >= $3 ORDER BY max_weight_grams LIMIT 1`,
		carrier, zone, weightGrams).Scan(&cents)
	if err == sql.ErrNoRows {
		return 0, fmt.Errorf("carrier %q has no rate table for zone %d", carrier, zone)
	}
	return cents, err
}
