package services

import (
	"errors"
	"fmt"
	"strings"

	"github.com/tracewayapp/traceway/backend/app/storage"
	"github.com/tracewayapp/traceway/backend/app/symbolicator/dart"
	"github.com/tracewayapp/traceway/backend/app/symbolicator/ios"
	"github.com/tracewayapp/traceway/backend/app/symbolicator/sourcemap"
	"github.com/tracewayapp/traceway/backend/app/symbolicator/twcache"

	traceway "go.tracewayapp.com"
)

func validArtifact(b []byte) bool {
	return sourcemap.ValidTW(b) || dart.ValidFlat(b) || ios.ValidFlat(b)
}

var sharedCache = newSymbolicatorCache()

func isStorageNotFound(err error) bool { return errors.Is(err, storage.ErrNotFound) }

func newSymbolicatorCache() *twcache.Cache {
	c := twcache.NewMem(200, 500<<20)
	c.NotFound = isStorageNotFound
	c.Validate = validArtifact
	c.SetWarn(func(err error) { traceway.CaptureException(err) })
	return c
}

func EnableSymbolicatorDiskCache(dir string, maxBytes int64) error {
	c, err := twcache.NewDisk(dir, maxBytes, func(err error) { traceway.CaptureException(err) })
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
