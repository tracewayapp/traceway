package otelprocessor

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"

	"github.com/tracewayapp/traceway/backend/app/symbolicator/dart"
	"github.com/tracewayapp/traceway/backend/app/symbolicator/sourcemap"
	"github.com/tracewayapp/traceway/backend/app/symbolicator/twcache"
)

func validArtifact(b []byte) bool {
	return sourcemap.ValidTW(b) || dart.ValidFlat(b)
}

func isObjectNotFound(err error) bool { return errors.Is(err, errObjectNotFound) }

func newCache(cfg *Config) (*twcache.Cache, error) {
	if cfg.CacheDir == "" {
		c := twcache.NewMem(cfg.SourceMapCacheSize, 1<<62)
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
	c, err := twcache.NewDisk(cfg.CacheDir, maxBytes, nil)
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

func dartCacheKey(symbolsKey string) string {
	sum := sha256.Sum256([]byte(symbolsKey))
	return hex.EncodeToString(sum[:]) + ".tw"
}
