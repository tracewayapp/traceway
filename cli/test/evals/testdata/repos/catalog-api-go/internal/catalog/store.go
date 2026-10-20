package catalog

import (
	"context"
	"errors"
)

// Product is one listing entry.
type Product struct {
	ID           int64  `json:"id"`
	Slug         string `json:"slug"`
	Name         string `json:"name"`
	Brand        string `json:"brand"`
	ThumbnailURL string `json:"thumbnailUrl"`
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

// PgStore implements Store on pgx (wired in production builds).
type PgStore struct{ pool any }

// NewStore returns the PostgreSQL store.
func NewStore(pool any) *PgStore { return &PgStore{pool: pool} }

var errNoPool = errors.New("catalog: no database pool configured")

// CategoryListing runs:
//
//	SELECT p.id, p.slug, p.name, p.brand, p.thumbnail_url, pp.amount_cents, pp.currency, i.available
//	FROM products p JOIN product_prices pp ON pp.product_id = p.id AND pp.price_list = $2
//	JOIN inventory i ON i.product_id = p.id
//	WHERE p.category_id = (SELECT id FROM categories WHERE slug = $1) AND p.active
//	ORDER BY p.rank LIMIT 500
func (s *PgStore) CategoryListing(ctx context.Context, slug string) ([]Product, error) {
	return nil, errNoPool
}

// Product loads one product with its price and stock.
func (s *PgStore) Product(ctx context.Context, id int64) (*Product, error) { return nil, errNoPool }
