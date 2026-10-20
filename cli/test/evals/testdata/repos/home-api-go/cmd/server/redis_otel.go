//go:build otel

package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"time"

	"github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"
)

// redisCache adapts go-redis, traced with redisotel, to home.Cache.
type redisCache struct{ c *redis.Client }

func newRedisCache(url string) *redisCache {
	opt, err := redis.ParseURL(url)
	if err != nil {
		slog.Error("redis url", "err", err)
		os.Exit(1)
	}
	c := redis.NewClient(opt)
	if err := redisotel.InstrumentTracing(c); err != nil {
		slog.Error("redis tracing", "err", err)
		os.Exit(1)
	}
	return &redisCache{c: c}
}

func (c *redisCache) Get(ctx context.Context, key string) ([]byte, bool, error) {
	b, err := c.c.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return b, true, nil
}

func (c *redisCache) Set(ctx context.Context, key string, val []byte, ttl time.Duration) error {
	return c.c.Set(ctx, key, val, ttl).Err()
}
