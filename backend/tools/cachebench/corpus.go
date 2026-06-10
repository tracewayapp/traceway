package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/tracewayapp/traceway/backend/app/services"
	"github.com/tracewayapp/traceway/backend/app/storage"
	"github.com/tracewayapp/traceway/backend/app/symbolicator"
	"github.com/tracewayapp/traceway/backend/app/symbolicator/sourcemap"

	"github.com/google/uuid"
)

const bundleFileFmt = "bundle-%06d.min.js"
const sourcePoolSize = 200
const namePoolSize = 500

type corpusManifest struct {
	Entries       int   `json:"entries"`
	Tokens        int   `json:"tokens"`
	BundleBytes   int   `json:"bundleBytes"`
	MapBytes      int   `json:"mapBytes"`
	TwBytes       int   `json:"twBytes"`
	ResolverBytes int64 `json:"resolverBytes"`
}

func loadManifest(corpusDir string) (*corpusManifest, error) {
	data, err := os.ReadFile(filepath.Join(corpusDir, "corpus-manifest.json"))
	if err != nil {
		return nil, err
	}
	var m corpusManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

func buildCanonical(tokens int) (bundle, mapJSON []byte) {
	var b strings.Builder
	b.Grow(tokens * 40)
	cols := make([]int, tokens)
	for i := range tokens {
		cols[i] = b.Len()
		fmt.Fprintf(&b, "function f%d(a,b){return a+b*3};", i)
	}
	bundle = []byte(b.String())

	sources := make([]string, sourcePoolSize)
	for i := range sources {
		sources[i] = fmt.Sprintf("src/module-%03d.js", i)
	}
	names := make([]string, namePoolSize)
	for i := range names {
		names[i] = fmt.Sprintf("origFn%d", i)
	}

	mappings := make([]byte, 0, tokens*10)
	prevCol, prevSrc, prevLine, prevSrcCol, prevName := 0, 0, 0, 0, 0
	for i := range tokens {
		if i > 0 {
			mappings = append(mappings, ',')
		}
		src := i % sourcePoolSize
		line := i % 1000
		srcCol := i % 80
		name := i % namePoolSize
		mappings = sourcemap.AppendVLQ(mappings, int64(cols[i]-prevCol))
		mappings = sourcemap.AppendVLQ(mappings, int64(src-prevSrc))
		mappings = sourcemap.AppendVLQ(mappings, int64(line-prevLine))
		mappings = sourcemap.AppendVLQ(mappings, int64(srcCol-prevSrcCol))
		mappings = sourcemap.AppendVLQ(mappings, int64(name-prevName))
		prevCol, prevSrc, prevLine, prevSrcCol, prevName = cols[i], src, line, srcCol, name
	}

	m := map[string]any{
		"version":  3,
		"sources":  sources,
		"names":    names,
		"mappings": string(mappings),
	}
	mapJSON, err := json.Marshal(m)
	if err != nil {
		panic(err)
	}
	return bundle, mapJSON
}

func generateCorpus(ctx context.Context, corpusDir string, projectId uuid.UUID, entries, tokens, workers int) error {
	bundle, mapJSON := buildCanonical(tokens)

	resolver, err := symbolicator.NewResolver(mapJSON, bundle)
	if err != nil {
		return fmt.Errorf("building canonical resolver: %w", err)
	}
	tw := resolver.MarshalTW()

	if frame, ok := resolver.Lookup(0, 10); !ok || !strings.HasPrefix(frame.File, "src/module-") {
		return fmt.Errorf("canonical resolver sanity check failed: %+v ok=%v", frame, ok)
	}

	var idx atomic.Int64
	var firstErr atomic.Value
	var wg sync.WaitGroup
	for range workers {
		wg.Go(func() {
			for {
				i := idx.Add(1) - 1
				if int(i) >= entries || firstErr.Load() != nil {
					return
				}
				base := services.SourceMapStorageKey(projectId, fmt.Sprintf(bundleFileFmt, i))
				for suffix, data := range map[string][]byte{"": bundle, ".map": mapJSON, ".tw": tw} {
					if err := storage.Store.Write(ctx, base+suffix, data); err != nil {
						firstErr.CompareAndSwap(nil, fmt.Errorf("writing %s%s: %w", base, suffix, err))
						return
					}
				}
				if i > 0 && i%1000 == 0 {
					fmt.Fprintf(os.Stderr, "generated %d/%d entries\n", i, entries)
				}
			}
		})
	}
	wg.Wait()
	if err := firstErr.Load(); err != nil {
		return err.(error)
	}

	manifest := corpusManifest{
		Entries:       entries,
		Tokens:        tokens,
		BundleBytes:   len(bundle),
		MapBytes:      len(mapJSON),
		TwBytes:       len(tw),
		ResolverBytes: resolver.ApproxSize(),
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(corpusDir, "corpus-manifest.json"), data, 0o644)
}
