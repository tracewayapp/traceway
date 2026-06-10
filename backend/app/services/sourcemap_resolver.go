package services

import (
	"container/list"
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tracewayapp/traceway/backend/app/storage"
	"github.com/tracewayapp/traceway/backend/app/symbolicator"

	"github.com/google/uuid"
	traceway "go.tracewayapp.com"
)

const sourceMapLoadTimeout = 5 * time.Second
const sourceMapFailReportInterval = time.Minute
const sourceMapNegativeBaseTTL = time.Minute
const sourceMapTransientNegativeBaseTTL = 15 * time.Second
const sourceMapNegativeMaxTTL = 15 * time.Minute
const sourceMapNegativeMaxKeys = 10000

type resolverBuild func(context.Context) (*symbolicator.Resolver, int64, error)

type resolverCache interface {
	getOrBuild(ctx context.Context, key string, build resolverBuild) (*symbolicator.Resolver, error)
	isNegative(key string) bool
	invalidate(key string)
	stats() SourceMapCacheStats
}

type sourceMapCache struct {
	mu                  sync.Mutex
	items               map[string]*list.Element
	order               *list.List
	loading             map[string]*resolverLoad
	negative            map[string]*negativeEntry
	maxEntries          int
	maxBytes            int64
	curBytes            int64
	hits                uint64
	misses              uint64
	evictions           uint64
	failures            uint64
	notFound            uint64
	negativeHits        uint64
	lastParseMs         float64
	failuresSinceReport uint64
	lastFailAt          time.Time
}

type negativeEntry struct {
	expiresAt time.Time
	failures  uint32
}

type sourceMapCacheEntry struct {
	key      string
	resolver *symbolicator.Resolver
	size     int64
}

type resolverLoad struct {
	done     chan struct{}
	resolver *symbolicator.Resolver
	err      error
}

var smCache = &sourceMapCache{
	items:      make(map[string]*list.Element),
	order:      list.New(),
	loading:    make(map[string]*resolverLoad),
	negative:   make(map[string]*negativeEntry),
	maxEntries: 200,
	maxBytes:   500 << 20,
}

var activeSMCache resolverCache = smCache

func InitSourceMapCache(maxEntries int, maxBytes int64) {
	smCache.mu.Lock()
	defer smCache.mu.Unlock()
	smCache.maxEntries = maxEntries
	smCache.maxBytes = maxBytes
	smCache.evictLocked()
}

func (c *sourceMapCache) getOrBuild(ctx context.Context, key string, build resolverBuild) (resolver *symbolicator.Resolver, err error) {
	c.mu.Lock()
	if el, ok := c.items[key]; ok {
		c.hits++
		c.order.MoveToFront(el)
		cached := el.Value.(*sourceMapCacheEntry).resolver
		c.mu.Unlock()
		return cached, nil
	}
	if l, ok := c.loading[key]; ok {
		c.mu.Unlock()
		<-l.done
		if l.err == nil {
			c.mu.Lock()
			c.hits++
			c.mu.Unlock()
		}
		return l.resolver, l.err
	}
	c.misses++
	l := &resolverLoad{done: make(chan struct{})}
	c.loading[key] = l
	c.mu.Unlock()

	var size int64
	var buildMs float64
	defer func() {
		if r := recover(); r != nil {
			l.resolver = nil
			l.err = fmt.Errorf("source map resolver build panicked (key=%s): %v", key, r)
			c.reportLoadFailure(l.err)
			resolver, err = nil, l.err
		}
		c.mu.Lock()
		delete(c.loading, key)
		if l.err == nil && l.resolver != nil {
			c.lastParseMs = buildMs
			c.insertLocked(key, l.resolver, size)
			delete(c.negative, key)
		} else if l.err != nil {
			if errors.Is(l.err, storage.ErrNotFound) {
				c.notFound++
				c.markNegativeLocked(key, sourceMapNegativeBaseTTL)
			} else {
				c.failures++
				c.markNegativeLocked(key, sourceMapTransientNegativeBaseTTL)
			}
		}
		c.mu.Unlock()
		close(l.done)
	}()

	start := time.Now()
	l.resolver, size, l.err = build(ctx)
	buildMs = float64(time.Since(start).Microseconds()) / 1000.0
	if l.err != nil && !errors.Is(l.err, storage.ErrNotFound) {
		c.reportLoadFailure(fmt.Errorf("failed to build source map resolver (key=%s): %w", key, l.err))
	}
	return l.resolver, l.err
}

