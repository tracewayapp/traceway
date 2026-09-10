package agentrunner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOutputRedactionAndBounds(t *testing.T) {
	got := redactOutput(`known-password sk-ant-private xoxb-slack ghp_github`, "known-password")
	if strings.Count(got, "[REDACTED]") != 4 {
		t.Fatal(got)
	}
	var b tailBuffer
	_, _ = b.Write([]byte(strings.Repeat("a", 16384)))
	_, _ = b.Write([]byte("last"))
	if b.Len() != 8192 || !strings.HasSuffix(b.String(), "last") {
		t.Fatal("unbounded output tail")
	}
}

func TestCLIProfileCannotFollowSymlinkOutsideWorkspace(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	outside := t.TempDir()
	if err := os.Mkdir(home, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(home, ".config")); err != nil {
		t.Fatal(err)
	}
	if err := writeCLIProfile(home, "http://localhost", "secret-token", "project"); err == nil {
		t.Fatal("profile write escaped through a symlink")
	}
	entries, err := os.ReadDir(outside)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatal("wrote outside the workspace")
	}
}
