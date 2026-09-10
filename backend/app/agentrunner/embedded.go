package agentrunner

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/tracewayapp/traceway/backend/app/agent"
	"github.com/tracewayapp/traceway/backend/app/agentrunner/agents"
	"github.com/tracewayapp/traceway/backend/app/agentrunner/workspace"
	"github.com/tracewayapp/traceway/backend/app/config"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
	"github.com/tracewayapp/traceway/backend/app/sandbox"
	traceway "go.tracewayapp.com"
)

const (
	ModeOff      = "off"
	ModeEmbedded = "embedded"
	ModeRemote   = "remote"

	ExecutorEmbedded = "embedded"
	ExecutorRemote   = "remote"

	// RunnerExecutorPrefix prefixes a remote runner's name to make its
	// claim identity, so attempts show which runner holds them.
	RunnerExecutorPrefix = "runner:"

	// RunnerOnlineWindow is how recently a runner must have polled to count
	// as online in operational health metrics.
	RunnerOnlineWindow = agent.RunnerOnlineWindow

	defaultWorkers = 2
	maxWorkers     = 16
	pollInterval   = 5 * time.Second
)

// EmbeddedOptions is what the backend knows when it starts the harness in
// its own process.
type EmbeddedOptions struct {
	InstanceURL string
	StoragePath string
	SkillDir    string
	CLIPath     string
}

// Mode is the executor the deployment runs, from AGENT_MODE.
func Mode() (string, error) {
	switch mode := strings.ToLower(strings.TrimSpace(config.Config.AgentMode)); mode {
	case "", ModeOff:
		return ModeOff, nil
	case ModeEmbedded, ModeRemote:
		return mode, nil
	default:
		return "", fmt.Errorf("AGENT_MODE=%q: expected off, embedded, or remote", config.Config.AgentMode)
	}
}

// StartEmbedded validates the host and starts AGENT_WORKERS goroutines that
// claim attempts from the queue and run them in-process. Every requirement
// is checked up front so a misconfigured image fails at boot rather than
// on the first attempt.
func StartEmbedded(ctx context.Context, opts EmbeddedOptions) error {
	if _, err := exec.LookPath("git"); err != nil {
		return errors.New("AGENT_MODE=embedded needs git on PATH")
	}
	claudePath, err := exec.LookPath("claude")
	if err != nil {
		return errors.New("AGENT_MODE=embedded needs the Claude Code CLI (claude) on PATH; the :agent image carries it")
	}
	if out, err := exec.CommandContext(ctx, claudePath, "--version").CombinedOutput(); err != nil {
		return fmt.Errorf("claude --version failed: %v: %s", err, strings.TrimSpace(string(out)))
	}
	cliPath := opts.CLIPath
	if cliPath == "" {
		cliPath, err = exec.LookPath("traceway")
		if err != nil {
			return errors.New("AGENT_MODE=embedded needs the traceway CLI on PATH for the agent to read telemetry with")
		}
	}

	probe := sandbox.Spec{Command: []string{"/bin/sh", "-c", "true"}, Workdir: "/tmp"}
	mode, err := sandbox.Resolve(ctx, config.Config.AgentSandbox, probe)
	if err != nil {
		return fmt.Errorf("AGENT_SANDBOX=%s is unusable: %w. Install bubblewrap or set AGENT_SANDBOX=off with AGENT_SANDBOX_ALLOW_OFF=true (dev only)", config.Config.AgentSandbox, err)
	}
	if mode == sandbox.ModeOff && config.Config.AgentSandboxAllowOff != "true" {
		return errors.New("AGENT_MODE=embedded refuses to run the agent unconfined: bubblewrap is unavailable and AGENT_SANDBOX_ALLOW_OFF is not true. The agent process would see this server's environment and every other attempt's workspace")
	}
	if mode == sandbox.ModeOff {
		config.Logf("WARNING: agent attempts run unconfined (AGENT_SANDBOX_ALLOW_OFF=true); never do this on a shared instance")
	}

	workers := defaultWorkers
	if raw := strings.TrimSpace(config.Config.AgentWorkers); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > maxWorkers {
			return fmt.Errorf("AGENT_WORKERS=%q: expected 1 to %d", raw, maxWorkers)
		}
		workers = parsed
	}

	control := Local{Executor: ExecutorEmbedded, InstanceURL: opts.InstanceURL}
	runner := &Runner{
		Claimer:     control,
		Reporter:    control,
		Backend:     sandbox.ForMode(mode),
		Mirrors:     &workspace.Mirrors{Root: filepath.Join(opts.StoragePath, "agent", "mirrors")},
		WorkRoot:    filepath.Join(opts.StoragePath, "agent", "work"),
		Agents:      map[string]agents.Agent{agents.ClaudeCodeName: agents.ClaudeCode{}},
		InstanceURL: opts.InstanceURL,
		SkillDir:    opts.SkillDir,
		CLIPath:     cliPath,
		HostTools:   true,
		Limits:      DefaultLimits,
	}
	executor := &embeddedExecutor{workers: workers, sandbox: mode}
	agent.RegisterExecutor(executor)
	for i := 0; i < workers; i++ {
		go executor.loop(ctx, runner, i+1)
	}
	config.Logf("agent: embedded executor started (workers: %d, sandbox: %s, mirrors: %s)", workers, mode, runner.Mirrors.Root)
	return nil
}

type embeddedExecutor struct {
	workers int
	sandbox sandbox.Mode
}

func (e *embeddedExecutor) Name() string { return ExecutorEmbedded }

func (e *embeddedExecutor) Available(context.Context, *sql.Tx) (bool, string) {
	return true, fmt.Sprintf("embedded harness, %d worker(s), sandbox %s", e.workers, e.sandbox)
}

// loop is one worker: claim one attempt at a time, run it, repeat, waking
// on the queue signal so a click starts within a second.
func (e *embeddedExecutor) loop(ctx context.Context, runner *Runner, index int) {
	defer traceway.Recover()
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	for {
		claimed, err := runner.Claimer.Claim(ctx, 1)
		if err != nil {
			traceway.CaptureException(fmt.Errorf("agent: worker %d claim: %w", index, err))
		}
		for _, attempt := range claimed {
			runner.Run(ctx, attempt)
		}
		if len(claimed) > 0 {
			continue
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		case <-agent.WakeChannel():
		}
	}
}

// RemoteExecutor is the backend's view of a runner fleet in AGENT_MODE=remote:
// nothing runs in-process. Availability describes current fleet health;
// the Fix it menu and preflight use deployment configuration instead.
type RemoteExecutor struct{}

func (RemoteExecutor) Name() string { return ExecutorRemote }

func (RemoteExecutor) Available(ctx context.Context, tx *sql.Tx) (bool, string) {
	online, err := transactional.AgentRunnerRepository.CountOnline(tx, time.Now().UTC().Add(-RunnerOnlineWindow))
	if err != nil {
		traceway.CaptureException(fmt.Errorf("agent: count online runners: %w", err))
		return false, "could not count runners"
	}
	if online == 0 {
		return false, "no agent runner has polled in the last two minutes"
	}
	return true, fmt.Sprintf("%d runner(s) online", online)
}
