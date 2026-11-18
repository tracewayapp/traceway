package retention

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestIsSafeStorageSubdir(t *testing.T) {
	cases := []struct {
		path string
		safe bool
	}{
		{"/", false},
		{".", false},
		{"/recordings", false},
		{"recordings", false},
		{"/storage/recordings", true},
		{"/var/lib/traceway/recordings", true},
	}
	for _, c := range cases {
		if got := isSafeStorageSubdir(c.path); got != c.safe {
			t.Errorf("isSafeStorageSubdir(%q) = %v, want %v", c.path, got, c.safe)
		}
	}
}

func TestRunRecordingDiskCleanup_DeletesOldKeepsFresh(t *testing.T) {
	dir := t.TempDir()

	oldFile := filepath.Join(dir, "project-1", "old.json")
	freshFile := filepath.Join(dir, "project-1", "fresh.json")
	nestedOld := filepath.Join(dir, "project-2", "sessions", "abc", "0.json")
	nestedFresh := filepath.Join(dir, "project-2", "sessions", "xyz", "0.json")

	mustWriteFile(t, oldFile, "old")
	mustWriteFile(t, freshFile, "fresh")
	mustWriteFile(t, nestedOld, "nested-old")
	mustWriteFile(t, nestedFresh, "nested-fresh")

	aged := time.Now().Add(-40 * 24 * time.Hour)
	mustChtime(t, oldFile, aged)
	mustChtime(t, nestedOld, aged)

	runDirAgeCleanup(context.Background(), dir, 30)

	assertNotExists(t, oldFile)
	assertExists(t, freshFile)
	assertNotExists(t, nestedOld)
	assertExists(t, nestedFresh)
}

func TestRunRecordingDiskCleanup_PrunesEmptyDirsKeepsRoot(t *testing.T) {
	dir := t.TempDir()

	emptyAfter := filepath.Join(dir, "project-empty", "old.json")
	mixedOld := filepath.Join(dir, "project-mixed", "old.json")
	mixedFresh := filepath.Join(dir, "project-mixed", "fresh.json")

	mustWriteFile(t, emptyAfter, "")
	mustWriteFile(t, mixedOld, "")
	mustWriteFile(t, mixedFresh, "")

	aged := time.Now().Add(-40 * 24 * time.Hour)
	mustChtime(t, emptyAfter, aged)
	mustChtime(t, mixedOld, aged)

	runDirAgeCleanup(context.Background(), dir, 30)

	assertExists(t, dir)
	assertNotExists(t, filepath.Join(dir, "project-empty"))
	assertExists(t, filepath.Join(dir, "project-mixed"))
	assertExists(t, mixedFresh)
}

func TestRunRecordingDiskCleanup_NoOpOnMissingDir(t *testing.T) {
	parent := t.TempDir()
	missing := filepath.Join(parent, "does-not-exist")

	runDirAgeCleanup(context.Background(), missing, 30)

	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Fatalf("expected missing dir to remain absent, got err=%v", err)
	}
}

func TestRunRecordingDiskCleanup_RefusesPathThatIsAFile(t *testing.T) {
	parent := t.TempDir()
	filePath := filepath.Join(parent, "not-a-dir")
	mustWriteFile(t, filePath, "important")

	aged := time.Now().Add(-40 * 24 * time.Hour)
	mustChtime(t, filePath, aged)

	runDirAgeCleanup(context.Background(), filePath, 30)

	assertExists(t, filePath)
}

func TestRunRecordingDiskCleanup_KeepsAllWhenNoneAreOld(t *testing.T) {
	dir := t.TempDir()

	files := []string{
		filepath.Join(dir, "p1", "a.json"),
		filepath.Join(dir, "p1", "b.json"),
		filepath.Join(dir, "p2", "c.json"),
	}
	for _, f := range files {
		mustWriteFile(t, f, "fresh")
	}

	runDirAgeCleanup(context.Background(), dir, 30)

	for _, f := range files {
		assertExists(t, f)
	}
}

func mustWriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func mustChtime(t *testing.T, path string, when time.Time) {
	t.Helper()
	if err := os.Chtimes(path, when, when); err != nil {
		t.Fatalf("chtimes %s: %v", path, err)
	}
}

func assertExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Errorf("expected %s to exist, got err=%v", path, err)
	}
}

func assertNotExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("expected %s to be gone, got err=%v", path, err)
	}
}
