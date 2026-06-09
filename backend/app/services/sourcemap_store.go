package services

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/storage"
	"github.com/tracewayapp/traceway/backend/app/symbolicator"
)

type FrameRef struct {
	DebugId string
	URL     string
}

type resolverBuild func(context.Context) (*symbolicator.Resolver, int64, error)

type SourceMapStore interface {
	Resolve(ctx context.Context, ref FrameRef) (cacheKey string, build resolverBuild, ok bool)
}

type tracewayStore struct {
	byBasename map[string]*models.SourceMap
	byDebugId  map[string]*models.SourceMap
}

func newTracewayStore(sourceMaps []*models.SourceMap) *tracewayStore {
	s := &tracewayStore{
		byBasename: make(map[string]*models.SourceMap, len(sourceMaps)*2),
		byDebugId:  make(map[string]*models.SourceMap),
	}
	for _, sm := range sourceMaps {
		s.byBasename[sm.FileName] = sm
		s.byBasename[filepath.Base(sm.FileName)] = sm
		if sm.DebugId != "" {
			s.byDebugId[sm.DebugId] = sm
		}
	}
	return s
}

func (s *tracewayStore) Resolve(ctx context.Context, ref FrameRef) (string, resolverBuild, bool) {
	var mapRow *models.SourceMap
	if ref.DebugId != "" {
		mapRow = s.byDebugId[ref.DebugId]
	}
	if mapRow == nil {
		mapRow = findSourceMap(ref.URL, s.byBasename)
	}
	if mapRow == nil {
		return "", nil, false
	}

	bundleRow := s.byBasename[strings.TrimSuffix(mapRow.FileName, ".map")]

	cacheKey := mapRow.StorageKey
	if mapRow.DebugId != "" {
		cacheKey = "debugid:" + mapRow.DebugId
	}

	build := func(ctx context.Context) (*symbolicator.Resolver, int64, error) {
		readCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), sourceMapLoadTimeout)
		defer cancel()

		mapBytes, err := storage.Store.Read(readCtx, mapRow.StorageKey)
		if err != nil {
			return nil, 0, err
		}

		var bundleBytes []byte
		if bundleRow != nil {
			if b, readErr := storage.Store.Read(readCtx, bundleRow.StorageKey); readErr == nil {
				bundleBytes = b
			}
		}

		resolver, err := symbolicator.NewResolver(mapBytes, bundleBytes)
		if err != nil {
			return nil, 0, err
		}
		return resolver, resolver.ApproxSize(), nil
	}

	return cacheKey, build, true
}

func findSourceMap(stackFile string, byBasename map[string]*models.SourceMap) *models.SourceMap {
	mapName := stackFile + ".map"
	if sm, ok := byBasename[mapName]; ok {
		return sm
	}

	base := filepath.Base(stackFile) + ".map"
	if sm, ok := byBasename[base]; ok {
		return sm
	}

	cleanName := stackFile
	if idx := strings.IndexAny(cleanName, "?#"); idx != -1 {
		cleanName = cleanName[:idx]
	}
	mapName = filepath.Base(cleanName) + ".map"
	if sm, ok := byBasename[mapName]; ok {
		return sm
	}

	return nil
}
