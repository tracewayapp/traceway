package services

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/tracewayapp/traceway/backend/app/storage"
	"github.com/tracewayapp/traceway/backend/app/symbolicator/sourcemap"
	"github.com/tracewayapp/traceway/backend/app/symbolicator/twcache"

	"github.com/google/uuid"
	traceway "go.tracewayapp.com"
)

const sourceMapLoadTimeout = 5 * time.Second

func InitSourceMapCache(maxEntries int, maxBytes int64) {
	sharedCache.SetLimits(maxEntries, maxBytes)
}

func SourceMapStorageKey(projectId uuid.UUID, fileName string) string {
	return fmt.Sprintf("sourcemaps/%s/%s", projectId, fileName)
}

func InvalidateSourceMap(projectId uuid.UUID, fileName string) {
	name := fileName
	if !strings.HasPrefix(name, sourceMapDebugIdDir) {
		name = filepath.Base(name)
	}
	if !strings.HasSuffix(name, ".map") {
		name += ".map"
	}
	sharedCache.Invalidate(twKeyFor(SourceMapStorageKey(projectId, name)))
}

type SourceMapCacheStats struct {
	Entries         int
	Bytes           int64
	MaxEntries      int
	MaxBytes        int64
	Hits            uint64
	Misses          uint64
	Evictions       uint64
	Failures        uint64
	NotFound        uint64
	NegativeHits    uint64
	NegativeEntries int
	LastParseMs     float64

	DiskEnabled   bool
	DiskEntries   int
	DiskBytes     int64
	DiskMaxBytes  int64
	DiskHits      uint64
	StoreHits     uint64
	Builds        uint64
	DiskEvictions uint64
}

func SourceMapStats() SourceMapCacheStats {
	s := sharedCache.Stats()
	out := SourceMapCacheStats{
		Entries:         s.Entries,
		Bytes:           s.Bytes,
		MaxEntries:      s.MaxEntries,
		MaxBytes:        s.MaxBytes,
		Hits:            s.Hits,
		Misses:          s.Misses,
		Evictions:       s.Evictions,
		Failures:        s.Failures,
		NotFound:        s.NotFound,
		NegativeHits:    s.NegativeHits,
		NegativeEntries: s.NegativeEntries,
		LastParseMs:     s.LastParseMs,
		StoreHits:       smStoreHits.Load(),
		Builds:          smBuilds.Load(),
	}
	if s.Mode == "disk" {
		out.DiskEnabled = true
		out.DiskEntries = s.Entries
		out.DiskBytes = s.Bytes
		out.DiskMaxBytes = s.MaxBytes
		out.DiskHits = s.Hits
		out.DiskEvictions = s.Evictions
	}
	return out
}

var stackFrameRe = regexp.MustCompile(`^(\s{4})(.+):(\d+):(\d+)$`)

type borrow struct {
	data []byte
	done func()
}

func releaseBorrows(local map[string]borrow) {
	for _, br := range local {
		if br.done != nil {
			br.done()
		}
	}
}

func ResolveStackTrace(ctx context.Context, projectId uuid.UUID, stackTrace string, debugIds map[string]string) string {
	prefix := SourceMapStorageKey(projectId, "")

	lines := strings.Split(stackTrace, "\n")
	resolved := make([]string, 0, len(lines))
	framesResolved := 0
	maxFrames := 50

	local := map[string]borrow{}
	defer releaseBorrows(local)

	for _, line := range lines {
		if framesResolved >= maxFrames {
			resolved = append(resolved, line)
			continue
		}

		matches := stackFrameRe.FindStringSubmatch(line)
		if matches == nil {
			resolved = append(resolved, line)
			continue
		}

		indent := matches[1]
		fileName := matches[2]
		lineNum, _ := strconv.Atoi(matches[3])
		colNum, _ := strconv.Atoi(matches[4])

		clean := fileName
		if idx := strings.IndexAny(clean, "?#"); idx != -1 {
			clean = clean[:idx]
		}
		base := filepath.Base(clean)

		data := frameData(ctx, prefix, fileName, base, debugIds, local)
		if data == nil {
			resolved = append(resolved, line)
			continue
		}

		frame, ok := sourcemap.LookupTW(data, uint32(lineNum-1), uint32(colNum-1))
		if !ok {
			resolved = append(resolved, line)
			continue
		}

		file := frame.File
		if file == "" {
			file = "<unknown>"
		}

		resolved = append(resolved, fmt.Sprintf("%s%s:%d:%d", indent, file, frame.Line, frame.Col))
		framesResolved++

		if frame.Fn != "" && len(resolved) >= 2 {
			prev := resolved[len(resolved)-2]
			if strings.HasSuffix(strings.TrimSpace(prev), "()") {
				trimmed := strings.TrimSpace(prev)
				prevIndent := prev[:len(prev)-len(trimmed)]
				resolved[len(resolved)-2] = prevIndent + frame.Fn + "()"
			}
		}
	}

	return strings.Join(resolved, "\n")
}

