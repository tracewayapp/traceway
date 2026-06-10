package services

import (
	"context"
	"fmt"
	"strings"

	"github.com/tracewayapp/traceway/backend/app/symbolicator"
	"github.com/tracewayapp/traceway/backend/app/symbolicator/twcache"

	traceway "go.tracewayapp.com"
)

type sourceMapDiskCache struct {
	mem  *sourceMapCache
	disk *twcache.Cache
}

func EnableSourceMapDiskCache(dir string, maxBytes int64) error {
	disk, err := twcache.New(dir, maxBytes, func(err error) {
		traceway.CaptureException(err)
	})
	if err != nil {
		return fmt.Errorf("source map disk cache: %w", err)
	}
	activeSMCache = &sourceMapDiskCache{mem: smCache, disk: disk}
	return nil
}

func twKeyFor(mapKey string) string {
	return strings.TrimSuffix(mapKey, ".map") + ".tw"
}

func (d *sourceMapDiskCache) getOrBuild(ctx context.Context, key string, build resolverBuild) (*symbolicator.Resolver, error) {
	return d.mem.getOrBuild(ctx, key, func(ctx context.Context) (*symbolicator.Resolver, int64, error) {
		return d.load(ctx, key, build)
	})
}

func (d *sourceMapDiskCache) load(ctx context.Context, mapKey string, build resolverBuild) (*symbolicator.Resolver, int64, error) {
	name := twKeyFor(mapKey)
	if r, err := d.disk.Open(name); err == nil {
		return r, r.ApproxSize(), nil
	}

	resolver, size, err := build(ctx)
	if err != nil {
		return nil, 0, err
	}
	if r, werr := d.disk.Write(name, resolver.MarshalTW()); werr == nil {
		return r, r.ApproxSize(), nil
	} else {
		traceway.CaptureException(fmt.Errorf("failed to write tw cache file (key=%s): %w", mapKey, werr))
	}
	return resolver, size, nil
}

func (d *sourceMapDiskCache) isNegative(key string) bool {
	return d.mem.isNegative(key)
}

func (d *sourceMapDiskCache) invalidate(key string) {
	d.mem.invalidate(key)
	d.disk.Remove(twKeyFor(key))
}

func (d *sourceMapDiskCache) stats() SourceMapCacheStats {
	s := d.mem.stats()
	ds := d.disk.Stats()
	s.DiskEnabled = true
	s.DiskEntries = ds.Entries
	s.DiskBytes = ds.Bytes
	s.DiskMaxBytes = ds.MaxBytes
	s.DiskHits = ds.Hits
	s.DiskEvictions = ds.Evictions
	return s
}
