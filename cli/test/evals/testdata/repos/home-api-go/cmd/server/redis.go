//go:build !otel

package main

import (
	"context"
	"time"
)

// redisCache is the local stand-in for the Redis cache (redis_otel.go in
// production builds); it never holds anything, so every lookup misses.
type redisCache struct{ url string }

func newRedisCache(url string) *redisCache { return &redisCache{url: url} }

func (c *redisCache) Get(ctx context.Context, key string) ([]byte, bool, error) {
	return nil, false, nil
}

func (c *redisCache) Set(ctx context.Context, key string, val []byte, ttl time.Duration) error {
	return nil
}
