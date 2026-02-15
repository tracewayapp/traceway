package services

import (
	"backend/app/cache"
	"backend/app/models"
	"backend/app/storage"
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-sourcemap/sourcemap"
	"github.com/google/uuid"
	traceway "go.tracewayapp.com"
)

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

	// Build lookup: basename of source map's file_name (without .map) -> source map
	// Also keep a map of file_name -> storage_key for direct lookup
	smByBasename := make(map[string]*models.SourceMap)
	for _, sm := range sourceMaps {
		smByBasename[sm.FileName] = sm
		// Also index by basename
		base := filepath.Base(sm.FileName)
		smByBasename[base] = sm
	}

	lines := strings.Split(stackTrace, "\n")
	resolved := make([]string, 0, len(lines))
	framesResolved := 0
	maxFrames := 50

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

		data, err := getSourceMapData(ctx, sm.StorageKey)
		if err != nil {
			resolved = append(resolved, line)
			continue
		}

		consumer, err := sourcemap.Parse("", data)
		if err != nil {
			resolved = append(resolved, line)
			continue
		}

		origFile, origName, origLine, origCol, ok := consumer.Source(lineNum, colNum)
		if !ok || origFile == "" {
			resolved = append(resolved, line)
			continue
		}

		if content := consumer.SourceContent(origFile); content != "" {
			if extracted := extractFunctionName(content, origLine); extracted != "" {
				origName = extracted
			}
		}

		resolved = append(resolved, fmt.Sprintf("%s%s:%d:%d", indent, origFile, origLine, origCol))
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
	// Try file.map directly
	mapName := stackFile + ".map"
	if sm, ok := smByBasename[mapName]; ok {
		return sm
	}

	// Try basename.map
	base := filepath.Base(stackFile) + ".map"
	if sm, ok := smByBasename[base]; ok {
		return sm
	}

	// Try without query params
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

func getSourceMapData(ctx context.Context, storageKey string) ([]byte, error) {
	if data, ok := cache.SourceMapCache.Get(storageKey); ok {
		return data, nil
	}

	data, err := storage.Store.Read(ctx, storageKey)
	if err != nil {
		traceway.CaptureException(fmt.Errorf("failed to read source map from storage (key=%s): %w", storageKey, err))
		return nil, err
	}

	cache.SourceMapCache.Put(storageKey, data)
	return data, nil
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