func (c *sourceMapCache) isNegative(key string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.negative[key]
	if !ok || time.Now().After(e.expiresAt) {
		return false
	}
	c.negativeHits++
	return true
}

func (c *sourceMapCache) markNegativeLocked(key string, base time.Duration) {
	e := c.negative[key]
	if e == nil {
		if len(c.negative) >= sourceMapNegativeMaxKeys {
			c.pruneNegativeLocked()
		}
		e = &negativeEntry{}
		c.negative[key] = e
	}
	ttl := min(base<<min(e.failures, 16), sourceMapNegativeMaxTTL)
	e.failures++
	e.expiresAt = time.Now().Add(ttl)
}

func (c *sourceMapCache) pruneNegativeLocked() {
	now := time.Now()
	for k, e := range c.negative {
		if now.After(e.expiresAt) {
			delete(c.negative, k)
		}
	}
	for k := range c.negative {
		if len(c.negative) < sourceMapNegativeMaxKeys {
			break
		}
		delete(c.negative, k)
	}
}

func (c *sourceMapCache) invalidate(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.negative, key)
	if el, ok := c.items[key]; ok {
		evicted := c.order.Remove(el).(*sourceMapCacheEntry)
		delete(c.items, key)
		c.curBytes -= evicted.size
	}
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
	activeSMCache.invalidate(SourceMapStorageKey(projectId, name))
}

func (c *sourceMapCache) reportLoadFailure(err error) {
	var report uint64
	c.mu.Lock()
	c.failuresSinceReport++
	if time.Since(c.lastFailAt) >= sourceMapFailReportInterval {
		report = c.failuresSinceReport
		c.failuresSinceReport = 0
		c.lastFailAt = time.Now()
	}
	c.mu.Unlock()
	if report > 0 {
		traceway.CaptureException(fmt.Errorf("source map resolver builds failed %d time(s) since last report: %w", report, err))
	}
}

func (c *sourceMapCache) insertLocked(key string, resolver *symbolicator.Resolver, size int64) {
	el := c.order.PushFront(&sourceMapCacheEntry{key: key, resolver: resolver, size: size})
	c.items[key] = el
	c.curBytes += size
	c.evictLocked()
}

func (c *sourceMapCache) evictLocked() {
	for c.order.Len() > c.maxEntries || c.curBytes > c.maxBytes {
		back := c.order.Back()
		if back == nil {
			break
		}
		evicted := c.order.Remove(back).(*sourceMapCacheEntry)
		delete(c.items, evicted.key)
		c.curBytes -= evicted.size
		c.evictions++
	}
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
	return activeSMCache.stats()
}

func (c *sourceMapCache) stats() SourceMapCacheStats {
	c.mu.Lock()
	defer c.mu.Unlock()
	return SourceMapCacheStats{
		Entries:         c.order.Len(),
		Bytes:           c.curBytes,
		MaxEntries:      c.maxEntries,
		MaxBytes:        c.maxBytes,
		Hits:            c.hits,
		Misses:          c.misses,
		Evictions:       c.evictions,
		Failures:        c.failures,
		NotFound:        c.notFound,
		NegativeHits:    c.negativeHits,
		NegativeEntries: len(c.negative),
		LastParseMs:     c.lastParseMs,
		StoreHits:       smStoreHits.Load(),
		Builds:          smBuilds.Load(),
	}
}

var stackFrameRe = regexp.MustCompile(`^(\s{4})(.+):(\d+):(\d+)$`)

