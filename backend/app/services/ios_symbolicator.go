package services

import (
	"context"
	"fmt"
	"strings"

	"github.com/tracewayapp/traceway/backend/app/storage"
	"github.com/tracewayapp/traceway/backend/app/symbolicator/ios"
	"github.com/tracewayapp/traceway/backend/app/symbolicator/twcache"

	"github.com/google/uuid"
	traceway "go.tracewayapp.com"
)

func IOSSymbolsKey(projectId uuid.UUID, debugId string) string {
	return fmt.Sprintf("iossymbols/%s/%s.dsym", projectId, ios.NormalizeUUID(debugId))
}

func iosFlatKey(symbolsKey string) string {
	return strings.TrimSuffix(symbolsKey, ".dsym") + ".tw"
}

func ResolveIOSStackTrace(ctx context.Context, projectId uuid.UUID, rawTrace string) string {
	if !ios.IsIOSTrace(rawTrace) {
		return rawTrace
	}
	trace := ios.ParseTrace(rawTrace)
	if len(trace.Frames) == 0 {
		return rawTrace
	}
	arch := trace.Arch
	if arch == "" {
		arch = "arm64"
	}

	local := map[string]borrow{}
	defer releaseBorrows(local)

	lookup := func(debugID string, off uint64) []ios.SymFrame {
		if debugID == "" {
			return nil
		}
		symbolsKey := IOSSymbolsKey(projectId, debugID)
		cacheKey := iosFlatKey(symbolsKey)
		if sharedCache.IsNegative(cacheKey) {
			return nil
		}
		data := getBlob(ctx, cacheKey, loadIOSBlob(cacheKey, symbolsKey, debugID, arch), local)
		if data == nil {
			return nil
		}
		return ios.LookupFlat(data, off)
	}

	out := ios.RenderResolved(trace, iosErrorPreamble(rawTrace), lookup)
	if out == "" {
		return rawTrace
	}
	return out
}

func iosErrorPreamble(raw string) string {
	lines := strings.Split(raw, "\n")
	cut := len(lines)
	for i, ln := range lines {
		if strings.HasPrefix(strings.TrimSpace(ln), "***") {
			cut = i
			break
		}
	}
	return strings.TrimRight(strings.Join(lines[:cut], "\n"), "\n")
}

func loadIOSBlob(cacheKey, symbolsKey, debugID, arch string) twcache.LoadFunc {
	return func(ctx context.Context) ([]byte, error) {
		base := context.WithoutCancel(ctx)

		if tw, err := readWithTimeout(base, cacheKey); err == nil {
			if ios.ValidFlat(tw) {
				return tw, nil
			}
		} else if !isStorageNotFound(err) {
			traceway.CaptureException(fmt.Errorf("failed to read ios flat artifact, rebuilding (key=%s): %w", cacheKey, err))
		}

		dsym, err := readWithTimeout(base, symbolsKey)
		if err != nil {
			return nil, err
		}
		blob, buildErr := ios.BuildFlat(dsym, debugID, arch)
		if buildErr != nil {
			return nil, fmt.Errorf("failed to build ios flat artifact (key=%s): %w", symbolsKey, buildErr)
		}
		if werr := storage.Store.Write(base, cacheKey, blob); werr != nil {
			traceway.CaptureException(fmt.Errorf("failed to persist ios flat artifact (key=%s): %w", cacheKey, werr))
		}
		return blob, nil
	}
}

func InvalidateIOSSymbols(keys ...string) {
	for _, k := range keys {
		cacheKey := iosFlatKey(k)
		sharedCache.Invalidate(cacheKey)
		_ = storage.Store.Delete(context.Background(), cacheKey)
	}
}
