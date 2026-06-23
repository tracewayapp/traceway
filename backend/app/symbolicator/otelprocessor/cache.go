package otelprocessor

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"

	"github.com/tracewayapp/traceway/backend/app/symbolicator/android"
	"github.com/tracewayapp/traceway/backend/app/symbolicator/dart"
	"github.com/tracewayapp/traceway/backend/app/symbolicator/ios"
	"github.com/tracewayapp/traceway/backend/app/symbolicator/sourcemap"
	"github.com/tracewayapp/traceway/backend/app/symbolicator/twcache"
)

type cacheEntry = any

func validArtifact(v cacheEntry) bool {
	switch t := v.(type) {
	case []byte:
		return sourcemap.ValidTW(t) || dart.ValidFlat(t) || ios.ValidFlat(t) || android.ValidR8Flat(t)
	case *android.Mapping:
		return t != nil
	default:
		return false
	}
}

func weighEntry(v cacheEntry) int64 {
	switch t := v.(type) {
	case []byte:
		return int64(len(t))
	case *android.Mapping:
		return t.ApproxSize()
	default:
		return 0
	}
}

func isObjectNotFound(err error) bool { return errors.Is(err, errObjectNotFound) }

func newCache(cfg *Config) (*twcache.Cache[cacheEntry], error) {
	if cfg.CacheDir == "" {
		c := twcache.NewMemFunc(cfg.SourceMapCacheSize, 1<<62, weighEntry)
		c.Validate = validArtifact
		c.NotFound = isObjectNotFound
		return c, nil
	}
	if err := os.MkdirAll(cfg.CacheDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create cache_dir: %w", err)
	}
	maxBytes := int64(cfg.CacheMaxMB) << 20
	if cfg.CacheMaxDiskPct > 0 {
		total, err := twcache.DiskCapacityBytes(cfg.CacheDir)
		if err != nil {
			return nil, fmt.Errorf("cache_max_disk_pct requires disk capacity detection: %w", err)
		}
		pctBytes := total / 100 * int64(cfg.CacheMaxDiskPct)
		if maxBytes <= 0 || pctBytes < maxBytes {
			maxBytes = pctBytes
		}
	}
	if maxBytes <= 0 {
		return nil, fmt.Errorf("the source map cache requires a positive byte cap (cache_max_mb or cache_max_disk_pct)")
	}
	c, err := twcache.NewDiskAny(cfg.CacheDir, maxBytes, nil)
	if err != nil {
		return nil, err
	}
	c.Validate = validArtifact
	c.NotFound = isObjectNotFound
	return c, nil
}

func cacheKey(url, buildUUID string) string {
	sum := sha256.Sum256([]byte(url + "|" + buildUUID))
	return hex.EncodeToString(sum[:]) + ".tw"
}

func flatCacheKey(symbolsKey string) string {
	sum := sha256.Sum256([]byte(symbolsKey))
	return hex.EncodeToString(sum[:]) + ".tw"
}
