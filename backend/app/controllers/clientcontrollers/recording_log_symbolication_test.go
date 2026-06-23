package clientcontrollers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/tracewayapp/traceway/backend/app/services"
	"github.com/tracewayapp/traceway/backend/app/storage"
)

type memStore struct{ data map[string][]byte }

func (m *memStore) Write(_ context.Context, key string, data []byte) error {
	m.data[key] = data
	return nil
}

func (m *memStore) Read(_ context.Context, key string) ([]byte, error) {
	d, ok := m.data[key]
	if !ok {
		return nil, fmt.Errorf("not found: %s", key)
	}
	return d, nil
}

func (m *memStore) Delete(_ context.Context, key string) error {
	delete(m.data, key)
	return nil
}

func newTestGinContext() *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	return c
}

func seedSimpleBundle(t *testing.T, store *memStore, projectId uuid.UUID) {
	t.Helper()
	prefix := fmt.Sprintf("sourcemaps/%s/", projectId)
	for _, f := range []struct{ key, path string }{
		{prefix + "minified.js.map", "../../services/testdata/sourcemapcache/simple/minified.js.map"},
		{prefix + "minified.js", "../../services/testdata/sourcemapcache/simple/minified.js"},
	} {
		data, err := os.ReadFile(f.path)
		if err != nil {
			t.Fatal(err)
		}
		store.data[f.key] = data
	}
}

func TestSymbolicateRecordingErrorLogs(t *testing.T) {
	services.InitSourceMapCache(100, 64<<20)
	prev := storage.Store
	defer func() { storage.Store = prev }()
	store := &memStore{data: map[string][]byte{}}
	storage.Store = store

	projectId := uuid.New()
	seedSimpleBundle(t, store, projectId)

	logs := []map[string]any{
		{"type": "log", "level": "info", "message": "products loaded"},
		{"type": "log", "level": "log", "message": "user clicked apply"},
		{"type": "log", "level": "error", "message": "plain error string with no stack"},
		{
			"type":      "log",
			"level":     "error",
			"timestamp": "2026-06-23T18:34:52.259Z",
			"message":   "TypeError: Cannot read properties of null (reading 'percent')\n    at someFn (http://localhost:3000/minified.js:1:11)",
		},
	}
	raw, err := json.Marshal(logs)
	if err != nil {
		t.Fatal(err)
	}

	out := symbolicateRecordingErrorLogs(newTestGinContext(), projectId, raw)

	var got []map[string]any
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatal(err)
	}

	if got[0]["message"] != "products loaded" || got[1]["message"] != "user clicked apply" {
		t.Errorf("non-error logs were modified: %v / %v", got[0]["message"], got[1]["message"])
	}
	if got[2]["message"] != "plain error string with no stack" {
		t.Errorf("error log without a stack trace was modified: %v", got[2]["message"])
	}

	errMsg, _ := got[3]["message"].(string)
	if !strings.Contains(errMsg, "original.js:2:10") {
		t.Errorf("console.error stack was not symbolicated to original source:\n%s", errMsg)
	}
	if !strings.Contains(errMsg, "abcd()") {
		t.Errorf("function name was not resolved from the bundle:\n%s", errMsg)
	}
	if strings.Contains(errMsg, "minified.js:1:11") {
		t.Errorf("symbolicated message still contains the minified frame:\n%s", errMsg)
	}
	// Unrelated fields on the modified entry survive.
	if got[3]["timestamp"] != "2026-06-23T18:34:52.259Z" || got[3]["level"] != "error" {
		t.Errorf("other fields on the error log were lost: %v", got[3])
	}
}

func TestSymbolicateRecordingErrorLogsLeavesUnresolvableUntouched(t *testing.T) {
	services.InitSourceMapCache(100, 64<<20)
	prev := storage.Store
	defer func() { storage.Store = prev }()
	store := &memStore{data: map[string][]byte{}}
	storage.Store = store

	projectId := uuid.New()
	// No source map seeded -> nothing resolves -> message must stay byte-identical.
	original := "TypeError: boom\n    at On (http://localhost:8090/assets/index-C5fNnsBr.js:139:11021)"
	logs := []map[string]any{{"type": "log", "level": "error", "message": original}}
	raw, _ := json.Marshal(logs)

	out := symbolicateRecordingErrorLogs(newTestGinContext(), projectId, raw)

	var got []map[string]any
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatal(err)
	}
	if got[0]["message"] != original {
		t.Errorf("unresolvable stack should be left untouched, got:\n%v", got[0]["message"])
	}
}
