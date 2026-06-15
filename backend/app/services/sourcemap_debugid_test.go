package services

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/tracewayapp/traceway/backend/app/storage"

	"github.com/google/uuid"
)

func TestExtractDebugIdFromMap(t *testing.T) {
	id := "85314830-023f-4cf1-a267-535f4e37bb17"
	if got := ExtractDebugId("app.js.map", []byte(`{"version":3,"debugId":"`+id+`"}`)); got != id {
		t.Errorf("debugId field: got %q, want %q", got, id)
	}
	if got := ExtractDebugId("app.js.map", []byte(`{"version":3,"debug_id":"`+strings.ToUpper(id)+`"}`)); got != id {
		t.Errorf("debug_id field should normalize: got %q, want %q", got, id)
	}
	if got := ExtractDebugId("app.js.map", []byte(`{"version":3}`)); got != "" {
		t.Errorf("missing field: got %q, want empty", got)
	}
	if got := ExtractDebugId("app.js.map", []byte(`{"version":3,"debugId":"not-a-uuid"}`)); got != "" {
		t.Errorf("invalid id: got %q, want empty", got)
	}
}

func TestExtractDebugIdFromBundle(t *testing.T) {
	id := "85314830-023f-4cf1-a267-535f4e37bb17"
	code := "console.log(1);\n//# debugId=" + id + "\n//# sourceMappingURL=app.js.map\n"
	if got := ExtractDebugId("app.js", []byte(code)); got != id {
		t.Errorf("bundle comment: got %q, want %q", got, id)
	}
	if got := ExtractDebugId("app.js", []byte("console.log(1);\n")); got != "" {
		t.Errorf("no comment: got %q, want empty", got)
	}
}

func TestResolveStackTracePrefersDebugId(t *testing.T) {
	useMemCache(t)
	prev := storage.Store
	defer func() { storage.Store = prev }()
	cs := &countingStorage{reads: map[string]int{}, data: map[string][]byte{}}
	storage.Store = cs

	projectId := uuid.New()
	prefix := fmt.Sprintf("sourcemaps/%s/", projectId)
	debugId := "85314830-023f-4cf1-a267-535f4e37bb17"
	seedFixture(t, cs, prefix+DebugIdMapName(debugId), "testdata/sourcemapcache/simple/minified.js.map")
	seedFixture(t, cs, prefix+DebugIdBundleName(debugId), "testdata/sourcemapcache/simple/minified.js")

	input := "Error: boom\nanonymous()\n    minified.js:1:11"
	debugIds := map[string]string{"minified.js": debugId}
	lines := strings.Split(ResolveStackTrace(context.Background(), projectId, input, debugIds), "\n")

	if got, want := lines[2], "    tests/fixtures/simple/original.js:2:10"; got != want {
		t.Errorf("location via debug id: got %q, want %q", got, want)
	}
	if got, want := lines[1], "abcd()"; got != want {
		t.Errorf("function name via debug id bundle: got %q, want %q", got, want)
	}
	if got := cs.reads[prefix+"minified.js.map"]; got != 0 {
		t.Errorf("filename map should not be read when debug id matches, got %d reads", got)
	}
}

func TestResolveStackTraceFallsBackToFilenameWhenDebugIdMissing(t *testing.T) {
	useMemCache(t)
	prev := storage.Store
	defer func() { storage.Store = prev }()
	cs := &countingStorage{reads: map[string]int{}, data: map[string][]byte{}}
	storage.Store = cs

	projectId := uuid.New()
	prefix := fmt.Sprintf("sourcemaps/%s/", projectId)
	seedFixture(t, cs, prefix+"minified.js.map", "testdata/sourcemapcache/simple/minified.js.map")
	seedFixture(t, cs, prefix+"minified.js", "testdata/sourcemapcache/simple/minified.js")

	input := "Error: boom\nanonymous()\n    minified.js:1:11"
	debugIds := map[string]string{"minified.js": "85314830-023f-4cf1-a267-535f4e37bb17"}
	lines := strings.Split(ResolveStackTrace(context.Background(), projectId, input, debugIds), "\n")

	if got, want := lines[2], "    tests/fixtures/simple/original.js:2:10"; got != want {
		t.Errorf("fallback location: got %q, want %q", got, want)
	}
}

func TestInvalidateSourceMapKeepsDebugIdDir(t *testing.T) {
	useMemCache(t)
	prev := storage.Store
	defer func() { storage.Store = prev }()
	storage.Store = &countingStorage{reads: map[string]int{}, data: map[string][]byte{}}

	projectId := uuid.New()
	debugId := "85314830-023f-4cf1-a267-535f4e37bb17"
	mapKey := SourceMapStorageKey(projectId, DebugIdMapName(debugId))
	bundleKey := SourceMapStorageKey(projectId, DebugIdBundleName(debugId))
	twKey := twKeyFor(mapKey)

	_, done, err := sharedCache.Get(context.Background(), twKey, loadSourceMapBlob(mapKey, bundleKey))
	done()
	if err == nil {
		t.Fatal("expected the missing map to fail to load")
	}
	if !sharedCache.IsNegative(twKey) {
		t.Fatal("expected a negative entry after the failed load")
	}

	InvalidateSourceMap(projectId, DebugIdBundleName(debugId))

	if sharedCache.IsNegative(twKey) {
		t.Error("expected debug id map key to be invalidated")
	}
}
