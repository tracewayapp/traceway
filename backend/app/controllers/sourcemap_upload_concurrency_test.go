package controllers

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/v2/app/middleware"
	"github.com/tracewayapp/traceway/backend/v2/app/services"
	"github.com/tracewayapp/traceway/backend/v2/app/storage"
)

type concurrentUploadStore struct {
	mu       sync.Mutex
	delay    time.Duration
	failKey  string
	live     int
	peak     int
	writes   int
	contents map[string]string
}

func (s *concurrentUploadStore) Write(ctx context.Context, key string, data []byte) error {
	s.mu.Lock()
	s.live++
	s.peak = max(s.peak, s.live)
	s.writes++
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.live--
		s.mu.Unlock()
	}()

	select {
	case <-time.After(s.delay):
	case <-ctx.Done():
		return ctx.Err()
	}
	if s.failKey != "" && strings.HasSuffix(key, s.failKey) {
		return errors.New("synthetic storage failure")
	}
	s.mu.Lock()
	s.contents[key] = string(data)
	s.mu.Unlock()
	return nil
}

func (s *concurrentUploadStore) Read(context.Context, string) ([]byte, error) {
	return nil, storage.ErrNotFound
}
func (s *concurrentUploadStore) Delete(context.Context, string) error { return nil }

type uploadFile struct{ name, body string }

func runSourceMapUpload(t *testing.T, store storage.Storage, files []uploadFile) (*httptest.ResponseRecorder, *gin.Context, uuid.UUID) {
	t.Helper()
	previous := storage.Store
	storage.Store = store
	t.Cleanup(func() {
		deadline := time.Now().Add(10 * time.Second)
		for services.SourceMapWarmupBacklog() > 0 {
			if time.Now().After(deadline) {
				t.Fatal("source-map warm-ups did not drain")
			}
			time.Sleep(5 * time.Millisecond)
		}
		storage.Store = previous
	})

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for _, f := range files {
		part, err := writer.CreateFormFile("files", f.name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(part, f.body); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	projectId := uuid.New()
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/sourcemaps/upload", &body)
	c.Request.Header.Set("Content-Type", writer.FormDataContentType())
	c.Set(middleware.ProjectIdContextKey, projectId)
	SourceMapController.Upload(c)
	return rec, c, projectId
}

func TestSourceMapUploadWritesConcurrentlyWithinBound(t *testing.T) {
	const fileCount = 80
	store := &concurrentUploadStore{delay: 20 * time.Millisecond, contents: map[string]string{}}
	var files []uploadFile
	for i := range fileCount {
		files = append(files, uploadFile{
			name: fmt.Sprintf("chunk-%d.js.map", i),
			body: fmt.Sprintf(`{"version":3,"debugId":"00000000-0000-4000-8000-%012d"}`, i),
		})
	}

	started := time.Now()
	rec, _, _ := runSourceMapUpload(t, store, files)
	elapsed := time.Since(started)

	if rec.Code != http.StatusOK || rec.Body.String() != fmt.Sprintf(`{"uploaded":%d}`, fileCount) {
		t.Fatalf("response: %d %s", rec.Code, rec.Body.String())
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if len(store.contents) != 2*fileCount {
		t.Fatalf("stored %d objects, want %d (file + debug id alias each)", len(store.contents), 2*fileCount)
	}
	if store.peak > sourceMapStoreConcurrency {
		t.Fatalf("peak concurrent writes %d exceeded the bound %d", store.peak, sourceMapStoreConcurrency)
	}
	if store.peak < 2 {
		t.Fatalf("writes ran one at a time (peak %d)", store.peak)
	}
	sequential := time.Duration(2*fileCount) * store.delay
	if elapsed > sequential/2 {
		t.Fatalf("upload took %s, sequential writes would take %s", elapsed, sequential)
	}
}

func TestSourceMapUploadKeepsUploadOrderForDuplicateKeys(t *testing.T) {
	store := &concurrentUploadStore{delay: time.Millisecond, contents: map[string]string{}}
	const sharedDebugId = "11111111-2222-4333-8444-555555555555"
	var files []uploadFile
	for i := range 40 {
		files = append(files, uploadFile{
			name: "app.js.map",
			body: fmt.Sprintf(`{"version":3,"debugId":"%s","x":%d}`, sharedDebugId, i),
		})
	}
	last := files[len(files)-1].body

	rec, _, projectId := runSourceMapUpload(t, store, files)
	if rec.Code != http.StatusOK {
		t.Fatalf("response: %d %s", rec.Code, rec.Body.String())
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	for _, key := range []string{
		"sourcemaps/" + projectId.String() + "/app.js.map",
		"sourcemaps/" + projectId.String() + "/by-debug-id/" + sharedDebugId + ".js.map",
	} {
		got, ok := store.contents[key]
		if !ok {
			t.Fatalf("missing %s, have %v", key, storedKeys(store.contents))
		}
		if got != last {
			t.Fatalf("%s holds %s, want the last uploaded file %s", key, got, last)
		}
	}
}

func storedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func TestSourceMapUploadStopsOnStorageFailure(t *testing.T) {
	const fileCount = 200
	store := &concurrentUploadStore{delay: 5 * time.Millisecond, failKey: "/chunk-3.js", contents: map[string]string{}}
	var files []uploadFile
	for i := range fileCount {
		files = append(files, uploadFile{name: fmt.Sprintf("chunk-%d.js", i), body: "console.log(1)"})
	}

	rec, c, _ := runSourceMapUpload(t, store, files)
	if rec.Code != http.StatusInternalServerError || len(c.Errors) != 1 {
		t.Fatalf("failure response: %d %v", rec.Code, c.Errors)
	}
	if !strings.Contains(c.Errors[0].Error(), "failed to write source map to storage") {
		t.Fatalf("unexpected error: %v", c.Errors[0])
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.writes > fileCount/2 {
		t.Fatalf("%d writes were attempted after a failure in the first batch", store.writes)
	}
}

func TestSourceMapUploadRejectsOversizedFileBeforeStoring(t *testing.T) {
	store := &concurrentUploadStore{contents: map[string]string{}}
	rec, _, _ := runSourceMapUpload(t, store, []uploadFile{
		{name: "ok.js", body: "console.log(1)"},
		{name: "huge.js.map", body: strings.Repeat("x", sourceMapMaxFileBytes+1)},
	})
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "huge.js.map exceeds 50MB limit") {
		t.Fatalf("response: %d %s", rec.Code, rec.Body.String())
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.writes != 0 {
		t.Fatalf("%d files were stored before the oversized file was rejected", store.writes)
	}
}
