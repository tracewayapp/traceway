package otelcontrollers

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/services"
	"github.com/tracewayapp/traceway/backend/app/storage"
)

func tokenProject(projectId uuid.UUID) *models.Project {
	token := "test-sm-token-" + projectId.String()
	return &models.Project{
		Id:             projectId,
		Token:          "test-token-" + projectId.String(),
		SourceMapToken: &token,
	}
}

type countingStore struct {
	inner fakeStore
	reads int
}

func (s *countingStore) Read(ctx context.Context, key string) ([]byte, error) {
	s.reads++
	return s.inner.Read(ctx, key)
}

func (s *countingStore) Write(context.Context, string, []byte) error { return nil }

func (s *countingStore) Delete(context.Context, string) error { return nil }

func TestOtelSymbolicateJs_NoTokenSkipsStorageLookups(t *testing.T) {
	projectId := uuid.MustParse("00000000-0000-0000-0000-0000000000ad")
	store := &countingStore{inner: fakeStore{files: map[string][]byte{
		services.SourceMapStorageKey(projectId, "minified.js.map"): []byte(testSourceMap),
	}}}
	prev := storage.Store
	storage.Store = store
	t.Cleanup(func() { storage.Store = prev })

	raw := "Error: boom\n    at t (https://cdn.example.com/assets/minified.js:1:11)"

	got := otelSymbolicateJs(nil, projectId, context.Background(), raw, "webjs", "")
	if got != "Error: boom\nt()\n    https://cdn.example.com/assets/minified.js:1:11" {
		t.Errorf("expected canonicalized-but-unresolved trace, got %q", got)
	}
	if store.reads != 0 {
		t.Errorf("expected ZERO storage reads without a source map token, got %d", store.reads)
	}

	got = otelSymbolicateJs(tokenProject(projectId), projectId, context.Background(), raw, "webjs", "")
	if !strings.Contains(got, "original.js") {
		t.Errorf("expected resolved trace once the token exists, got %q", got)
	}
	if store.reads == 0 {
		t.Error("expected storage reads after the token was generated")
	}
}
