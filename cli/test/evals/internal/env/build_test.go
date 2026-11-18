package env

import (
	"path/filepath"
	"testing"
	"time"
)

// TestLockFileExcludes holds the build lock and checks that a second holder
// waits for it. flock locks belong to the open file, so two opens in one
// process exclude each other the same way two processes do.
func TestLockFileExcludes(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".build.lock")
	unlock, err := lockFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := make(chan struct{})
	go func() {
		defer close(got)
		second, err := lockFile(path)
		if err != nil {
			t.Error(err)
			return
		}
		second()
	}()
	select {
	case <-got:
		t.Fatal("a second holder took the lock while the first still held it")
	case <-time.After(200 * time.Millisecond):
	}
	unlock()
	select {
	case <-got:
	case <-time.After(5 * time.Second):
		t.Fatal("the lock was not handed over after unlock")
	}
}
