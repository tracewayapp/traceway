package workspace

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func run(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%v: %v: %s", args, err, out)
	}
	return string(out)
}

// upstream builds a bare repository with one commit on main, standing in
// for the code host.
func upstream(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	work := filepath.Join(root, "work")
	run(t, "", "git", "init", "--quiet", "-b", "main", work)
	run(t, work, "git", "config", "user.email", "t@example.com")
	run(t, work, "git", "config", "user.name", "t")
	if err := os.WriteFile(filepath.Join(work, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, work, "git", "add", "-A")
	run(t, work, "git", "commit", "--quiet", "-m", "init")
	bare := filepath.Join(root, "upstream.git")
	run(t, "", "git", "clone", "--quiet", "--bare", work, bare)
	return bare
}

func TestMirrorCheckoutDiffAndCommit(t *testing.T) {
	ctx := context.Background()
	remote := upstream(t)
	mirrors := &Mirrors{Root: t.TempDir()}

	path, err := mirrors.Update(ctx, 7, "acme", "app", remote, nil)
	if err != nil {
		t.Fatalf("first update: %v", err)
	}
	if path != filepath.Join(mirrors.Root, "7", "acme", "app.git") {
		t.Fatalf("mirror path = %s", path)
	}
	again, err := mirrors.Update(ctx, 7, "acme", "app", remote, &Credential{Username: "x-access-token", Password: "secret"})
	if err != nil || again != path {
		t.Fatalf("second update must fetch the same mirror: %s, %v", again, err)
	}
	if config, _ := os.ReadFile(filepath.Join(path, "config")); strings.Contains(string(config), "secret") {
		t.Fatal("the credential leaked into the mirror config")
	}

	dir := filepath.Join(t.TempDir(), "attempt", "repo")
	if err := Checkout(ctx, path, remote, "main", dir); err != nil {
		t.Fatalf("checkout: %v", err)
	}
	if origin := strings.TrimSpace(run(t, dir, "git", "remote", "get-url", "origin")); origin != remote {
		t.Fatalf("origin = %s, want the real remote", origin)
	}

	changes, err := Changes(ctx, dir)
	if err != nil || len(changes) != 0 {
		t.Fatalf("clean checkout must have no changes: %v %v", changes, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "notes"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "notes", "helper.sh"), []byte("echo hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	changes, err = Changes(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 2 || changes[0].Status != "M" || changes[1].Path != "notes/helper.sh" || changes[1].Status != "??" {
		t.Fatalf("changes = %+v (a new directory must list its files)", changes)
	}
	if err := ScratchGuard(changes, DefaultScratchPatterns); !errors.Is(err, ErrScratchFile) || !strings.Contains(err.Error(), "notes/helper.sh") {
		t.Fatalf("the helper script must be refused: %v", err)
	}
	if err := os.RemoveAll(filepath.Join(dir, "notes")); err != nil {
		t.Fatal(err)
	}
	changes, _ = Changes(ctx, dir)
	if err := ScratchGuard(changes, DefaultScratchPatterns); err != nil {
		t.Fatalf("a plain source change must pass the guard: %v", err)
	}

	patch, err := Diff(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(patch, "+func main() {}") {
		t.Fatalf("patch = %s", patch)
	}
	if err := DiffLimit(patch, changes, 10, 0); !errors.Is(err, ErrDiffTooLarge) {
		t.Fatalf("byte limit: %v", err)
	}
	if err := DiffLimit(patch, changes, 0, 0); err != nil {
		t.Fatalf("no limit: %v", err)
	}
	if err := SecretScan(patch); err != nil {
		t.Fatalf("clean patch: %v", err)
	}

	if err := Commit(ctx, dir, "traceway/fix-abc-1", "Fix abc", nil); err != nil {
		t.Fatalf("commit and push: %v", err)
	}
	if _, err := mirrors.Update(ctx, 7, "acme", "app", remote, nil); err != nil {
		t.Fatal(err)
	}
	if !HasBranch(ctx, path, "traceway/fix-abc-1") {
		t.Fatal("the pushed branch must be visible in the refreshed mirror")
	}
	if HasBranch(ctx, path, "nope") {
		t.Fatal("HasBranch must be false for an unknown branch")
	}
}

func TestSecretScanCatchesAddedCredentialsOnly(t *testing.T) {
	for _, line := range []string{
		"+const key = \"AKIAIOSFODNN7EXAMPLE\"",
		"+token: ghp_abcdefghijklmnopqrstuvwxyz0123456789ABCD",
		"+-----BEGIN RSA PRIVATE KEY-----",
		"+TRACEWAY_TOKEN=twp_abcdefghijklmnopqrstuv",
		"+slack: xoxb-1234567890-abcdefghij",
		"+ANTHROPIC_API_KEY=sk-ant-api03-abcdefghijklmnopqrstuvwxyz",
	} {
		if err := SecretScan("--- a/x\n+++ b/x\n" + line + "\n"); !errors.Is(err, ErrSecretInDiff) {
			t.Errorf("%q must be refused, got %v", line, err)
		}
	}
	removed := "--- a/x\n+++ b/x\n-const key = \"AKIAIOSFODNN7EXAMPLE\"\n+const key = os.Getenv(\"AWS_KEY\")\n"
	if err := SecretScan(removed); err != nil {
		t.Fatalf("removing a credential must pass: %v", err)
	}
}
