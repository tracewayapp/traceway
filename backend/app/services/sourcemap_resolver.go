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

type sourceMapCache struct {
	mu          sync.Mutex
	items       map[string]*list.Element
	order       *list.List
	loading     map[string]*sourceMapLoad
	maxEntries  int
	maxBytes    int64
	curBytes    int64
	hits        uint64
	misses      uint64
	evictions   uint64
	lastParseMs float64
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
}

func (c *sourceMapCache) getOrLoad(ctx context.Context, key string) (*parsedSourceMap, error) {
	c.mu.Lock()
	if el, ok := c.items[key]; ok {
		c.hits++
		c.order.MoveToFront(el)
		sm := el.Value.(*sourceMapCacheEntry).sm
		c.mu.Unlock()
		return sm, nil
	}
	if l, ok := c.loading[key]; ok {
		c.hits++
		c.mu.Unlock()
		<-l.done
		return l.sm, l.err
	}
	c.misses++
	l := &sourceMapLoad{done: make(chan struct{})}
	c.loading[key] = l
	c.mu.Unlock()

	l.sm, l.err = c.load(ctx, key)

	c.mu.Lock()
	delete(c.loading, key)
	if l.err == nil {
		c.insertLocked(key, l.sm)
	}
	c.mu.Unlock()
	close(l.done)
	return l.sm, l.err
}

func (c *sourceMapCache) load(ctx context.Context, key string) (*parsedSourceMap, error) {
	data, err := storage.Store.Read(context.WithoutCancel(ctx), key)
	if err != nil {
		traceway.CaptureException(fmt.Errorf("failed to read source map from storage (key=%s): %w", key, err))
		return nil, err
	}

	parseStart := time.Now()
	sm, err := parseSourceMap(data)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	c.lastParseMs = float64(time.Since(parseStart).Microseconds()) / 1000.0
	c.mu.Unlock()
	return sm, nil
}

func (c *sourceMapCache) insertLocked(key string, sm *parsedSourceMap) {
	if el, ok := c.items[key]; ok {
		entry := el.Value.(*sourceMapCacheEntry)
		c.curBytes += sm.size - entry.sm.size
		entry.sm = sm
		c.order.MoveToFront(el)
	} else {
		el := c.order.PushFront(&sourceMapCacheEntry{key: key, sm: sm})
		c.items[key] = el
		c.curBytes += sm.size
	}
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
