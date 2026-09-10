package browserexec

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/tracewayapp/traceway/backend/app/sandbox"
)

type Sandbox = sandbox.Mode

const (
	SandboxAuto  = sandbox.ModeAuto
	SandboxBwrap = sandbox.ModeBwrap
	SandboxOff   = sandbox.ModeOff
)

// ResolveSandbox keeps the auto | bwrap | off semantics for the browser
// executors: the probe runs node --version through the exact bind set a
// real spec gets, from under .runs like a real run, so it exercises the
// sibling-hiding tmpfs too.
func ResolveSandbox(configured string, harnessDir string) (Sandbox, error) {
	mode, err := sandbox.ParseMode(configured)
	if err != nil {
		return SandboxOff, err
	}
	if mode == sandbox.ModeDocker {
		return SandboxOff, fmt.Errorf("the docker sandbox is only available to the agent runner; browser checks use auto, bwrap, or off")
	}
	if mode == SandboxOff {
		return SandboxOff, nil
	}
	if runtime.GOOS != "linux" {
		if mode == SandboxBwrap {
			return SandboxOff, fmt.Errorf("bubblewrap is Linux-only (this host is %s)", runtime.GOOS)
		}
		return SandboxOff, nil
	}
	node, err := exec.LookPath("node")
	if err != nil {
		if mode == SandboxBwrap {
			return SandboxOff, fmt.Errorf("node is not on PATH: %w", err)
		}
		return SandboxOff, nil
	}
	runsDir := filepath.Join(harnessDir, ".runs")
	if err := os.MkdirAll(runsDir, 0o755); err != nil {
		return SandboxOff, fmt.Errorf("failed to create the runs dir in the harness: %w", err)
	}
	probeDir, err := os.MkdirTemp(runsDir, ".probe-*")
	if err != nil {
		return SandboxOff, fmt.Errorf("failed to create a probe dir in the harness: %w", err)
	}
	defer os.RemoveAll(probeDir)

	probe := specFor(commandSpec{NodePath: node, HarnessDir: harnessDir, RunDir: probeDir, BrowsersPath: browsersPath()})
	probe.Command = []string{node, "--version"}
	return sandbox.Resolve(context.Background(), configured, probe)
}

type commandSpec struct {
	NodePath     string
	CLIPath      string
	ConfigPath   string
	HarnessDir   string
	RunDir       string
	BrowsersPath string
	Env          map[string]string
}

// specFor is the confinement of one Playwright run: the harness read-only,
// the run's own dir writable, the sibling run dirs hidden behind a tmpfs
// over their container, node and the browser registry visible, network
// shared since probes must reach the internet.
func specFor(spec commandSpec) sandbox.Spec {
	readOnly := []sandbox.Bind{}
	if dir, ok := sandbox.ParentDir(spec.NodePath); ok {
		readOnly = append(readOnly, sandbox.Bind{Path: dir, Optional: true})
	}
	readOnly = append(readOnly, sandbox.Bind{Path: spec.HarnessDir})
	if spec.BrowsersPath != "" {
		readOnly = append(readOnly, sandbox.Bind{Path: spec.BrowsersPath, Optional: true})
	}
	return sandbox.Spec{
		Command:  []string{spec.NodePath, spec.CLIPath, "test", "--config", spec.ConfigPath},
		Workdir:  spec.RunDir,
		ReadOnly: readOnly,
		// The read-only harness bind also exposes concurrent sibling run
		// dirs under .runs. Hiding that container behind an empty tmpfs and
		// binding back only this run's dir is what keeps runs apart;
		// node_modules is above the container, so resolution still works.
		Hidden:   []string{filepath.Dir(spec.RunDir)},
		Writable: []sandbox.Bind{{Path: spec.RunDir}},
		Env:      spec.Env,
		Network:  sandbox.NetworkPolicy{Mode: sandbox.NetworkShared},
	}
}

func buildCommand(ctx context.Context, mode Sandbox, spec commandSpec) (*exec.Cmd, sandbox.Cleanup, error) {
	return sandbox.ForMode(mode).Build(ctx, specFor(spec))
}
