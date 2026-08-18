package browserexec

import (
	"context"
	"slices"
	"strings"
	"testing"
)

func testSpec() commandSpec {
	return commandSpec{
		NodePath:     "/usr/local/bin/node",
		CLIPath:      "/opt/harness/node_modules/@playwright/test/cli.js",
		ConfigPath:   "/opt/harness/.runs/run-1/traceway.config.cjs",
		HarnessDir:   "/opt/harness",
		RunDir:       "/opt/harness/.runs/run-1",
		BrowsersPath: "/opt/ms-playwright",
	}
}

func indexOfBind(args []string, op, src string) int {
	for i := 0; i+2 < len(args); i++ {
		if args[i] == op && args[i+1] == src && args[i+2] == src {
			return i
		}
	}
	return -1
}

func indexOfSingle(args []string, op, val string) int {
	for i := 0; i+1 < len(args); i++ {
		if args[i] == op && args[i+1] == val {
			return i
		}
	}
	return -1
}

func TestBwrapArgsBindsHarnessReadOnlyBeforeRunDir(t *testing.T) {
	args := bwrapArgs(testSpec())

	harness := indexOfBind(args, "--ro-bind", "/opt/harness")
	if harness < 0 {
		t.Fatalf("the harness is not bound read-only: %v", args)
	}
	runDir := indexOfBind(args, "--bind", "/opt/harness/.runs/run-1")
	if runDir < 0 {
		t.Fatalf("the run dir is not bound writable: %v", args)
	}
	if harness > runDir {
		t.Fatalf("the read-only harness bind must precede the run dir bind, got %d and %d", harness, runDir)
	}
	if indexOfBind(args, "--bind", "/opt/harness") >= 0 {
		t.Fatal("the harness must never be bound writable: a spec could poison node_modules for the next tenant")
	}
}

func TestBwrapArgsHidesSiblingRunDirsWithTmpfs(t *testing.T) {
	args := bwrapArgs(testSpec())

	// testSpec's run dir is /opt/harness/.runs/run-1, so its container is
	// /opt/harness/.runs. That container must be overlaid with an empty tmpfs
	// so a concurrent sibling run dir under it is not exposed by the harness
	// read-only bind.
	tmpfs := indexOfSingle(args, "--tmpfs", "/opt/harness/.runs")
	if tmpfs < 0 {
		t.Fatalf("the run-dir container must be overlaid with a tmpfs to hide sibling runs: %v", args)
	}
	harness := indexOfBind(args, "--ro-bind", "/opt/harness")
	runDir := indexOfBind(args, "--bind", "/opt/harness/.runs/run-1")
	if harness < 0 || runDir < 0 {
		t.Fatalf("expected both the harness ro-bind and the run-dir bind: %v", args)
	}
	if !(harness < tmpfs && tmpfs < runDir) {
		t.Fatalf("the tmpfs must come after the harness ro-bind and before the run-dir bind, got harness=%d tmpfs=%d runDir=%d", harness, tmpfs, runDir)
	}
	// The tmpfs must never land on the harness root itself: that would bury
	// node_modules and break module resolution.
	if indexOfSingle(args, "--tmpfs", "/opt/harness") >= 0 {
		t.Fatal("the harness root must never be overlaid with a tmpfs: it holds node_modules")
	}
}

func TestBwrapArgsNeverBindsEtcWholesale(t *testing.T) {
	args := bwrapArgs(testSpec())

	for _, op := range []string{"--bind", "--ro-bind", "--ro-bind-try", "--dev-bind"} {
		if indexOfBind(args, op, "/etc") >= 0 {
			t.Fatalf("%s /etc exposes the unit file and EnvironmentFile holding the runner secret", op)
		}
	}
	if indexOfBind(args, "--ro-bind-try", "/etc/resolv.conf") < 0 {
		t.Fatal("DNS resolution needs /etc/resolv.conf bound")
	}
}

func TestBwrapArgsIsolatesProcessesAndKeepsNetwork(t *testing.T) {
	args := bwrapArgs(testSpec())

	if !slices.Contains(args, "--unshare-pid") {
		t.Error("expected --unshare-pid")
	}
	if indexOfBind(args, "--proc", "/proc") < 0 && !slices.Contains(args, "--proc") {
		t.Error("expected a fresh /proc")
	}
	if !slices.Contains(args, "--die-with-parent") {
		t.Error("expected --die-with-parent so a killed run leaves nothing behind")
	}
	if slices.Contains(args, "--unshare-net") {
		t.Error("the network namespace must stay shared")
	}
}

func TestBwrapArgsBindsNodeAndBrowsers(t *testing.T) {
	args := bwrapArgs(testSpec())

	if indexOfBind(args, "--ro-bind-try", "/usr/local/bin") < 0 {
		t.Error("node's directory must be bound: it is often outside the system dirs")
	}
	if indexOfBind(args, "--ro-bind-try", "/opt/ms-playwright") < 0 {
		t.Error("the browser registry must be bound, HOME is sandboxed away from it")
	}
}

func TestBuildCommandOffSpawnsNodeDirectly(t *testing.T) {
	spec := testSpec()
	cmd, err := buildCommand(context.Background(), SandboxOff, spec)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if cmd.Path != spec.NodePath {
		t.Errorf("expected node to be spawned directly, got %q", cmd.Path)
	}
	if cmd.Dir != spec.HarnessDir {
		t.Errorf("expected the harness as cwd, got %q", cmd.Dir)
	}
	if !slices.Contains(cmd.Args, spec.ConfigPath) {
		t.Errorf("expected the generated config in the argv, got %v", cmd.Args)
	}
}

func TestResolveSandboxOff(t *testing.T) {
	sandbox, err := ResolveSandbox("off", t.TempDir())
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if sandbox != SandboxOff {
		t.Errorf("expected off, got %q", sandbox)
	}
}

func TestResolveSandboxUnknownValue(t *testing.T) {
	_, err := ResolveSandbox("docker", t.TempDir())
	if err == nil {
		t.Fatal("expected an error for an unknown sandbox")
	}
	if !strings.Contains(err.Error(), "auto, bwrap, or off") {
		t.Errorf("expected the error to list the valid modes, got %q", err)
	}
}

func TestResolveSandboxAutoNeverErrors(t *testing.T) {
	if _, err := ResolveSandbox("", t.TempDir()); err != nil {
		t.Fatalf("empty: %v", err)
	}
	if _, err := ResolveSandbox("auto", t.TempDir()); err != nil {
		t.Fatalf("auto: %v", err)
	}
}
