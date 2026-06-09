package services

import (
	"context"
	"strings"
	"testing"

	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/storage"

	"github.com/google/uuid"
)

func TestResolveStackTraceWithBundle(t *testing.T) {
	InitSourceMapCache(100, 64<<20)
	store, err := storage.NewLocalStorage("testdata")
	if err != nil {
		t.Fatal(err)
	}
	storage.Store = store
	projectId := uuid.New()

	sourceMaps := []*models.SourceMap{
		{ProjectId: projectId, Version: "1.0.0", FileName: "minified.js.map", StorageKey: "sourcemapcache/simple/minified.js.map"},
		{ProjectId: projectId, Version: "1.0.0", FileName: "minified.js", StorageKey: "sourcemapcache/simple/minified.js"},
	}

	input := "Error: boom\nanonymous()\n    minified.js:1:11"
	lines := strings.Split(ResolveStackTrace(context.Background(), input, sourceMaps), "\n")

	if got, want := lines[2], "    tests/fixtures/simple/original.js:2:10"; got != want {
		t.Errorf("location: got %q, want %q", got, want)
	}
	if got, want := lines[1], "abcd()"; got != want {
		t.Errorf("function name (from bundle): got %q, want %q", got, want)
	}
}

func TestResolveStackTraceLocationOnlyWithoutBundle(t *testing.T) {
	InitSourceMapCache(100, 64<<20)
	store, err := storage.NewLocalStorage("testdata")
	if err != nil {
		t.Fatal(err)
	}
	storage.Store = store
	projectId := uuid.New()

	sourceMaps := []*models.SourceMap{
		{ProjectId: projectId, Version: "1.0.0", FileName: "preact-missing-source-contents.module.js.map", StorageKey: "sourcemapcache/preact-missing-source-contents.module.js.map"},
	}

	input := "Error: boom\nanonymous()\n    preact-missing-source-contents.module.js:1:133"
	lines := strings.Split(ResolveStackTrace(context.Background(), input, sourceMaps), "\n")

	if got, want := lines[2], "    ../src/util.js:12:23"; got != want {
		t.Errorf("location: got %q, want %q", got, want)
	}
	if got, want := lines[1], "anonymous()"; got != want {
		t.Errorf("function name should stay unresolved without a bundle: got %q, want %q", got, want)
	}
}
