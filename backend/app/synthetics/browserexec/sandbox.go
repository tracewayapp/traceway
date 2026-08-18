package browserexec

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type Sandbox string

const (
	SandboxAuto  Sandbox = "auto"
	SandboxBwrap Sandbox = "bwrap"
	SandboxOff   Sandbox = "off"
)

const sandboxProbeTimeout = 15 * time.Second

var etcBinds = []string{
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

var systemBinds = []string{"/usr", "/bin", "/sbin", "/lib", "/lib32", "/lib64"}

func ResolveSandbox(configured string, harnessDir string) (Sandbox, error) {
	switch Sandbox(strings.ToLower(strings.TrimSpace(configured))) {
	case SandboxOff:
		return SandboxOff, nil
	case SandboxBwrap:
		if err := probeBwrap(harnessDir); err != nil {
			return SandboxOff, err
		}
		return SandboxBwrap, nil
	case "", SandboxAuto:
		if probeBwrap(harnessDir) != nil {
			return SandboxOff, nil
		}
		return SandboxBwrap, nil
	default:
		return SandboxOff, fmt.Errorf("unknown sandbox %q: expected auto, bwrap, or off", configured)
	}
}

func probeBwrap(harnessDir string) error {
	if runtime.GOOS != "linux" {
		return fmt.Errorf("bubblewrap is Linux-only (this host is %s)", runtime.GOOS)
	}
	bwrap, err := exec.LookPath("bwrap")
	if err != nil {
		return fmt.Errorf("bubblewrap (bwrap) is not on PATH: %w", err)
	}
	node, err := exec.LookPath("node")
	if err != nil {
		return fmt.Errorf("node is not on PATH: %w", err)
	}
	// Probe from under .runs, the same container real runs use, so the probe
	// exercises the sibling-hiding tmpfs too.
	runsDir := filepath.Join(harnessDir, ".runs")
	if err := os.MkdirAll(runsDir, 0o755); err != nil {
		return fmt.Errorf("failed to create the runs dir in the harness: %w", err)
	}
	probeDir, err := os.MkdirTemp(runsDir, ".probe-*")
	if err != nil {
		return fmt.Errorf("failed to create a probe dir in the harness: %w", err)
	}
	defer os.RemoveAll(probeDir)

	ctx, cancel := context.WithTimeout(context.Background(), sandboxProbeTimeout)
	defer cancel()

	args := append(bwrapArgs(commandSpec{
		NodePath:     node,
		HarnessDir:   harnessDir,
		RunDir:       probeDir,
		BrowsersPath: browsersPath(),
	}), node, "--version")
	output, err := exec.CommandContext(ctx, bwrap, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("the bubblewrap probe failed (%v): %s", err, firstOutputLines(string(output), 3))
	}
	return nil
}

type commandSpec struct {
	NodePath     string
	CLIPath      string
	ConfigPath   string
	HarnessDir   string
	RunDir       string
	BrowsersPath string
}

func buildCommand(ctx context.Context, sandbox Sandbox, spec commandSpec) (*exec.Cmd, error) {
	if sandbox != SandboxBwrap {
		cmd := exec.CommandContext(ctx, spec.NodePath, spec.CLIPath, "test", "--config", spec.ConfigPath)
		cmd.Dir = spec.HarnessDir
		return cmd, nil
	}
	bwrap, err := exec.LookPath("bwrap")
	if err != nil {
		return nil, fmt.Errorf("bubblewrap (bwrap) is not on PATH: %w", err)
	}
	args := append(bwrapArgs(spec), spec.NodePath, spec.CLIPath, "test", "--config", spec.ConfigPath)
	return exec.CommandContext(ctx, bwrap, args...), nil
}

func bwrapArgs(spec commandSpec) []string {
	args := []string{
		"--die-with-parent",
		"--new-session",
		"--unshare-user",
		"--unshare-pid",
		"--unshare-ipc",
		"--unshare-uts",
		"--unshare-cgroup-try",
		"--proc", "/proc",
		"--dev", "/dev",
		"--tmpfs", "/tmp",
		"--tmpfs", "/dev/shm",
	}
	for _, dir := range systemBinds {
		args = append(args, "--ro-bind-try", dir, dir)
	}
	args = append(args, "--ro-bind-try", "/sys", "/sys")
	for _, path := range etcBinds {
		args = append(args, "--ro-bind-try", path, path)
	}
	if dir := filepath.Dir(spec.NodePath); dir != "" && dir != "." && dir != "/" {
		args = append(args, "--ro-bind-try", dir, dir)
	}
	args = append(args, "--ro-bind", spec.HarnessDir, spec.HarnessDir)
	if spec.BrowsersPath != "" {
		args = append(args, "--ro-bind-try", spec.BrowsersPath, spec.BrowsersPath)
	}
	// The read-only harness bind also exposes concurrent sibling run dirs
	// under .runs. Overlay that container with an empty tmpfs, then bind back
	// only this run's dir. node_modules is above the container, so resolution
	// still works. Removing this line silently reintroduces a cross-run leak.
	args = append(args, "--tmpfs", filepath.Dir(spec.RunDir))
	args = append(args, "--bind", spec.RunDir, spec.RunDir)
	args = append(args, "--chdir", spec.RunDir)
	return args
}
