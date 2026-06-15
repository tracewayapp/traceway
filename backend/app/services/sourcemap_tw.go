package services

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/tracewayapp/traceway/backend/app/storage"

	"github.com/google/uuid"
	traceway "go.tracewayapp.com"
)

func GenerateTWArtifacts(ctx context.Context, projectId uuid.UUID, fileNames []string) {
	ctx = context.WithoutCancel(ctx)

	bases := make(map[string]bool)
	for _, name := range fileNames {
		bases[strings.TrimSuffix(name, ".map")] = true
	}

	for base := range bases {
		mapKey := SourceMapStorageKey(projectId, base+".map")
		bundleKey := SourceMapStorageKey(projectId, base)
		twKey := twKeyFor(mapKey)

		if err := storage.Store.Delete(ctx, twKey); err != nil {
			traceway.CaptureException(fmt.Errorf("tw generation: failed to delete stale tw artifact (key=%s): %w", twKey, err))
		}
		InvalidateSourceMap(projectId, base)

		_, done, err := sharedCache.Get(ctx, twKey, loadSourceMapBlob(mapKey, bundleKey))
		if err != nil && !errors.Is(err, storage.ErrNotFound) {
			traceway.CaptureException(fmt.Errorf("tw generation: failed to warm resolver (key=%s): %w", mapKey, err))
		}
		done()
	}
}
