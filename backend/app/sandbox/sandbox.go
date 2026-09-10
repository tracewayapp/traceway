// Package sandbox confines a child process: the Playwright runs of synthetic
// browser checks and the coding agent of a fix attempt both go through it.
// A Backend turns a Spec into an exec.Cmd; bwrap is the backend for the
// embedded executors (a container cannot start sibling containers), and
// docker joins it for remote runners. Resolve keeps the auto | bwrap | off
// semantics of SYNTHETICS_BROWSER_SANDBOX for every caller.
package sandbox

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type Mode string

const (
	ModeAuto   Mode = "auto"
	ModeBwrap  Mode = "bwrap"
	ModeDocker Mode = "docker"
	ModeOff    Mode = "off"
)

// Bind maps a host path into the sandbox at the same path.
type Bind struct {
	Path string
	// Optional binds are skipped when the path does not exist on the host
	// (bwrap --ro-bind-try); a required one fails the start.
	Optional bool
}

// NetworkPolicy says what the sandboxed process may reach. bwrap can only
// unshare the network entirely or leave it shared; the Allow list is
// enforced by the docker backend and is documented as best-effort under
// bwrap, where it degrades to Shared.
type NetworkPolicy struct {
	Mode  NetworkMode
	Allow []HostPort
}

type NetworkMode string

const (
	NetworkShared NetworkMode = "shared"
	NetworkOff    NetworkMode = "off"
	NetworkAllow  NetworkMode = "allow"
)

type HostPort struct {
	Host string
	Port int
}

// Limits bound the process. Zero means unlimited. WallClock is enforced by
// every backend through the command's context; cpu, memory and pids need a
// cgroup, which bwrap gets from systemd-run when the host runs systemd and
// docker gets natively.
type Limits struct {
	CPU       float64
	MemoryMB  int
	PIDs      int
	WallClock time.Duration
}

// Spec is one confined command.
type Spec struct {
	Command []string
	Workdir string
	// Image is the container image the docker backend runs the command in;
	// the process backends ignore it.
	Image string
	// ReadOnly binds are visible but immutable. Writable binds are the only
	// places the process may write besides /tmp and /dev/shm. Hidden
	// directories are overlaid with an empty tmpfs before the writable binds
	// are applied, which is how one run's workspace stays invisible to a
	// sibling run under the same parent directory.
	ReadOnly []Bind
	Writable []Bind
	Hidden   []string
	// Env is the complete environment of the process; nothing is inherited.
	Env     map[string]string
	Limits  Limits
	Network NetworkPolicy
}

// Cleanup releases what Build allocated (nothing for bwrap, a container and
// a network for docker). It is safe to call more than once.
type Cleanup func()

type Backend interface {
	Name() string
	// Probe runs the spec through the real confinement and reports why the
	// host cannot sandbox, if it cannot.
	Probe(ctx context.Context, spec Spec) error
	Build(ctx context.Context, spec Spec) (*exec.Cmd, Cleanup, error)
}

var ErrUnknownMode = errors.New("unknown sandbox mode")

// ParseMode accepts auto, bwrap, docker and off; empty means auto. docker
// is a runner-only mode: Resolve does not probe it, the runner builds the
// Docker backend and probes it itself.
func ParseMode(configured string) (Mode, error) {
	switch Mode(strings.ToLower(strings.TrimSpace(configured))) {
	case ModeOff:
		return ModeOff, nil
	case ModeBwrap:
		return ModeBwrap, nil
	case ModeDocker:
		return ModeDocker, nil
	case "", ModeAuto:
		return ModeAuto, nil
	default:
		return ModeOff, fmt.Errorf("%w %q: expected auto, bwrap, docker, or off", ErrUnknownMode, configured)
	}
}

// Resolve decides the mode in effect: off stays off, bwrap must pass the
// probe or the error is returned, auto degrades to off when the probe
// fails. The probe spec is what the caller will actually run, so the probe
// exercises the real bind set.
func Resolve(ctx context.Context, configured string, probe Spec) (Mode, error) {
	mode, err := ParseMode(configured)
	if err != nil {
		return ModeOff, err
	}
	if mode == ModeOff {
		return ModeOff, nil
	}
	if mode == ModeDocker {
		return ModeOff, errors.New("the docker sandbox is only available to the agent runner")
	}
	if err := (Bwrap{}).Probe(ctx, probe); err != nil {
		if mode == ModeBwrap {
			return ModeOff, err
		}
		return ModeOff, nil
	}
	return ModeBwrap, nil
}

// ForMode returns the backend of a resolved mode.
func ForMode(mode Mode) Backend {
	if mode == ModeBwrap {
		return Bwrap{}
	}
	return Off{}
}

// Off runs the command unconfined, with the spec's environment and working
// directory. It is what a host without bubblewrap gets under auto.
type Off struct{}

func (Off) Name() string { return string(ModeOff) }

func (Off) Probe(context.Context, Spec) error { return nil }

func (Off) Build(ctx context.Context, spec Spec) (*exec.Cmd, Cleanup, error) {
	if len(spec.Command) == 0 {
		return nil, nil, errors.New("sandbox: spec has no command")
	}
	ctx, cancel := withWallClock(ctx, spec.Limits.WallClock)
	cmd := exec.CommandContext(ctx, spec.Command[0], spec.Command[1:]...)
	cmd.Dir = spec.Workdir
	cmd.Env = environ(spec.Env)
	configureProcessGroup(cmd)
	return cmd, Cleanup(cancel), nil
}

func withWallClock(ctx context.Context, limit time.Duration) (context.Context, context.CancelFunc) {
	if limit <= 0 {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, limit)
}

func environ(env map[string]string) []string {
	out := make([]string, 0, len(env))
	for name, value := range env {
		name = strings.TrimSpace(name)
		if name == "" || strings.ContainsAny(name, "=\x00") {
			continue
		}
		out = append(out, name+"="+value)
	}
	return out
}
