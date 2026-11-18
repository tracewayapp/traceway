package home

import (
	"context"
	"database/sql"
)

// Feed is the home page document.
type Feed struct {
	Banners     []Row `json:"banners"`
	Collections []Row `json:"collections"`
	TopSellers  []Row `json:"topSellers"`
	NewArrivals []Row `json:"newArrivals"`
	Editorial   []Row `json:"editorial"`
	Trending    []Row `json:"trending"`
	Categories  []Row `json:"categories"`
	Deals       []Row `json:"deals"`
	Brands      []Row `json:"brands"`
	TopReviewed []Row `json:"topReviewed"`
	Stories     []Row `json:"stories"`
	ForYou      []Row `json:"forYou"`
}

// Row is one result row, column name to value.
type Row = map[string]any

// sections are the home queries, in page order.
var sections = []struct {
	field func(*Feed) *[]Row
	query string
}{
	{func(f *Feed) *[]Row { return &f.Banners }, `SELECT id, image_url, target_url, starts_at, ends_at FROM banners WHERE placement = 'home' AND now() BETWEEN starts_at AND ends_at ORDER BY priority`},
	{func(f *Feed) *[]Row { return &f.Collections }, `SELECT c.id, c.title, c.slug, c.hero_image FROM collections c WHERE c.featured ORDER BY c.position LIMIT 6`},
	{func(f *Feed) *[]Row { return &f.TopSellers }, `SELECT ol.sku, SUM(ol.qty) AS sold FROM order_lines ol JOIN orders o ON o.id = ol.order_id WHERE o.placed_at > now() - interval '7 days' GROUP BY ol.sku ORDER BY sold DESC LIMIT 24`},
	{func(f *Feed) *[]Row { return &f.NewArrivals }, `SELECT id, sku, title, price_minor, image_url FROM products WHERE published_at > now() - interval '14 days' ORDER BY published_at DESC LIMIT 24`},
	{func(f *Feed) *[]Row { return &f.Editorial }, `SELECT p.id, p.sku, p.title, p.price_minor, p.image_url FROM editorial_picks e JOIN products p ON p.id = e.product_id WHERE e.week = date_trunc('week', now()) ORDER BY e.position`},
	{func(f *Feed) *[]Row { return &f.Trending }, `SELECT term, count(*) AS n FROM search_events WHERE created_at > now() - interval '1 day' GROUP BY term ORDER BY n DESC LIMIT 10`},
	{func(f *Feed) *[]Row { return &f.Categories }, `SELECT id, slug, title, tile_image FROM categories WHERE parent_id IS NULL ORDER BY position`},
	{func(f *Feed) *[]Row { return &f.Deals }, `SELECT d.id, d.product_id, d.discount_pct, d.ends_at FROM deals d WHERE d.active AND d.ends_at > now() ORDER BY d.discount_pct DESC LIMIT 12`},
	{func(f *Feed) *[]Row { return &f.Brands }, `SELECT b.id, b.name, b.logo_url FROM brands b WHERE b.spotlight_until > now() LIMIT 4`},
	{func(f *Feed) *[]Row { return &f.TopReviewed }, `SELECT r.product_id, avg(r.rating) AS rating, count(*) AS n FROM reviews r WHERE r.created_at > now() - interval '3 days' GROUP BY r.product_id ORDER BY n DESC LIMIT 12`},
	{func(f *Feed) *[]Row { return &f.Stories }, `SELECT id, title, image_url FROM seasonal_stories WHERE season = $1 ORDER BY position LIMIT 3`},
	{func(f *Feed) *[]Row { return &f.ForYou }, `SELECT sku FROM product_scores WHERE segment = 'default' ORDER BY score DESC LIMIT 24`},
}

// SQLBuilder builds the feed with one query per section.
type SQLBuilder struct {
	DB     *sql.DB
	Season string
}

// Build runs the section queries in page order.
func (b *SQLBuilder) Build(ctx context.Context) (*Feed, error) {
	f := &Feed{}
	for _, s := range sections {
		var args []any
		if s.field(f) == &f.Stories {
			args = append(args, b.Season)
		}
		rows, err := b.DB.QueryContext(ctx, s.query, args...)
		if err != nil {
			return nil, err
		}
		items, err := scanMaps(rows)
		if err != nil {
			return nil, err
		}
		*s.field(f) = items
	}
	return f, nil
}

func scanMaps(rows *sql.Rows) ([]Row, error) {
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	var out []Row
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		m := Row{}
		for i, c := range cols {
			m[c] = vals[i]
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
