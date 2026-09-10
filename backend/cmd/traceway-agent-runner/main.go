// traceway-agent-runner executes fix-agent attempts for a Traceway instance
// running with AGENT_MODE=remote. It long-polls the backend over HTTPS
// (outbound only), runs the same harness the embedded executor runs, and
// reports events, blobs and the outcome back. The backend keeps every
// credential but the provider key and a short git credential: the runner
// never sees the database, SECRETS_KEY or another tenant's attempt.
//
// Configuration (environment):
//
//	TRACEWAY_URL              base URL of the Traceway instance (required)
//	AGENT_RUNNER_SECRET       the server's AGENT_RUNNER_SECRET (required)
//	AGENT_RUNNER_NAME         name this runner registers under (default: hostname)
//	AGENT_RUNNER_WORKERS      concurrent attempts (default 2, max 16)
//	AGENT_RUNNER_STATE        directory for mirrors and workspaces (default ./agent-runner)
//	AGENT_SANDBOX             auto | docker | bwrap | off (default auto: bwrap when the host can, else off)
//	AGENT_SANDBOX_ALLOW_OFF   "true" accepts an unconfined agent (development only)
//	AGENT_DEFAULT_IMAGE       docker sandbox image when the repository names none
//	AGENT_DOCKER_RUNTIME      optional docker --runtime (runsc for gVisor)
//	AGENT_SKILL_DIR           the traceway skill directory (default /opt/traceway/skill)
//	TRACEWAY_CLI              path of the traceway CLI for bwrap/off modes (default: on PATH)
//	AGENT_RUNNER_MONITORING   optional Traceway SDK connection string for the runner's own telemetry
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/tracewayapp/traceway/backend/app/agentrunner"
	"github.com/tracewayapp/traceway/backend/app/agentrunner/agents"
	"github.com/tracewayapp/traceway/backend/app/agentrunner/remote"
	"github.com/tracewayapp/traceway/backend/app/agentrunner/workspace"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/sandbox"
	traceway "go.tracewayapp.com"
)

const (
	version         = "1.0.0"
	pollRetryDelay  = 5 * time.Second
	defaultWorkers  = 2
	maxWorkers      = 16
	defaultSkillDir = "/opt/traceway/skill"
)

type runnerConfig struct {
	baseURL       string
	secret        string
	name          string
	workers       int
	stateDir      string
	sandbox       string
	allowOff      bool
	defaultImage  string
	dockerRuntime string
	skillDir      string
	cliPath       string
	monitoring    string
}

func main() {
	cfg := loadConfig()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	backend, mode, hostTools, err := resolveSandbox(ctx, cfg)
	if err != nil {
		log.Fatalf("AGENT_SANDBOX=%s is unusable: %v", cfg.sandbox, err)
	}
	if hostTools {
		if err := checkHostTools(ctx, &cfg); err != nil {
			log.Fatal(err)
		}
	}
	if _, err := exec.LookPath("git"); err != nil {
		log.Fatal("traceway-agent-runner needs git on PATH for mirrors, checkouts and pushes")
	}

	if cfg.monitoring != "" {
		if err := traceway.Init(cfg.monitoring, traceway.WithServerName(cfg.name), traceway.WithVersion(version)); err != nil {
			log.Printf("self-monitoring disabled: %v", err)
		} else {
			log.Printf("self-monitoring enabled (server name: %s)", cfg.name)
		}
	}

	client := remote.New(cfg.baseURL, cfg.secret, cfg.name, version, remote.Capabilities{
		SchemaVersion: agentrunner.ProtocolVersion,
		Agents:        []string{agents.ClaudeCodeName},
		Sandbox:       string(mode),
		Workers:       cfg.workers,
	})
	runner := &agentrunner.Runner{
		Claimer:     client,
		Reporter:    client,
		Backend:     backend,
		Mirrors:     &workspace.Mirrors{Root: filepath.Join(cfg.stateDir, "mirrors")},
		WorkRoot:    filepath.Join(cfg.stateDir, "work"),
		Agents:      map[string]agents.Agent{agents.ClaudeCodeName: agents.ClaudeCode{}},
		InstanceURL: cfg.baseURL,
		SkillDir:    cfg.skillDir,
		CLIPath:     cfg.cliPath,
		HostTools:   hostTools,
		Image:       cfg.defaultImage,
		Limits:      agentrunner.DefaultLimits,
	}

	log.Printf("traceway-agent-runner %s (%s): polling %s (workers: %d, sandbox: %s, state: %s)", version, cfg.name, cfg.baseURL, cfg.workers, mode, cfg.stateDir)
	var wg sync.WaitGroup
	for i := 0; i < cfg.workers; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			loop(ctx, client, runner, index)
		}(i + 1)
	}
	wg.Wait()
	log.Println("traceway-agent-runner stopped")
}

