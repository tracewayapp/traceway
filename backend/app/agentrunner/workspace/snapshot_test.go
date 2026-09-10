package workspace

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSnapshotIgnoresHostileGitMetadataAndUpdatesExistingBranch(t *testing.T) {
	ctx := context.Background()
	remote := upstream(t)
	mirrors := &Mirrors{Root: t.TempDir()}
	mirror, err := mirrors.Update(ctx, 1, "owner", "repo", remote, nil)
	if err != nil {
		t.Fatal(err)
	}
	ref := "main"
	for round, content := range []string{"package main\nfunc main() {}\n", "package main\nfunc main() { println(1) }\n"} {
		dir := filepath.Join(t.TempDir(), "repo")
		if err := Checkout(ctx, mirror, remote, ref, dir); err != nil {
			t.Fatal(err)
		}
		marker := filepath.Join(t.TempDir(), "hook-ran")
		hook := "#!/bin/sh\ntouch '" + marker + "'\nexit 1\n"
		for _, name := range []string{"pre-commit", "pre-push"} {
			if err := os.WriteFile(filepath.Join(dir, ".git", "hooks", name), []byte(hook), 0700); err != nil {
				t.Fatal(err)
			}
		}
		run(t, dir, "git", "config", "core.fsmonitor", "touch '"+marker+"'")
		run(t, dir, "git", "config", "filter.hostile.clean", "touch '"+marker+"'")
		run(t, dir, "git", "remote", "set-url", "--push", "origin", "ext::sh -c false")
		if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, ".gitattributes"), []byte("main.go filter=hostile\n"), 0600); err != nil {
			t.Fatal(err)
		}
		snapshot, err := NewSnapshot(ctx, mirror, remote, ref, dir)
		if err != nil {
			t.Fatal(err)
		}
		patch, err := snapshot.Diff(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(patch, "+func main()") {
			t.Fatalf("round %d: missing change: %s", round, patch)
		}
		if err := snapshot.Commit(ctx, "traceway/regression", "fix", &Credential{Username: "test", Password: "dummy-token"}); err != nil {
			t.Fatal(err)
		}
		snapshot.Close()
		if _, err := os.Stat(marker); !os.IsNotExist(err) {
			t.Fatalf("untrusted Git program ran: %v", err)
		}
		if _, err := mirrors.Update(ctx, 1, "owner", "repo", remote, nil); err != nil {
			t.Fatal(err)
		}
		ref = "traceway/regression"
		got := run(t, "", "git", "--git-dir", remote, "show", ref+":main.go")
		if got != content {
			t.Fatalf("published tree differs: %q", got)
		}
	}
}

func TestSnapshotPreservesSymlinksWithoutReadingTheirTargets(t *testing.T) {
	ctx := context.Background()
	remote := upstream(t)
	source := t.TempDir()
	secret := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(secret, []byte("private-value"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(secret, filepath.Join(source, "link")); err != nil {
		t.Fatal(err)
	}
	s, err := NewSnapshot(ctx, remote, remote, "main", source)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	patch, err := s.Diff(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(patch, "private-value") {
		t.Fatal("read a symlink target outside the repository")
	}
}
