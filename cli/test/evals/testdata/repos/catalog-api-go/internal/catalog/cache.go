package catalog

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// Categories are the storefront's category slugs, most visited first.
var Categories = []string{
	"shoes", "jackets", "t-shirts", "jeans", "sneakers", "dresses", "hoodies", "bags", "accessories", "socks",
	"shirts", "shorts", "skirts", "sweaters", "boots", "hats", "scarves", "belts", "swimwear", "sportswear",
	"underwear", "sunglasses", "watches", "gloves",
}

// Cache keeps each category's full listing in memory. Each instance has
// its own; a restart starts it empty.
type Cache struct {
	store Store
	every time.Duration

	mu    sync.RWMutex
	items map[string][]Product
}

// NewCache returns an empty cache refreshed every `every`.
func NewCache(s Store, every time.Duration) *Cache {
	return &Cache{store: s, every: every, items: map[string][]Product{}}
}

// Get returns a category's listing, loading it on a miss.
func (c *Cache) Get(ctx context.Context, slug string) ([]Product, error) {
	c.mu.RLock()
	ps, ok := c.items[slug]
	c.mu.RUnlock()
	if ok {
		return ps, nil
	}
	ps, err := c.store.CategoryListing(ctx, slug)
	if err != nil {
		return nil, err
	}
	c.put(slug, ps)
	return ps, nil
}

func (c *Cache) put(slug string, ps []Product) {
	c.mu.Lock()
	c.items[slug] = ps
	c.mu.Unlock()
}

// Preload loads every category once, one after the other, waiting pause
// before each one.
func (c *Cache) Preload(ctx context.Context, slugs []string, pause time.Duration) {
	for _, slug := range slugs {
		if pause > 0 {
			select {
			case <-ctx.Done():
				return
			case <-time.After(pause):
			}
		}
		c.mu.RLock()
		_, ok := c.items[slug]
		c.mu.RUnlock()
		if ok {
			continue
		}
		ps, err := c.store.CategoryListing(ctx, slug)
		if err != nil {
			slog.Warn("preload category", "slug", slug, "err", err)
			continue
		}
		c.put(slug, ps)
	}
}

// RefreshLoop reloads every category on a timer, off the request path.
func (c *Cache) RefreshLoop(ctx context.Context, slugs []string) {
	t := time.NewTicker(c.every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			for _, slug := range slugs {
				if ps, err := c.store.CategoryListing(ctx, slug); err == nil {
					c.put(slug, ps)
				}
			}
		}
	}
}
