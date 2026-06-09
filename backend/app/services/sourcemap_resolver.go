package services

import (
	"container/list"
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/storage"

	"github.com/google/uuid"
	traceway "go.tracewayapp.com"
)

const sourceMapLoadTimeout = 5 * time.Second
const sourceMapFailReportInterval = time.Minute

type sourceMapCache struct {
	mu                  sync.Mutex
	items               map[string]*list.Element
	order               *list.List
	loading             map[string]*sourceMapLoad
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
	key string
	sm  *parsedSourceMap
}

type sourceMapLoad struct {
	done chan struct{}
	sm   *parsedSourceMap
	err  error
}

var smCache = &sourceMapCache{
	items:      make(map[string]*list.Element),
	order:      list.New(),
	loading:    make(map[string]*sourceMapLoad),
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

func (c *sourceMapCache) getOrLoad(ctx context.Context, key string) (sm *parsedSourceMap, err error) {
	c.mu.Lock()
	if el, ok := c.items[key]; ok {
		c.hits++
		c.order.MoveToFront(el)
		cached := el.Value.(*sourceMapCacheEntry).sm
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
		return l.sm, l.err
	}
	c.misses++
	l := &sourceMapLoad{done: make(chan struct{})}
	c.loading[key] = l
	c.mu.Unlock()

	var parseMs float64
	defer func() {
		if r := recover(); r != nil {
			l.sm = nil
			l.err = fmt.Errorf("source map load panicked (key=%s): %v", key, r)
			c.reportLoadFailure(l.err)
			sm, err = nil, l.err
		}
		c.mu.Lock()
		delete(c.loading, key)
		if l.err == nil && l.sm != nil {
			c.lastParseMs = parseMs
			c.insertLocked(key, l.sm)
		} else {
			c.failures++
		}
		c.mu.Unlock()
		close(l.done)
	}()

	l.sm, parseMs, l.err = c.load(ctx, key)
	return l.sm, l.err
}

func (c *sourceMapCache) load(ctx context.Context, key string) (*parsedSourceMap, float64, error) {
	readCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), sourceMapLoadTimeout)
	defer cancel()
	data, err := storage.Store.Read(readCtx, key)
	if err != nil {
		c.reportLoadFailure(fmt.Errorf("failed to read source map from storage (key=%s): %w", key, err))
		return nil, 0, err
	}

	parseStart := time.Now()
	sm, err := parseSourceMap(data)
	if err != nil {
		c.reportLoadFailure(fmt.Errorf("failed to parse source map (key=%s): %w", key, err))
		return nil, 0, err
	}
	return sm, float64(time.Since(parseStart).Microseconds()) / 1000.0, nil
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
		traceway.CaptureException(fmt.Errorf("source map loads failed %d time(s) since last report: %w", report, err))
	}
}

func (c *sourceMapCache) insertLocked(key string, sm *parsedSourceMap) {
	el := c.order.PushFront(&sourceMapCacheEntry{key: key, sm: sm})
	c.items[key] = el
	c.curBytes += sm.size
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
		c.curBytes -= evicted.sm.size
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
var jsFuncDeclRe = regexp.MustCompile(
	`(?:(?:export\s+(?:default\s+)?)?function\s+(\w+)` +
		`|(?:const|let|var)\s+(\w+)\s*=` +
		`|^\s*(?:async\s+)?(\w+)\s*\([^)]*\)\s*\{)`,
)

var jsControlFlowKeywords = map[string]bool{
	"if": true, "for": true, "while": true, "switch": true,
	"catch": true, "return": true, "throw": true, "else": true,
}

func ResolveStackTrace(ctx context.Context, projectId uuid.UUID, stackTrace string, sourceMaps []*models.SourceMap) string {
	if len(sourceMaps) == 0 {
		return stackTrace
	}

	smByBasename := make(map[string]*models.SourceMap)
	for _, sm := range sourceMaps {
		smByBasename[sm.FileName] = sm
		base := filepath.Base(sm.FileName)
		smByBasename[base] = sm
	}

	lines := strings.Split(stackTrace, "\n")
	resolved := make([]string, 0, len(lines))
	framesResolved := 0
	maxFrames := 50

	localMaps := make(map[string]*parsedSourceMap)

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

		sm := findSourceMap(fileName, smByBasename)
		if sm == nil {
			resolved = append(resolved, line)
			continue
		}

		pm, err := getSourceMap(ctx, sm.StorageKey, localMaps)
		if err != nil || pm == nil {
			resolved = append(resolved, line)
			continue
		}

		origFile, origName, origLine, origCol, ok := pm.source(lineNum, colNum-1)
		if !ok {
			resolved = append(resolved, line)
			continue
		}

		if content := pm.sourceContent(origFile); content != "" {
			if extracted := extractFunctionName(content, origLine); extracted != "" {
				origName = extracted
			}
		}

		if origFile == "" {
			origFile = "<unknown>"
		}

		resolved = append(resolved, fmt.Sprintf("%s%s:%d:%d", indent, origFile, origLine, origCol+1))
		framesResolved++

		if origName != "" && len(resolved) >= 2 {
			prev := resolved[len(resolved)-2]
			if strings.HasSuffix(strings.TrimSpace(prev), "()") {
				trimmed := strings.TrimSpace(prev)
				indent := prev[:len(prev)-len(trimmed)]
				resolved[len(resolved)-2] = indent + origName + "()"
			}
		}
	}

	return strings.Join(resolved, "\n")
}

func findSourceMap(stackFile string, smByBasename map[string]*models.SourceMap) *models.SourceMap {
	mapName := stackFile + ".map"
	if sm, ok := smByBasename[mapName]; ok {
		return sm
	}

	base := filepath.Base(stackFile) + ".map"
	if sm, ok := smByBasename[base]; ok {
		return sm
	}

	cleanName := stackFile
	if idx := strings.IndexAny(cleanName, "?#"); idx != -1 {
		cleanName = cleanName[:idx]
	}
	mapName = filepath.Base(cleanName) + ".map"
	if sm, ok := smByBasename[mapName]; ok {
		return sm
	}

	return nil
}

func getSourceMap(ctx context.Context, storageKey string, localMaps map[string]*parsedSourceMap) (*parsedSourceMap, error) {
	if m, ok := localMaps[storageKey]; ok {
		return m, nil
	}
	m, err := smCache.getOrLoad(ctx, storageKey)
	if err != nil {
		localMaps[storageKey] = nil
		return nil, err
	}
	localMaps[storageKey] = m
	return m, nil
}

func extractFunctionName(sourceContent string, line int) string {
	lines := strings.Split(sourceContent, "\n")
	for i := line - 1; i >= 0 && i >= line-50; i-- {
		matches := jsFuncDeclRe.FindStringSubmatch(lines[i])
		if matches != nil {
			for _, m := range matches[1:] {
				if m != "" && !jsControlFlowKeywords[m] {
					return m
				}
			}
		}
	}
	return ""
}
