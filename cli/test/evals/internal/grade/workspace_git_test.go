package grade

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
)

// TestWorkspaceGitIgnoresRepoConfig: a .git/config the agent wrote (the
// setup suite's container shell can) never runs a program on the host when
// the graders list or diff the workspace's files, and a change it committed
// still counts as changed.
func TestWorkspaceGitIgnoresRepoConfig(t *testing.T) {
	dir := t.TempDir()
	work := filepath.Join(dir, "work")
	os.MkdirAll(work, 0o755)
	os.WriteFile(filepath.Join(work, "main.go"), []byte("package main\n"), 0o644)
	os.WriteFile(filepath.Join(work, "util.go"), []byte("package main\n"), 0o644)
	os.WriteFile(filepath.Join(work, "gone.go"), []byte("package main\n"), 0o644)
	git := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = work
		cmd.Env = []string{"HOME=" + work, "PATH=" + os.Getenv("PATH"), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null"}
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Skipf("git unavailable: %v %s", err, out)
		}
	}
	git("init", "-q")
	git("add", "-A")
	git("-c", "user.email=x@y", "-c", "user.name=x", "commit", "-qm", "init")
	pristine, err := Snapshot(work)
	if err != nil {
		t.Fatal(err)
	}

	// What the agent does: edit, commit (hiding the edit from git diff
	// HEAD), add files, and plant config that runs programs.
	os.WriteFile(filepath.Join(work, "util.go"), []byte("package main\n\nfunc util() {}\n"), 0o644)
	git("add", "util.go")
	git("-c", "user.email=x@y", "-c", "user.name=x", "commit", "-qm", "fix")
	os.Remove(filepath.Join(work, "gone.go"))
	hooks := filepath.Join(work, "hooks")
	os.MkdirAll(hooks, 0o755)
	for _, h := range []string{"post-index-change", "reference-transaction"} {
		os.WriteFile(filepath.Join(hooks, h), []byte("#!/bin/sh\necho x > ../PWNED\n"), 0o755)
	}
	cfg, _ := os.ReadFile(filepath.Join(work, ".git", "config"))
	cfg = append(cfg, []byte("[core]\n\tfsmonitor = \"echo x > ../PWNED; false\"\n\thooksPath = hooks\n\tworktree = ..\n"+
		"[filter \"x\"]\n\tclean = \"sh -c 'echo x > ../PWNED; cat'\"\n\tprocess = \"sh -c 'echo x > ../PWNED'\"\n")...)
	os.WriteFile(filepath.Join(work, ".git", "config"), cfg, 0o644)
	os.WriteFile(filepath.Join(work, ".gitattributes"), []byte("* filter=x\n"), 0o644)
	os.WriteFile(filepath.Join(work, ".git", "info", "attributes"), []byte("* filter=x\n"), 0o644)
	os.WriteFile(filepath.Join(work, ".gitignore"), []byte("build/\n"), 0o644)
	os.MkdirAll(filepath.Join(work, "build"), 0o755)
	os.WriteFile(filepath.Join(work, "build", "out.js"), []byte("x"), 0o644)
	os.WriteFile(filepath.Join(work, "new.go"), []byte("package main\n"), 0o644)
	os.WriteFile(filepath.Join(work, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644)

	files, ok := gitVisibleFiles(work)
	if !ok || !slices.Contains(files, "main.go") || !slices.Contains(files, "new.go") || slices.Contains(files, "work/main.go") {
		t.Errorf("gitVisibleFiles = %v, %v", files, ok)
	}
	changed, err := changedFiles(work, pristine)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"main.go", "util.go", "gone.go", "new.go"} {
		if !slices.Contains(changed, want) {
			t.Errorf("changedFiles = %v, missing %s", changed, want)
		}
	}
	if slices.Contains(changed, "build/out.js") {
		t.Errorf("changedFiles = %v: ignored build output must not count", changed)
	}
	for _, p := range []string{filepath.Join(dir, "PWNED"), filepath.Join(work, "PWNED")} {
		if _, err := os.Stat(p); err == nil {
			t.Fatalf("git ran a program from the workspace's config (%s)", p)
		}
	}

	// A .git file pointing at another repository is refused.
	os.Rename(filepath.Join(work, ".git"), filepath.Join(dir, "elsewhere"))
	os.WriteFile(filepath.Join(work, ".git"), []byte("gitdir: "+filepath.Join(dir, "elsewhere")+"\n"), 0o644)
	if _, ok := gitVisibleFiles(work); ok {
		t.Fatal("a gitdir: file must not be followed")
	}
}

// TestGraderReadsSkipPlantedFiles: symlinks and FIFOs in the workspace are
// never read through, and the hidden-test copy does not write through a
// planted link.
func TestGraderReadsSkipPlantedFiles(t *testing.T) {
	dir := t.TempDir()
	work := filepath.Join(dir, "work")
	outside := filepath.Join(dir, "outside")
	os.MkdirAll(work, 0o755)
	os.MkdirAll(outside, 0o755)
	os.WriteFile(filepath.Join(outside, "secret"), []byte("token-123"), 0o644)
	os.Symlink(filepath.Join(outside, "secret"), filepath.Join(work, "leak.go"))
	if _, ok := readRegular(filepath.Join(work, "leak.go")); ok {
		t.Fatal("a symlink must not be read through")
	}
	if err := checkNoTokens(work, []string{"token-123"}); err != nil {
		t.Fatalf("a symlink to a file outside the workspace is not a leak in it: %v", err)
	}

	src := filepath.Join(dir, "hidden")
	os.MkdirAll(filepath.Join(src, "pkg"), 0o755)
	os.WriteFile(filepath.Join(src, "pkg", "h_test.go"), []byte("package pkg\n"), 0o644)
	os.Symlink(outside, filepath.Join(work, "pkg"))
	if err := copyTree(src, work); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(outside, "h_test.go")); err == nil {
		t.Fatal("the hidden-test copy wrote through a symlinked directory")
	}
	if fi, err := os.Lstat(filepath.Join(work, "pkg", "h_test.go")); err != nil || !fi.Mode().IsRegular() {
		t.Fatalf("the hidden test must land in the workspace: %v", err)
	}
}
