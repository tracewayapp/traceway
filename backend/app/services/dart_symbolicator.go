package services

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/tracewayapp/traceway/backend/app/storage"
	"github.com/tracewayapp/traceway/backend/app/symbolicator/dart"
	"github.com/tracewayapp/traceway/backend/app/symbolicator/twcache"

	"github.com/google/uuid"
	traceway "go.tracewayapp.com"
)

const maxDartFrames = 50

func DartSymbolsKey(projectId uuid.UUID, debugId, arch string) string {
	return fmt.Sprintf("dartsymbols/%s/%s-%s.symbols", projectId, NormalizeDartDebugId(debugId), NormalizeDartArch(arch))
}

func NormalizeDartDebugId(debugId string) string { return dart.NormalizeDebugID(debugId) }
func NormalizeDartArch(arch string) string       { return dart.NormalizeArch(arch) }

func ResolveDartStackTrace(ctx context.Context, projectId uuid.UUID, rawTrace string) string {
	if !dart.IsNonSymbolic(rawTrace) {
		return rawTrace
	}
	trace := dart.ParseTrace(rawTrace)
	if len(trace.Frames) == 0 {
		return rawTrace
	}

	data, done := loadDartData(ctx, projectId, trace)
	defer done()

	var b strings.Builder
	if preamble := dartErrorPreamble(rawTrace); preamble != "" {
		b.WriteString(preamble)
		b.WriteByte('\n')
	}

	n := 0
	for _, f := range trace.Frames {
		if n >= maxDartFrames {
			break
		}
		var resolved []dart.SymFrame
		if data != nil {
			resolved = dart.LookupFlat(data, f)
		}
		if len(resolved) == 0 {

			fmt.Fprintf(&b, "#%d  %s+%x\n", n, dart.InstructionSymbol(f.Section), f.Offset)
			n++
			continue
		}
		for _, sf := range resolved {
			if n >= maxDartFrames {
				break
			}
			fmt.Fprintf(&b, "#%d  %s (%s)\n", n, sf.Function, sf.Location())
			n++
		}
	}

	out := strings.TrimRight(b.String(), "\n")
	if out == "" {
		return rawTrace
	}
	return out
}

func dartErrorPreamble(raw string) string {
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

func loadDartData(ctx context.Context, projectId uuid.UUID, trace dart.StackTrace) ([]byte, func()) {

	if trace.BuildID == "" || trace.Arch == "" {
		return nil, noop
	}
	cacheKey := dartFlatKey(DartSymbolsKey(projectId, trace.BuildID, trace.Arch))
	if sharedCache.IsNegative(cacheKey) {
		return nil, noop
	}
	data, done, err := sharedCache.Get(ctx, cacheKey, loadDartBlob(cacheKey))
	if err != nil {
		return nil, noop
	}
	return data, done
}

func dartFlatKey(symbolsKey string) string {
	return strings.TrimSuffix(symbolsKey, ".symbols") + ".tw"
}

func InvalidateDartSymbols(keys ...string) {
	for _, k := range keys {
		cacheKey := dartFlatKey(k)
		sharedCache.Invalidate(cacheKey)
		_ = storage.Store.Delete(context.Background(), cacheKey)
	}
}

func loadDartBlob(cacheKey string) twcache.LoadFunc {
	symbolsKey := strings.TrimSuffix(cacheKey, ".tw") + ".symbols"
	return func(ctx context.Context) ([]byte, error) {
		base := context.WithoutCancel(ctx)

		if twBytes, err := readWithTimeout(base, cacheKey); err == nil {
			if dart.ValidFlat(twBytes) {
				return twBytes, nil
			}

		} else if !errors.Is(err, storage.ErrNotFound) {
			traceway.CaptureException(fmt.Errorf("failed to read dart flat artifact, rebuilding (key=%s): %w", cacheKey, err))
		}

		elf, err := readWithTimeout(base, symbolsKey)
		if err != nil {
			return nil, err
		}
		blob, buildErr := dart.BuildFlat(elf)
		if buildErr != nil {
			return nil, fmt.Errorf("failed to build dart flat artifact (key=%s): %w", symbolsKey, buildErr)
		}
		if werr := storage.Store.Write(base, cacheKey, blob); werr != nil {
			traceway.CaptureException(fmt.Errorf("failed to persist dart flat artifact (key=%s): %w", cacheKey, werr))
		}
		return blob, nil
	}
}
