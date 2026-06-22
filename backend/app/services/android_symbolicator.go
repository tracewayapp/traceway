package services

import (
	"context"
	"fmt"
	"strings"

	"github.com/tracewayapp/traceway/backend/app/storage"
	"github.com/tracewayapp/traceway/backend/app/symbolicator/android"
	"github.com/tracewayapp/traceway/backend/app/symbolicator/twcache"

	"github.com/google/uuid"
	traceway "go.tracewayapp.com"
)

func loadAndroidMapping(mappingKey string) twcache.LoadFunc[SymbolicationCacheEntry] {
	return func(ctx context.Context) (SymbolicationCacheEntry, error) {
		raw, err := readWithTimeout(context.WithoutCancel(ctx), mappingKey)
		if err != nil {
			return nil, err
		}
		return android.ParseMapping(string(raw)), nil
	}
}

func getMapping(ctx context.Context, key string, load twcache.LoadFunc[SymbolicationCacheEntry]) (*android.Mapping, func()) {
	data, done, err := sharedCache.Get(ctx, key, load)
	if err != nil {
		return nil, noop
	}
	m, ok := data.(*android.Mapping)
	if !ok || m == nil {
		done()
		return nil, noop
	}
	return m, done
}

func AndroidMappingKey(projectId uuid.UUID, proguardUuid string) string {
	return fmt.Sprintf("androidmappings/%s/%s.txt", projectId, NormalizeProguardUUID(proguardUuid))
}

func androidMappingFlatKey(mappingKey string) string {
	return strings.TrimSuffix(mappingKey, ".txt") + ".tw"
}

func NormalizeProguardUUID(s string) string { return android.NormalizeProguardUUID(s) }

func ResolveAndroidStackTrace(ctx context.Context, projectId uuid.UUID, rawTrace, proguardUuid string) string {
	if !android.IsAndroidTrace(rawTrace) {
		return rawTrace
	}
	text := rawTrace

	local := map[string]borrow{}
	defer releaseBorrows(local)

	if proguardUuid != "" && android.IsR8Trace(text) {
		mappingKey := AndroidMappingKey(projectId, proguardUuid)
		if symbolicatorOnDisk() {
			cacheKey := androidMappingFlatKey(mappingKey)
			if !sharedCache.IsNegative(cacheKey) {
				if data := getBlob(ctx, cacheKey, loadAndroidMappingBlob(cacheKey, mappingKey), local); data != nil {
					text = android.RetraceFlat(data, text)
				}
			}
		} else if !sharedCache.IsNegative(mappingKey) {
			if m, done := getMapping(ctx, mappingKey, loadAndroidMapping(mappingKey)); m != nil {
				text = m.Retrace(text)
				done()
			}
		}
	}

	out := strings.TrimRight(text, "\n")
	if out == "" || out == strings.TrimRight(rawTrace, "\n") {
		return rawTrace
	}
	return out
}

func loadAndroidMappingBlob(cacheKey, mappingKey string) twcache.LoadFunc[SymbolicationCacheEntry] {
	return func(ctx context.Context) (SymbolicationCacheEntry, error) {
		base := context.WithoutCancel(ctx)

		if tw, err := readWithTimeout(base, cacheKey); err == nil {
			if android.ValidR8Flat(tw) {
				return tw, nil
			}
		} else if !isStorageNotFound(err) {
			traceway.CaptureException(fmt.Errorf("failed to read android mapping artifact, rebuilding (key=%s): %w", cacheKey, err))
		}

		raw, err := readWithTimeout(base, mappingKey)
		if err != nil {
			return nil, err
		}
		blob := android.BuildR8Flat(string(raw))
		if werr := storage.Store.Write(base, cacheKey, blob); werr != nil {
			traceway.CaptureException(fmt.Errorf("failed to persist android mapping artifact (key=%s): %w", cacheKey, werr))
		}
		return blob, nil
	}
}

func InvalidateAndroidMapping(mappingKey string) {
	flatKey := androidMappingFlatKey(mappingKey)
	sharedCache.Invalidate(flatKey)
	_ = storage.Store.Delete(context.Background(), flatKey)
	sharedCache.Invalidate(mappingKey)
}
