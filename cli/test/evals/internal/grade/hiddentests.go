package grade

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// HiddenTests verifies a code change: it records the diff, copies the case's
// hidden tests (testdata/hidden/<name>/) into the workspace only after the
// agent finished, runs the case command, and checks the diff touched the
// culprit. A real model's patch runs in a container with no network; it runs
// on the host only for oracle runs (our own code) or when AllowHostTests is
// set explicitly.
func HiddenTests(ctx context.Context, in *Input) *Verdict {
	spec := in.Case.Expect.Code
	v := newVerdict("hidden_tests", spec.Gating)
	changed, err := changedFiles(in.Workspace, in.Pristine)
	if err != nil {
		v.fail("read the diff: %v", err)
		return v
	}
	v.note("changed files: %s", strings.Join(changed, ", "))
	if spec.TouchesCulprit && in.Case.Expect.Answer != nil && in.Case.Expect.Answer.Culprit != nil {
		want := in.Case.Expect.Answer.Culprit.File
		hit := false
		for _, c := range changed {
			if samePath(c, want) {
				hit = true
			}
		}
		if !hit {
			v.fail("the diff does not touch the culprit file %s", want)
		}
	}
	for _, g := range spec.MustChange {
		if !anyGlob(g, changed) {
			v.fail("the diff does not change %s", g)
		}
	}
	for _, g := range spec.MustNotChange {
		if anyGlob(g, changed) {
			v.fail("the diff must not change %s", g)
		}
	}
	if spec.Static || spec.Command == "" {
		return v
	}
	if spec.HiddenTests != "" {
		src := filepath.Join(in.EvalsRoot, "testdata", "hidden", spec.HiddenTests)
		if err := copyTree(src, in.Workspace); err != nil {
			v.fail("copy hidden tests: %v", err)
			return v
		}
	}
	timeout := spec.Timeout.D()
	if timeout == 0 {
		timeout = 5 * time.Minute
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	var cmd *exec.Cmd
	switch {
	case in.FakeModel || in.AllowHostTests:
		cmd = exec.CommandContext(ctx, "sh", "-c", spec.Command)
		cmd.Dir = in.Workspace
		cmd.Env = hostTestEnv(in.Workspace)
		// The timeout kills the whole group: go test's children would
		// otherwise keep the output pipe open past it.
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	case in.SandboxImage != "" && dockerAvailable():
		name := fmt.Sprintf("twevals-hidden-%d-%d", os.Getpid(), time.Now().UnixNano())
		cmd = exec.CommandContext(ctx, "docker", "run", "--rm", "--name", name, "--network", "none",
			"--user", fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid()),
			"-v", in.Workspace+":/work", "-w", "/work", "-e", "HOME=/tmp", "-e", "GOFLAGS=-mod=mod", "-e", "GOCACHE=/tmp/gocache",
			in.SandboxImage, "sh", "-c", spec.Command)
		// Killing the docker client leaves the container running; remove it.
		cmd.Cancel = func() error {
			_ = exec.Command("docker", "rm", "-f", name).Run()
			return cmd.Process.Kill()
		}
	default:
		v.note("tests not run: no container runtime for the model's patch (set TWEVALS_HOST_TESTS=1 to run it on the host)")
		// The diff checks above still gate; only a clean diff is skipped.
		v.Skipped = len(v.Failures) == 0
		return v
	}
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	cmd.WaitDelay = 10 * time.Second
	err = cmd.Run()
	tail := out.String()
	if len(tail) > 3000 {
		tail = tail[len(tail)-3000:]
	}
	if err != nil {
		v.fail("%s failed: %v\n%s", spec.Command, err, tail)
		return v
	}
	v.note("%s passed", spec.Command)
	return v
}

func hostTestEnv(work string) []string {
	home := filepath.Join(work, ".twevals-home")
	_ = os.MkdirAll(home, 0o755)
	env := []string{"HOME=" + home, "PATH=" + os.Getenv("PATH"), "TZ=UTC", "LANG=C.UTF-8", "GOFLAGS=-mod=mod", "GOTOOLCHAIN=local"}
	for _, k := range []string{"GOCACHE", "GOMODCACHE", "GOPATH"} {
		if v := os.Getenv(k); v != "" {
			env = append(env, k+"="+v)
		}
	}
	if os.Getenv("GOCACHE") == "" {
		if d, err := os.UserCacheDir(); err == nil {
			env = append(env, "GOCACHE="+filepath.Join(d, "go-build"))
		}
	}
	return env
}

func dockerAvailable() bool {
	if _, err := exec.LookPath("docker"); err != nil {
		return false
	}
	return exec.Command("docker", "info").Run() == nil
}

func anyGlob(g string, files []string) bool {
	for _, f := range files {
		if ok, _ := path.Match(g, f); ok || globAny(g, f) || f == g {
			return true
		}
	}
	return false
}

// copyTree copies the hidden tests into the workspace. The agent may have
// left symlinks there (the setup container can make them), so every target
// directory must resolve inside dst and the file itself is never followed:
// a planted link is replaced, not written through.
func copyTree(src, dst string) error {
	realDst, err := filepath.EvalSymlinks(dst)
	if err != nil {
		return err
	}
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			if fi, err := os.Lstat(target); err == nil && !fi.IsDir() {
				if err := os.Remove(target); err != nil {
					return err
				}
			}
			return os.MkdirAll(target, 0o755)
		}
		dir, err := filepath.EvalSymlinks(filepath.Dir(target))
		if err != nil {
			return err
		}
		if dir != realDst && !strings.HasPrefix(dir, realDst+string(filepath.Separator)) {
			return fmt.Errorf("%s resolves outside the workspace", filepath.Dir(target))
		}
		raw, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		if fi, err := os.Lstat(target); err == nil && !fi.Mode().IsRegular() {
			if err := os.Remove(target); err != nil {
				return err
			}
		}
		f, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC|syscall.O_NOFOLLOW, 0o644)
		if err != nil {
			return err
		}
		if _, err := f.Write(raw); err != nil {
			f.Close()
			return err
		}
		return f.Close()
	})
}
