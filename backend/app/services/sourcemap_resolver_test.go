package services

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/tracewayapp/traceway/backend/app/storage"

	"github.com/google/uuid"
)

func seedFixture(t *testing.T, cs *countingStorage, key, fixturePath string) {
	t.Helper()
	data, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatal(err)
	}
	cs.data[key] = data
}

func TestResolveStackTraceWithBundle(t *testing.T) {
	InitSourceMapCache(100, 64<<20)
	prev := storage.Store
	defer func() { storage.Store = prev }()
	cs := &countingStorage{reads: map[string]int{}, data: map[string][]byte{}}
	storage.Store = cs

	projectId := uuid.New()
	prefix := fmt.Sprintf("sourcemaps/%s/", projectId)
	seedFixture(t, cs, prefix+"minified.js.map", "testdata/sourcemapcache/simple/minified.js.map")
	seedFixture(t, cs, prefix+"minified.js", "testdata/sourcemapcache/simple/minified.js")

	input := "Error: boom\nanonymous()\n    minified.js:1:11"
	lines := strings.Split(ResolveStackTrace(context.Background(), projectId, input), "\n")

	if got, want := lines[2], "    tests/fixtures/simple/original.js:2:10"; got != want {
		t.Errorf("location: got %q, want %q", got, want)
	}
	if got, want := lines[1], "abcd()"; got != want {
		t.Errorf("function name (from bundle): got %q, want %q", got, want)
	}
}

func TestResolveStackTraceLocationOnlyWithoutBundle(t *testing.T) {
	InitSourceMapCache(100, 64<<20)
	prev := storage.Store
	defer func() { storage.Store = prev }()
	cs := &countingStorage{reads: map[string]int{}, data: map[string][]byte{}}
	storage.Store = cs

	projectId := uuid.New()
	prefix := fmt.Sprintf("sourcemaps/%s/", projectId)
	seedFixture(t, cs, prefix+"preact-missing-source-contents.module.js.map", "testdata/sourcemapcache/preact-missing-source-contents.module.js.map")

	input := "Error: boom\nanonymous()\n    preact-missing-source-contents.module.js:1:133"
	lines := strings.Split(ResolveStackTrace(context.Background(), projectId, input), "\n")

	if got, want := lines[2], "    ../src/util.js:12:23"; got != want {
		t.Errorf("location: got %q, want %q", got, want)
	}
	if got, want := lines[1], "anonymous()"; got != want {
		t.Errorf("function name should stay unresolved without a bundle: got %q, want %q", got, want)
	}
}

func TestResolveStackTraceNegativeCacheAvoidsRepeatReads(t *testing.T) {
	InitSourceMapCache(100, 64<<20)
	prev := storage.Store
	defer func() { storage.Store = prev }()
	cs := &countingStorage{reads: map[string]int{}, data: map[string][]byte{}}
	storage.Store = cs

	projectId := uuid.New()
	input := "Error: boom\n    foo()\n    missing.js:1:5"

	_ = ResolveStackTrace(context.Background(), projectId, input)
	_ = ResolveStackTrace(context.Background(), projectId, input)

	mapKey := fmt.Sprintf("sourcemaps/%s/missing.js.map", projectId)
	if got := cs.reads[mapKey]; got != 1 {
		t.Errorf("negative cache should prevent repeat reads, got %d reads", got)
	}
}
