package sandbox

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Bwrap confines the process with bubblewrap: fresh pid, ipc, uts and user
// namespaces with their own /proc, the system directories bound read-only,
// a minimal /etc (DNS, CA certificates, locale; never the whole directory,
// which would expose unit files and environment files holding secrets),
// tmpfs over /tmp, and only the spec's writable binds writable. The network
// namespace stays shared unless the policy says off; bwrap cannot filter
// egress.
type Bwrap struct{}

const probeTimeout = 15 * time.Second

// SystemBinds are the read-only directories every sandboxed process gets,
// so binaries and libraries resolve.
var SystemBinds = []string{"/usr", "/bin", "/sbin", "/lib", "/lib32", "/lib64"}

// EtcBinds is the allowlist of /etc entries a process needs to resolve
// names, trust certificates, know its timezone and look up its own user.
var EtcBinds = []string{
	"/etc/resolv.conf",
	"/etc/hosts",
	"/etc/nsswitch.conf",
	"/etc/localtime",
	"/etc/ssl",
	"/etc/pki",
	"/etc/ca-certificates",
	"/etc/ca-certificates.conf",
	"/etc/fonts",
	"/etc/passwd",
	"/etc/group",
	"/etc/machine-id",
}

func (Bwrap) Name() string { return string(ModeBwrap) }

func (b Bwrap) Probe(ctx context.Context, spec Spec) error {
	if runtime.GOOS != "linux" {
		return fmt.Errorf("bubblewrap is Linux-only (this host is %s)", runtime.GOOS)
	}
	if _, err := exec.LookPath("bwrap"); err != nil {
		return fmt.Errorf("bubblewrap (bwrap) is not on PATH: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()
	cmd, cleanup, err := b.Build(ctx, spec)
	if err != nil {
		return err
	}
	defer cleanup()
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("the bubblewrap probe failed (%v): %s", err, firstLines(string(output), 3))
	}
	return nil
}

func (Bwrap) Build(ctx context.Context, spec Spec) (*exec.Cmd, Cleanup, error) {
	if len(spec.Command) == 0 {
		return nil, nil, errors.New("sandbox: spec has no command")
	}
	bwrap, err := exec.LookPath("bwrap")
	if err != nil {
		return nil, nil, fmt.Errorf("bubblewrap (bwrap) is not on PATH: %w", err)
	}
	ctx, cancel := withWallClock(ctx, spec.Limits.WallClock)
	args := append(Args(spec), spec.Command...)
	program, programArgs := bwrap, args
	if wrapper := cgroupWrapper(spec.Limits); len(wrapper) > 0 {
		program = wrapper[0]
		programArgs = append(append(wrapper[1:], bwrap), args...)
	}
	cmd := exec.CommandContext(ctx, program, programArgs...)
	cmd.Env = environ(spec.Env)
	configureProcessGroup(cmd)
	return cmd, Cleanup(cancel), nil
}

// Args renders the bubblewrap arguments for a spec, without the command.
// Order matters: the read-only binds first, then the hidden containers
// overlaid with tmpfs, then the writable binds back on top.
func Args(spec Spec) []string {
	args := []string{
		"--die-with-parent",
		"--new-session",
		"--unshare-user",
		"--unshare-pid",
		"--unshare-ipc",
		"--unshare-uts",
		"--unshare-cgroup-try",
	}
	if spec.Network.Mode == NetworkOff {
		args = append(args, "--unshare-net")
	}
	args = append(args,
		"--proc", "/proc",
		"--dev", "/dev",
		"--tmpfs", "/tmp",
		"--tmpfs", "/dev/shm",
	)
	for _, dir := range SystemBinds {
		args = append(args, "--ro-bind-try", dir, dir)
	}
	args = append(args, "--ro-bind-try", "/sys", "/sys")
	for _, path := range EtcBinds {
		args = append(args, "--ro-bind-try", path, path)
	}
	for _, bind := range spec.ReadOnly {
		args = append(args, bindOp("--ro-bind", bind), bind.Path, bind.Path)
	}
	for _, hidden := range spec.Hidden {
		args = append(args, "--tmpfs", hidden)
	}
	for _, bind := range spec.Writable {
		args = append(args, bindOp("--bind", bind), bind.Path, bind.Path)
	}
	if spec.Workdir != "" {
		args = append(args, "--chdir", spec.Workdir)
	}
	return args
}

func bindOp(op string, bind Bind) string {
	if bind.Optional {
		return op + "-try"
	}
	return op
}

// ParentDirs lists the directories a path's executable lives in, for callers
// that bind a tool outside the system directories.
func ParentDir(path string) (string, bool) {
	dir := filepath.Dir(path)
	if dir == "" || dir == "." || dir == "/" {
		return "", false
	}
	return dir, true
}

func firstLines(output string, n int) string {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) > n {
		lines = lines[:n]
	}
	return strings.Join(lines, " | ")
}
