package catalog

import (
	"context"
	"database/sql"
	"errors"
)

// Product is one listing entry.
type Product struct {
	ID           int64  `json:"id"`
	Slug         string `json:"slug"`
	Name         string `json:"name"`
	Brand        string `json:"brand"`
	ThumbnailURL string `json:"thumbnailUrl,omitempty"`
	Description  string `json:"description,omitempty"`
	AmountCents  int64  `json:"amountCents"`
	Currency     string `json:"currency"`
	Available    int    `json:"available"`
	Badge        string `json:"badge,omitempty"`
}

// Store reads the catalog.
type Store interface {
	// CategoryListing runs the listing query: every active product of the
	// category with its price and stock, in rank order (up to 500).
	CategoryListing(ctx context.Context, slug string) ([]Product, error)
	Product(ctx context.Context, id int64) (*Product, error)
}

// SQLStore implements Store on PostgreSQL.
type SQLStore struct {
	db        *sql.DB
	priceList string
}

// NewStore returns the PostgreSQL store, pricing from priceList.
func NewStore(db *sql.DB, priceList string) *SQLStore { return &SQLStore{db: db, priceList: priceList} }

// CategoryListing runs the listing query.
func (s *SQLStore) CategoryListing(ctx context.Context, slug string) ([]Product, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT p.id, p.slug, p.name, p.brand, p.thumbnail_url, pp.amount_cents, pp.currency, i.available FROM products p JOIN product_prices pp ON pp.product_id = p.id AND pp.price_list = $2 JOIN inventory i ON i.product_id = p.id WHERE p.category_id = (SELECT id FROM categories WHERE slug = $1) AND p.active ORDER BY p.rank LIMIT 500`,
		slug, s.priceList)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Product
	for rows.Next() {
		var p Product
		if err := rows.Scan(&p.ID, &p.Slug, &p.Name, &p.Brand, &p.ThumbnailURL, &p.AmountCents, &p.Currency, &p.Available); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// Product loads one product with its price and stock.
func (s *SQLStore) Product(ctx context.Context, id int64) (*Product, error) {
	var p Product
	err := s.db.QueryRowContext(ctx, `SELECT p.id, p.slug, p.name, p.brand, p.description, pp.amount_cents, pp.currency, i.available FROM products p JOIN product_prices pp ON pp.product_id = p.id AND pp.price_list = $2 JOIN inventory i ON i.product_id = p.id WHERE p.id = $1`,
		id, s.priceList).Scan(&p.ID, &p.Slug, &p.Name, &p.Brand, &p.Description, &p.AmountCents, &p.Currency, &p.Available)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}
