package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSearchSkipsSymlinks: the setup container shares the workspace and can
// plant a symlink to a host file; the host-side search never reads through
// it.
func TestSearchSkipsSymlinks(t *testing.T) {
	root := t.TempDir()
	work := filepath.Join(root, "work")
	os.MkdirAll(work, 0o755)
	os.WriteFile(filepath.Join(root, "secret"), []byte("TOKEN=host-secret\n"), 0o644)
	os.Symlink(filepath.Join(root, "secret"), filepath.Join(work, "leak.txt"))
	os.WriteFile(filepath.Join(work, "app.txt"), []byte("TOKEN=from-env\n"), 0o644)
	raw, _ := json.Marshal(map[string]string{"pattern": "TOKEN"})
	r := (&Search{Sandbox: &Sandbox{Root: work}}).Call(context.Background(), raw)
	if strings.Contains(r.Output, "host-secret") || !strings.Contains(r.Output, "from-env") {
		t.Fatalf("search output: %q", r.Output)
	}
}
