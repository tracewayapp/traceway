package controllers

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/tracewayapp/traceway/backend/app/middleware"
	"github.com/tracewayapp/traceway/backend/app/services"
	"github.com/tracewayapp/traceway/backend/app/storage"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type memStore struct {
	mu   sync.Mutex
	data map[string][]byte
}

func newMemStore() *memStore { return &memStore{data: map[string][]byte{}} }

func (s *memStore) Write(_ context.Context, k string, d []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[k] = d
	return nil
}

func (s *memStore) Read(_ context.Context, k string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if d, ok := s.data[k]; ok {
		return d, nil
	}
	return nil, storage.ErrNotFound
}

func (s *memStore) Delete(_ context.Context, k string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, k)
	return nil
}

func runSymbolsUpload(t *testing.T, projectId uuid.UUID, filename string, data []byte) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	fw, _ := w.CreateFormFile("files", filename)
	fw.Write(data)
	w.Close()

	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/symbols/upload", &body)
	c.Request.Header.Set("Content-Type", w.FormDataContentType())
	c.Set(middleware.ProjectIdContextKey, projectId)

	SymbolsController.Upload(c)
	return rec
}

func runSymbolsUploadWithFields(t *testing.T, projectId uuid.UUID, filename string, data []byte, fields map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	fw, _ := w.CreateFormFile("files", filename)
	fw.Write(data)
	for k, v := range fields {
		_ = w.WriteField(k, v)
	}
	w.Close()

	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/symbols/upload", &body)
	c.Request.Header.Set("Content-Type", w.FormDataContentType())
	c.Set(middleware.ProjectIdContextKey, projectId)

	SymbolsController.Upload(c)
	return rec
}

func iosUploadFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "symbolicator", "ios", "fixtures", "sample", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func androidUploadFixture(t *testing.T, parts ...string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(append([]string{"..", "symbolicator", "android", "fixtures"}, parts...)...))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestUploadAndroidMapping(t *testing.T) {
	prev := storage.Store
	defer func() { storage.Store = prev }()
	store := newMemStore()
	storage.Store = store

	projectId := uuid.New()
	const proguardUuid = "6A8CB813-45F6-3652-AD33-778FD1EAB196"
	rec := runSymbolsUploadWithFields(t, projectId, "mapping.txt",
		androidUploadFixture(t, "r8", "mapping.txt"),
		map[string]string{"proguard_uuid": proguardUuid})
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	if _, ok := store.data[services.AndroidMappingKey(projectId, proguardUuid)]; !ok {
		t.Errorf("mapping not stored at %s; keys=%v", services.AndroidMappingKey(projectId, proguardUuid), keysOf(store))
	}
}

func TestUploadAndroidMappingRequiresUuid(t *testing.T) {
	prev := storage.Store
	defer func() { storage.Store = prev }()
	storage.Store = newMemStore()

	rec := runSymbolsUpload(t, uuid.New(), "mapping.txt", androidUploadFixture(t, "r8", "mapping.txt"))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing proguard_uuid, got %d: %s", rec.Code, rec.Body.String())
	}
}

func keysOf(s *memStore) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, 0, len(s.data))
	for k := range s.data {
		out = append(out, k)
	}
	return out
}

func TestUploadIOSFatStoresPerSlice(t *testing.T) {
	prev := storage.Store
	defer func() { storage.Store = prev }()
	store := newMemStore()
	storage.Store = store

	projectId := uuid.New()
	rec := runSymbolsUpload(t, projectId, "App", iosUploadFixture(t, "sample_fat.dsym"))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	for _, u := range []string{"3185fc2b69b738638f132803dac76198", "7b1d8aee3ed8339e88b1595d0f815ffe"} {
		if _, ok := store.data[services.IOSSymbolsKey(projectId, u)]; !ok {
			t.Errorf("missing stored dSYM for slice %s", u)
		}
	}
}

func TestUploadIOSSkipsNonSymbolFile(t *testing.T) {
	prev := storage.Store
	defer func() { storage.Store = prev }()
	store := newMemStore()
	storage.Store = store

	rec := runSymbolsUpload(t, uuid.New(), "junk.bin", []byte("not a symbol file"))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	if len(store.data) != 0 {
		t.Errorf("expected nothing stored for a non-Mach-O/non-.symbols file, got %d keys", len(store.data))
	}
}
