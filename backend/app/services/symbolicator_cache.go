package services

import (
	"errors"
	"fmt"
	"strings"

	"github.com/tracewayapp/traceway/backend/v2/app/storage"
	"github.com/tracewayapp/traceway/backend/v2/app/symbolicator/android"
	"github.com/tracewayapp/traceway/backend/v2/app/symbolicator/dart"
	"github.com/tracewayapp/traceway/backend/v2/app/symbolicator/ios"
	"github.com/tracewayapp/traceway/backend/v2/app/symbolicator/sourcemap"
	"github.com/tracewayapp/traceway/backend/v2/app/symbolicator/twcache"
	"github.com/tracewayapp/traceway/backend/v2/app/traceway"
)

type SymbolicationCacheEntry = any

func validArtifact(v SymbolicationCacheEntry) bool {
	switch t := v.(type) {
	case []byte:
		return sourcemap.ValidTW(t) || dart.ValidFlat(t) || ios.ValidFlat(t) || android.ValidR8Flat(t)
	case *android.Mapping:
		return t != nil
	default:
		return false
	}
}

func weighEntry(v SymbolicationCacheEntry) int64 {
	switch t := v.(type) {
	case []byte:
		return int64(len(t))
	case *android.Mapping:
		return t.ApproxSize()
	default:
		return 0
	}
}

var sharedCache = newSymbolicatorCache()

func symbolicatorOnDisk() bool { return sharedCache.Dir() != "" }

func isStorageNotFound(err error) bool { return errors.Is(err, storage.ErrNotFound) }

func newSymbolicatorCache() *twcache.Cache[SymbolicationCacheEntry] {
	c := twcache.NewMemFunc(200, 500<<20, weighEntry)
	c.NotFound = isStorageNotFound
	c.Validate = validArtifact
	c.SetWarn(func(err error) { traceway.CaptureException(err) })
	return c
}

func EnableSymbolicatorDiskCache(dir string, maxBytes int64) error {
	c, err := twcache.NewDiskAny(dir, maxBytes, func(err error) { traceway.CaptureException(err) })
	if err != nil {
		return fmt.Errorf("symbolicator disk cache: %w", err)
	}
	c.NotFound = isStorageNotFound
	c.Validate = validArtifact
	sharedCache = c
	return nil
}

func twKeyFor(mapKey string) string {
	return strings.TrimSuffix(mapKey, ".map") + ".tw"
}

func noop() {}
