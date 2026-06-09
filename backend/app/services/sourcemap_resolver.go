package services

import (
	"container/list"
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/symbolicator"

	traceway "go.tracewayapp.com"
)

const sourceMapLoadTimeout = 5 * time.Second
const sourceMapFailReportInterval = time.Minute

type sourceMapCache struct {
	mu                  sync.Mutex
	items               map[string]*list.Element
	order               *list.List
	loading             map[string]*resolverLoad
	maxEntries          int
	maxBytes            int64
	curBytes            int64
	hits                uint64
	misses              uint64
	evictions           uint64
	failures            uint64
	lastParseMs         float64
	failuresSinceReport uint64
	lastFailAt          time.Time
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
	maxEntries: 200,
	maxBytes:   500 << 20,
}

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
		} else {
			c.failures++
		}
		c.mu.Unlock()
		close(l.done)
	}()

	start := time.Now()
	l.resolver, size, l.err = build(ctx)
	buildMs = float64(time.Since(start).Microseconds()) / 1000.0
	if l.err != nil {
		c.reportLoadFailure(fmt.Errorf("failed to build source map resolver (key=%s): %w", key, l.err))
	}
	return l.resolver, l.err
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
	Entries     int
	Bytes       int64
	MaxEntries  int
	MaxBytes    int64
	Hits        uint64
	Misses      uint64
	Evictions   uint64
	Failures    uint64
	LastParseMs float64
}

func SourceMapStats() SourceMapCacheStats {
	smCache.mu.Lock()
	defer smCache.mu.Unlock()
	return SourceMapCacheStats{
		Entries:     smCache.order.Len(),
		Bytes:       smCache.curBytes,
		MaxEntries:  smCache.maxEntries,
		MaxBytes:    smCache.maxBytes,
		Hits:        smCache.hits,
		Misses:      smCache.misses,
		Evictions:   smCache.evictions,
		Failures:    smCache.failures,
		LastParseMs: smCache.lastParseMs,
	}
}

var stackFrameRe = regexp.MustCompile(`^(\s{4})(.+):(\d+):(\d+)$`)

func ResolveStackTrace(ctx context.Context, stackTrace string, sourceMaps []*models.SourceMap) string {
	if len(sourceMaps) == 0 {
		return stackTrace
	}

	store := newTracewayStore(sourceMaps)

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

		cacheKey, build, ok := store.Resolve(ctx, FrameRef{URL: fileName})
		if !ok {
			resolved = append(resolved, line)
			continue
		}

		resolver, err := getResolver(ctx, cacheKey, build, localResolvers)
		if err != nil || resolver == nil {
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

func getResolver(ctx context.Context, cacheKey string, build resolverBuild, local map[string]*symbolicator.Resolver) (*symbolicator.Resolver, error) {
	if r, ok := local[cacheKey]; ok {
		return r, nil
	}
	r, err := smCache.getOrBuild(ctx, cacheKey, build)
	if err != nil {
		local[cacheKey] = nil
		return nil, err
	}
	local[cacheKey] = r
	return r, nil
}
