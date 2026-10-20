// Package listings serves marketplace listings.
package listings

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// Listing is one marketplace listing.
type Listing struct {
	ID           int64     `json:"id"`
	SellerID     int64     `json:"sellerId"`
	Title        string    `json:"title"`
	Description  string    `json:"description,omitempty"`
	PriceCents   int64     `json:"priceCents"`
	Currency     string    `json:"currency"`
	ThumbnailURL string    `json:"thumbnailUrl,omitempty"`
	Status       string    `json:"status,omitempty"`
	RankedAt     time.Time `json:"rankedAt"`
	Images       []Image   `json:"images,omitempty"`
}

// Image is a listing photo.
type Image struct {
	URL      string `json:"url"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	Position int    `json:"position"`
}

// Repo reads listings from PostgreSQL with a Redis read-through cache.
type Repo struct {
	db       *pgxpool.Pool
	cache    *redis.Client
	log      *slog.Logger
	pageSize int
	cacheTTL time.Duration
}

// NewRepo returns a Repo.
func NewRepo(db *pgxpool.Pool, cache *redis.Client, log *slog.Logger, pageSize int, cacheTTL time.Duration) *Repo {
	return &Repo{db: db, cache: cache, log: log, pageSize: pageSize, cacheTTL: cacheTTL}
}

// Search returns the active listings of a category, best ranked first.
func (r *Repo) Search(ctx context.Context, category int64, cursor string) ([]Listing, error) {
	key := fmt.Sprintf("listings:search:%d:%x", category, cursor)
	r.log.DebugContext(ctx, "cache lookup", "cache.key", key)
	if raw, err := r.cache.Get(ctx, key).Bytes(); err == nil {
		var out []Listing
		if json.Unmarshal(raw, &out) == nil {
			return out, nil
		}
	}
	rows, err := r.db.Query(ctx, `SELECT id, seller_id, title, price_cents, currency, thumbnail_url, ranked_at FROM listings WHERE category_id = $1 AND status = 'active' ORDER BY ranked_at DESC LIMIT `+fmt.Sprint(r.pageSize), category)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Listing
	for rows.Next() {
		var l Listing
		if err := rows.Scan(&l.ID, &l.SellerID, &l.Title, &l.PriceCents, &l.Currency, &l.ThumbnailURL, &l.RankedAt); err != nil {
			return nil, err
		}
		r.log.DebugContext(ctx, "decoded listing", "listing.id", l.ID, "seller.id", l.SellerID)
		out = append(out, l)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	r.log.DebugContext(ctx, "query executed", "db.table", "listings", "rows", len(out))
	if raw, err := json.Marshal(out); err == nil {
		_ = r.cache.Set(ctx, key, raw, r.cacheTTL).Err()
	}
	return out, nil
}

// Get returns one listing with its images.
func (r *Repo) Get(ctx context.Context, id int64) (*Listing, error) {
	key := fmt.Sprintf("listing:%d", id)
	r.log.DebugContext(ctx, "cache lookup", "cache.key", key)
	var l Listing
	if raw, err := r.cache.Get(ctx, key).Bytes(); err == nil && json.Unmarshal(raw, &l) == nil {
		return &l, nil
	}
	err := r.db.QueryRow(ctx, `SELECT id, seller_id, title, description, price_cents, currency, status, ranked_at FROM listings WHERE id = $1`, id).
		Scan(&l.ID, &l.SellerID, &l.Title, &l.Description, &l.PriceCents, &l.Currency, &l.Status, &l.RankedAt)
	if err != nil {
		return nil, err
	}
	r.log.DebugContext(ctx, "query executed", "db.table", "listings", "rows", 1)
	rows, err := r.db.Query(ctx, `SELECT url, width, height, position FROM listing_images WHERE listing_id = $1 ORDER BY position`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var img Image
		if err := rows.Scan(&img.URL, &img.Width, &img.Height, &img.Position); err != nil {
			return nil, err
		}
		l.Images = append(l.Images, img)
	}
	r.log.DebugContext(ctx, "query executed", "db.table", "listing_images", "rows", len(l.Images))
	r.log.DebugContext(ctx, "decoded listing", "listing.id", l.ID)
	for _, img := range l.Images {
		r.log.DebugContext(ctx, "image url resolved", "listing.id", l.ID, "position", img.Position)
	}
	return &l, rows.Err()
}

// Categories returns the category tree, cached for an hour.
func (r *Repo) Categories(ctx context.Context) (json.RawMessage, error) {
	r.log.DebugContext(ctx, "cache lookup", "cache.key", "categories:v3")
	raw, err := r.cache.Get(ctx, "categories:v3").Bytes()
	if err == nil {
		r.log.DebugContext(ctx, "cache hit", "cache.key", "categories:v3")
		return raw, nil
	}
	var tree json.RawMessage
	if err := r.db.QueryRow(ctx, `SELECT tree FROM category_trees ORDER BY version DESC LIMIT 1`).Scan(&tree); err != nil {
		return nil, err
	}
	_ = r.cache.Set(ctx, "categories:v3", []byte(tree), time.Hour).Err()
	return tree, nil
}

// RecordView counts a listing view.
func (r *Repo) RecordView(ctx context.Context, id, viewer int64) error {
	r.log.DebugContext(ctx, "recording view")
	_, err := r.db.Exec(ctx, `INSERT INTO listing_views (listing_id, viewer_id, viewed_at) VALUES ($1, $2, now())`, id, viewer)
	r.log.DebugContext(ctx, "query executed", "db.table", "listing_views", "rows", 1)
	return err
}
