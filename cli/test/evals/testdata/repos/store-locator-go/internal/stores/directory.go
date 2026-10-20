package stores

import (
	"context"
	"database/sql"

	"example.com/store-locator/internal/maps"
)

// OpeningHours is one weekday's opening time, "09:00" to "20:00".
type OpeningHours struct {
	Weekday  int    `json:"weekday"`
	OpensAt  string `json:"opensAt"`
	ClosesAt string `json:"closesAt"`
}

// SQLDirectory implements Directory on PostgreSQL.
type SQLDirectory struct{ db *sql.DB }

// NewDirectory returns a Directory backed by db.
func NewDirectory(db *sql.DB) *SQLDirectory { return &SQLDirectory{db: db} }

const searchRadiusMeters = 50000

// Near returns the ten stores closest to p within searchRadiusMeters.
func (d *SQLDirectory) Near(ctx context.Context, p maps.Point) ([]Store, error) {
	rows, err := d.db.QueryContext(ctx, `SELECT id, name, address, lat, lng FROM stores WHERE ST_DWithin(location, ST_MakePoint($1, $2)::geography, $3) ORDER BY location <-> ST_MakePoint($1, $2)::geography LIMIT 10`,
		p.Lng, p.Lat, searchRadiusMeters)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Store
	for rows.Next() {
		var s Store
		if err := rows.Scan(&s.ID, &s.Name, &s.Address, &s.Lat, &s.Lng); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// Hours returns a store's opening hours, one row per weekday.
func (d *SQLDirectory) Hours(ctx context.Context, storeID int) ([]OpeningHours, error) {
	rows, err := d.db.QueryContext(ctx, `SELECT weekday, opens_at, closes_at FROM store_hours WHERE store_id = $1`, storeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []OpeningHours
	for rows.Next() {
		var h OpeningHours
		if err := rows.Scan(&h.Weekday, &h.OpensAt, &h.ClosesAt); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}
