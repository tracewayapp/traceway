package main

import (
	"context"
	"time"
)

// redisCache adapts go-redis (with redisotel tracing) to home.Cache in
// production builds; this stand-in keeps the module dependency-free.
type redisCache struct{ url string }

func newRedisCache(url string) *redisCache { return &redisCache{url: url} }

func (c *redisCache) Get(ctx context.Context, key string) ([]byte, bool, error) { return nil, false, nil }

func (c *redisCache) Set(ctx context.Context, key string, val []byte, ttl time.Duration) error {
	return nil
}