func ResolveStackTrace(ctx context.Context, projectId uuid.UUID, stackTrace string, debugIds map[string]string) string {
	prefix := SourceMapStorageKey(projectId, "")

	lines := strings.Split(stackTrace, "\n")
	resolved := make([]string, 0, len(lines))
	framesResolved := 0
	maxFrames := 50

	localResolvers := make(map[string]*symbolicator.Resolver)

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

		resolver := frameResolver(ctx, prefix, fileName, base, debugIds, localResolvers)
		if resolver == nil {
			resolved = append(resolved, line)
			continue
		}

		frame, ok := resolver.Lookup(uint32(lineNum-1), uint32(colNum-1))
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

func frameResolver(ctx context.Context, prefix, fileName, base string, debugIds map[string]string, local map[string]*symbolicator.Resolver) *symbolicator.Resolver {
	id := NormalizeDebugId(debugIds[fileName])
	if id == "" {
		id = NormalizeDebugId(debugIds[base])
	}
	if id != "" {
		mapKey := prefix + DebugIdMapName(id)
		if !activeSMCache.isNegative(mapKey) {
			bundleKey := prefix + DebugIdBundleName(id)
			if r, err := getResolver(ctx, mapKey, buildResolver(mapKey, bundleKey), local); err == nil && r != nil {
				return r
			}
		}
	}

	mapKey := prefix + base + ".map"
	if activeSMCache.isNegative(mapKey) {
		return nil
	}
	r, err := getResolver(ctx, mapKey, buildResolver(mapKey, prefix+base), local)
	if err != nil {
		return nil
	}
	return r
}

func getResolver(ctx context.Context, cacheKey string, build resolverBuild, local map[string]*symbolicator.Resolver) (*symbolicator.Resolver, error) {
	if r, ok := local[cacheKey]; ok {
		return r, nil
	}
	r, err := activeSMCache.getOrBuild(ctx, cacheKey, build)
	if err != nil {
		local[cacheKey] = nil
		return nil, err
	}
	local[cacheKey] = r
	return r, nil
}

var smStoreHits, smBuilds atomic.Uint64

func buildResolver(mapKey, bundleKey string) resolverBuild {
	return func(ctx context.Context) (*symbolicator.Resolver, int64, error) {
		base := context.WithoutCancel(ctx)

		refreshStoreTw := true
		twKey := twKeyFor(mapKey)
		twBytes, err := readWithTimeout(base, twKey)
		if err == nil {
			if r, twErr := symbolicator.OpenTW(twBytes); twErr == nil {
				smStoreHits.Add(1)
				return r, r.ApproxSize(), nil
			}
		} else if !errors.Is(err, storage.ErrNotFound) {
			refreshStoreTw = false
			traceway.CaptureException(fmt.Errorf("failed to read tw artifact, rebuilding from source map (key=%s): %w", twKey, err))
		}

		mapBytes, err := readWithTimeout(base, mapKey)
		if err != nil {
			return nil, 0, err
		}

		var bundleBytes []byte
		if b, readErr := readWithTimeout(base, bundleKey); readErr == nil {
			bundleBytes = b
		} else if !errors.Is(readErr, storage.ErrNotFound) {
			return nil, 0, fmt.Errorf("failed to read bundle (key=%s): %w", bundleKey, readErr)
		}

		resolver, err := symbolicator.NewResolver(mapBytes, bundleBytes)
		if err != nil {
			return nil, 0, err
		}
		smBuilds.Add(1)
		if refreshStoreTw {
			if werr := storage.Store.Write(base, twKey, resolver.MarshalTW()); werr != nil {
				traceway.CaptureException(fmt.Errorf("failed to refresh tw artifact in storage (key=%s): %w", twKey, werr))
			}
		}
		return resolver, resolver.ApproxSize(), nil
	}
}

func readWithTimeout(ctx context.Context, key string) ([]byte, error) {
	readCtx, cancel := context.WithTimeout(ctx, sourceMapLoadTimeout)
	defer cancel()
	return storage.Store.Read(readCtx, key)
}
