package env

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// EvalsRoot is the cli/test/evals module directory.
func EvalsRoot() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

// BinariesForTest builds (or reuses) the binaries for a test, skipping it in
// -short mode or when no Go toolchain is on PATH.
func BinariesForTest(t testing.TB) *Binaries {
	t.Helper()
	if testing.Short() {
		t.Skip("boots a backend")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain not on PATH")
	}
	root := EvalsRoot()
	repo, err := RepoRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	b, err := BuildBinaries(repo, filepath.Join(root, ".cache"))
	if err != nil {
		t.Fatal(err)
	}
	return b
}