func frameData(ctx context.Context, prefix, fileName, base string, debugIds map[string]string, local map[string]borrow) []byte {
	id := NormalizeDebugId(debugIds[fileName])
	if id == "" {
		id = NormalizeDebugId(debugIds[base])
	}
	if id != "" {
		mapKey := prefix + DebugIdMapName(id)
		twKey := twKeyFor(mapKey)
		if !sharedCache.IsNegative(twKey) {
			bundleKey := prefix + DebugIdBundleName(id)
			if data := getBlob(ctx, twKey, loadSourceMapBlob(mapKey, bundleKey), local); data != nil {
				return data
			}
		}
	}

	mapKey := prefix + base + ".map"
	twKey := twKeyFor(mapKey)
	if sharedCache.IsNegative(twKey) {
		return nil
	}
	return getBlob(ctx, twKey, loadSourceMapBlob(mapKey, prefix+base), local)
}

func getBlob(ctx context.Context, twKey string, load twcache.LoadFunc, local map[string]borrow) []byte {
	if br, ok := local[twKey]; ok {
		return br.data
	}
	data, done, err := sharedCache.Get(ctx, twKey, load)
	if err != nil {
		local[twKey] = borrow{}
		return nil
	}
	local[twKey] = borrow{data: data, done: done}
	return data
}

var smStoreHits, smBuilds atomic.Uint64

func loadSourceMapBlob(mapKey, bundleKey string) twcache.LoadFunc {
	return func(ctx context.Context) ([]byte, error) {
		base := context.WithoutCancel(ctx)
		twKey := twKeyFor(mapKey)

		refreshStoreTw := true
		twBytes, err := readWithTimeout(base, twKey)
		if err == nil {
			if sourcemap.ValidTW(twBytes) {
				smStoreHits.Add(1)
				return twBytes, nil
			}

		} else if !errors.Is(err, storage.ErrNotFound) {
			refreshStoreTw = false
			traceway.CaptureException(fmt.Errorf("failed to read tw artifact, rebuilding from source map (key=%s): %w", twKey, err))
		}

		mapBytes, err := readWithTimeout(base, mapKey)
		if err != nil {
			return nil, err
		}
		var bundleBytes []byte
		if b, readErr := readWithTimeout(base, bundleKey); readErr == nil {
			bundleBytes = b
		} else if !errors.Is(readErr, storage.ErrNotFound) {
			return nil, fmt.Errorf("failed to read bundle (key=%s): %w", bundleKey, readErr)
		}

		blob, err := sourcemap.BuildTW(mapBytes, bundleBytes)
		if err != nil {
			return nil, err
		}
		smBuilds.Add(1)
		if refreshStoreTw {
			if werr := storage.Store.Write(base, twKey, blob); werr != nil {
				traceway.CaptureException(fmt.Errorf("failed to refresh tw artifact in storage (key=%s): %w", twKey, werr))
			}
		}
		return blob, nil
	}
}

func readWithTimeout(ctx context.Context, key string) ([]byte, error) {
	readCtx, cancel := context.WithTimeout(ctx, sourceMapLoadTimeout)
	defer cancel()
	return storage.Store.Read(readCtx, key)
}
