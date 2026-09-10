package sandbox

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// TestBwrapLive proves the confinement on a Linux host with bubblewrap: the
// child sees only the allowlisted environment, cannot write outside its
// writable bind, cannot see a sibling workspace, and dies at the wall clock.
// Skipped elsewhere; the :browser and :agent images are where it runs.
func TestBwrapLive(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("bubblewrap is Linux-only")
	}
	probe := Spec{Command: []string{"/bin/sh", "-c", "true"}, Workdir: "/tmp"}
	if err := (Bwrap{}).Probe(context.Background(), probe); err != nil {
		t.Skipf("this host cannot sandbox: %v", err)
	}
	t.Setenv("PARENT_SECRET", "sentinel-9f3a")

	root := t.TempDir()
	mine := filepath.Join(root, "workspaces", "attempt-1")
	sibling := filepath.Join(root, "workspaces", "attempt-2")
	for _, dir := range []string{mine, sibling} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(sibling, "secret.txt"), []byte("sibling"), 0o644); err != nil {
		t.Fatal(err)
	}

	run := func(script string, limits Limits) (string, error) {
		spec := Spec{
			Command:  []string{"/bin/sh", "-c", script},
			Workdir:  mine,
			ReadOnly: []Bind{{Path: root}},
			Hidden:   []string{filepath.Join(root, "workspaces")},
			Writable: []Bind{{Path: mine}},
			Env:      map[string]string{"PATH": "/usr/bin:/bin", "ONLY": "yes"},
			Limits:   limits,
		}
		cmd, cleanup, err := (Bwrap{}).Build(context.Background(), spec)
		if err != nil {
			return "", err
		}
		defer cleanup()
		out, err := cmd.CombinedOutput()
		return string(out), err
	}

	out, err := run(`env | sort; ls /proc/1/environ >/dev/null 2>&1 && cat /proc/*/environ 2>/dev/null | tr '\0' '\n' | grep -c sentinel-9f3a || echo 0`, Limits{})
	if err != nil {
		t.Fatalf("env probe: %v\n%s", err, out)
	}
	if strings.Contains(out, "PARENT_SECRET") || strings.Contains(out, "sentinel-9f3a") {
		t.Fatalf("the parent environment leaked into the sandbox:\n%s", out)
	}
	if !strings.Contains(out, "ONLY=yes") {
		t.Fatalf("the allowlisted environment did not reach the child:\n%s", out)
	}

	out, err = run(`echo mine > own.txt && cat own.txt; echo x > ../escape.txt 2>/dev/null; (echo x > `+root+`/readonly.txt 2>/dev/null && echo WROTE-READONLY || echo readonly-blocked); ls ../ 2>/dev/null | grep -c attempt-2 || true`, Limits{})
	if err != nil {
		t.Fatalf("write probe: %v\n%s", err, out)
	}
	if !strings.Contains(out, "mine") || !strings.Contains(out, "readonly-blocked") {
		t.Fatalf("the child must write its workspace and nothing in a read-only bind:\n%s", out)
	}
	if strings.Contains(out, "attempt-2") || strings.HasSuffix(strings.TrimSpace(out), "1") {
		t.Fatalf("the sibling workspace must be hidden:\n%s", out)
	}
	if _, err := os.Stat(filepath.Join(mine, "own.txt")); err != nil {
		t.Fatalf("the write inside the workspace must land on the host: %v", err)
	}
	// The hidden container is a tmpfs inside the namespace: a write there
	// succeeds for the child and is gone with the sandbox.
	if _, err := os.Stat(filepath.Join(root, "workspaces", "escape.txt")); !os.IsNotExist(err) {
		t.Fatal("a write beside the workspace reached the host")
	}
	if _, err := os.Stat(filepath.Join(root, "readonly.txt")); !os.IsNotExist(err) {
		t.Fatal("a write into the read-only bind reached the host")
	}

	started := time.Now()
	if out, err := run(`sleep 30; echo survived`, Limits{WallClock: 500 * time.Millisecond}); err == nil || strings.Contains(out, "survived") {
		t.Fatalf("the wall clock must kill the sandboxed process group: %v %q", err, out)
	}
	if time.Since(started) > 5*time.Second {
		t.Fatal("the kill took longer than the limit")
	}
}
