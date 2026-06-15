package storage

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestLocalStorageRejectsTraversalKeys(t *testing.T) {
	base := t.TempDir()
	s, err := NewLocalStorage(base)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	for _, key := range []string{"../escape.txt", "../../etc/x", "a/../../escape", "/abs/path", ""} {
		if err := s.Write(ctx, key, []byte("x")); err == nil {
			t.Errorf("Write(%q) succeeded, want rejection", key)
		}
		if _, err := s.Read(ctx, key); err == nil {
			t.Errorf("Read(%q) succeeded, want rejection", key)
		}
		if err := s.Delete(ctx, key); err == nil {
			t.Errorf("Delete(%q) succeeded, want rejection", key)
		}
	}

	escaped := filepath.Join(filepath.Dir(base), "escape.txt")
	if _, err := os.Stat(escaped); err == nil {
		t.Errorf("traversal write escaped to %s", escaped)
	}

	if err := s.Write(ctx, "ok/nested/file.txt", []byte("data")); err != nil {
		t.Fatalf("legit nested key rejected: %v", err)
	}
	got, err := s.Read(ctx, "ok/nested/file.txt")
	if err != nil || string(got) != "data" {
		t.Fatalf("legit read failed: got %q err %v", got, err)
	}
}