func loadConfig() runnerConfig {
	cfg := runnerConfig{
		baseURL:       strings.TrimRight(strings.TrimSpace(os.Getenv("TRACEWAY_URL")), "/"),
		secret:        os.Getenv("AGENT_RUNNER_SECRET"),
		name:          strings.TrimSpace(os.Getenv("AGENT_RUNNER_NAME")),
		workers:       defaultWorkers,
		stateDir:      strings.TrimSpace(os.Getenv("AGENT_RUNNER_STATE")),
		sandbox:       strings.TrimSpace(os.Getenv("AGENT_SANDBOX")),
		allowOff:      strings.EqualFold(strings.TrimSpace(os.Getenv("AGENT_SANDBOX_ALLOW_OFF")), "true"),
		defaultImage:  strings.TrimSpace(os.Getenv("AGENT_DEFAULT_IMAGE")),
		dockerRuntime: strings.TrimSpace(os.Getenv("AGENT_DOCKER_RUNTIME")),
		skillDir:      strings.TrimSpace(os.Getenv("AGENT_SKILL_DIR")),
		cliPath:       strings.TrimSpace(os.Getenv("TRACEWAY_CLI")),
		monitoring:    strings.TrimSpace(os.Getenv("AGENT_RUNNER_MONITORING")),
	}
	if cfg.baseURL == "" || cfg.secret == "" {
		log.Fatal("TRACEWAY_URL and AGENT_RUNNER_SECRET are required (the secret must match the server's AGENT_RUNNER_SECRET)")
	}
	if cfg.name == "" {
		if hostname, err := os.Hostname(); err == nil {
			cfg.name = hostname
		} else {
			cfg.name = "unnamed"
		}
	}
	if raw := os.Getenv("AGENT_RUNNER_WORKERS"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > maxWorkers {
			log.Fatalf("AGENT_RUNNER_WORKERS=%q: expected 1 to %d", raw, maxWorkers)
		}
		cfg.workers = parsed
	}
	if cfg.stateDir == "" {
		cfg.stateDir = "./agent-runner"
	}
	if cfg.skillDir == "" {
		cfg.skillDir = defaultSkillDir
	}
	return cfg
}

// resolveSandbox picks the backend: docker is probed with the default
// image, the process modes go through the shared resolver, and an
// unconfined agent needs the explicit opt-in the embedded executor also
// demands. It reports whether the agent's tools live on this host (bwrap,
// off) or in the sandbox image (docker).
func resolveSandbox(ctx context.Context, cfg runnerConfig) (sandbox.Backend, sandbox.Mode, bool, error) {
	mode, err := sandbox.ParseMode(cfg.sandbox)
	if err != nil {
		return nil, "", false, err
	}
	probe := sandbox.Spec{Command: []string{"/bin/sh", "-c", "true"}, Workdir: "/tmp"}
	if mode == sandbox.ModeDocker {
		docker := &sandbox.Docker{Image: cfg.defaultImage, Runtime: cfg.dockerRuntime}
		if err := docker.Probe(ctx, probe); err != nil {
			return nil, "", false, err
		}
		if !docker.EgressEnforced() {
			return nil, "", false, errors.New("Docker agent execution requires enforced egress: run on a Linux host with root and iptables")
		}
		return docker, sandbox.ModeDocker, false, nil
	}
	resolved, err := sandbox.Resolve(ctx, cfg.sandbox, probe)
	if err != nil {
		return nil, "", false, err
	}
	if resolved == sandbox.ModeOff && !cfg.allowOff {
		return nil, "", false, errors.New("bubblewrap is unavailable and AGENT_SANDBOX_ALLOW_OFF is not true; install bubblewrap, use AGENT_SANDBOX=docker, or accept an unconfined agent for development")
	}
	if resolved == sandbox.ModeOff {
		log.Print("WARNING: agent attempts run unconfined (AGENT_SANDBOX_ALLOW_OFF=true); never do this on a shared host")
	}
	return sandbox.ForMode(resolved), resolved, true, nil
}

func checkHostTools(ctx context.Context, cfg *runnerConfig) error {
	claudePath, err := exec.LookPath("claude")
	if err != nil {
		return errors.New("the Claude Code CLI (claude) is not on PATH; install it, or use AGENT_SANDBOX=docker with an image that carries it")
	}
	if out, err := exec.CommandContext(ctx, claudePath, "--version").CombinedOutput(); err != nil {
		return fmt.Errorf("claude --version failed: %v: %s", err, strings.TrimSpace(string(out)))
	}
	if cfg.cliPath == "" {
		cfg.cliPath, err = exec.LookPath("traceway")
		if err != nil {
			return errors.New("the traceway CLI is not on PATH; set TRACEWAY_CLI or install it")
		}
	}
	return nil
}

// loop is one worker: claim one attempt at a time through the long poll,
// run it, repeat. A rejected secret stops the process.
func loop(ctx context.Context, client *remote.Client, runner *agentrunner.Runner, index int) {
	for ctx.Err() == nil {
		claimed, err := client.Claim(ctx, 1)
		if errors.Is(err, remote.ErrUnauthorized) {
			log.Fatalf("worker %d: %v", index, err)
		}
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("worker %d: poll failed: %v (retrying in %s)", index, err, pollRetryDelay)
			select {
			case <-ctx.Done():
				return
			case <-time.After(pollRetryDelay):
			}
			continue
		}
		for _, attempt := range claimed {
			log.Printf("worker %d: running attempt %s (%s #%d)", index, attempt.Id, attempt.SubjectRef, attempt.Number)
			runGuarded(ctx, runner, attempt, index)
		}
	}
}

func runGuarded(ctx context.Context, runner *agentrunner.Runner, attempt *models.AgentAttempt, index int) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("worker %d: attempt %s panicked: %v", index, attempt.Id, r)
			traceway.CaptureException(fmt.Errorf("panic running attempt %s: %v", attempt.Id, r))
		}
	}()
	runner.Run(ctx, attempt)
}
