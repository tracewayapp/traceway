// Package home assembles and caches the storefront home feed.
package home

import (
	"context"
	"encoding/json"
	"time"
)

// FeedKey is the Redis key of the cached home feed.
const FeedKey = "home:feed:v3"

// FeedTTL is how long a built feed is served from the cache.
const FeedTTL = 10 * time.Minute

// Cache is the subset of Redis the service uses. Get reports ok=false on a
// miss.
type Cache interface {
	Get(ctx context.Context, key string) (val []byte, ok bool, err error)
	Set(ctx context.Context, key string, val []byte, ttl time.Duration) error
}

// Builder assembles the feed from the database.
type Builder interface {
	Build(ctx context.Context) (*Feed, error)
}

// Service returns the home feed.
type Service struct {
	Cache   Cache
	Builder Builder
}

// Feed returns the cached feed, rebuilding and caching it on a miss.
func (s *Service) Feed(ctx context.Context) (*Feed, error) {
	if raw, ok, err := s.Cache.Get(ctx, FeedKey); err == nil && ok {
		var f Feed
		if err := json.Unmarshal(raw, &f); err == nil {
			return &f, nil
		}
	}
	f, err := s.Builder.Build(ctx)
	if err != nil {
		return nil, err
	}
	if raw, err := json.Marshal(f); err == nil {
		_ = s.Cache.Set(ctx, FeedKey, raw, FeedTTL)
	}
	return f, nil
}
