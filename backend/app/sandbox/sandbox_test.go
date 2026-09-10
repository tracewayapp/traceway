package sandbox

import (
	"context"
	"errors"
	"os/exec"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
)

func agentSpec() Spec {
	return Spec{
		Command:  []string{"/usr/local/bin/claude", "-p"},
		Workdir:  "/var/agent/workspaces/attempt-1/repo",
		ReadOnly: []Bind{{Path: "/usr/local/bin"}, {Path: "/opt/traceway/skill", Optional: true}},
		Hidden:   []string{"/var/agent/workspaces"},
		Writable: []Bind{{Path: "/var/agent/workspaces/attempt-1"}},
		Env:      map[string]string{"HOME": "/var/agent/workspaces/attempt-1/home", "TRACEWAY_TOKEN": "run-token"},
		Limits:   Limits{WallClock: time.Minute},
		Network:  NetworkPolicy{Mode: NetworkAllow, Allow: []HostPort{{Host: "api.anthropic.com", Port: 443}}},
	}
}

func indexOfTriple(args []string, op, a, b string) int {
	for i := 0; i+2 < len(args); i++ {
		if args[i] == op && args[i+1] == a && args[i+2] == b {
			return i
		}
	}
	return -1
}

func TestArgsOrderReadOnlyHiddenWritable(t *testing.T) {
	args := Args(agentSpec())
	ro := indexOfTriple(args, "--ro-bind", "/usr/local/bin", "/usr/local/bin")
	optional := indexOfTriple(args, "--ro-bind-try", "/opt/traceway/skill", "/opt/traceway/skill")
	hidden := slices.Index(args, "/var/agent/workspaces")
	writable := indexOfTriple(args, "--bind", "/var/agent/workspaces/attempt-1", "/var/agent/workspaces/attempt-1")
	if ro < 0 || optional < 0 || hidden < 0 || writable < 0 {
		t.Fatalf("missing binds in %v", args)
	}
	if args[hidden-1] != "--tmpfs" {
		t.Fatalf("hidden directories must be overlaid with a tmpfs: %v", args)
	}
	if !(ro < hidden && hidden < writable) {
		t.Fatalf("read-only binds, then hidden tmpfs, then writable binds; got %d %d %d", ro, hidden, writable)
	}
	if last := len(args) - 2; args[last] != "--chdir" || args[last+1] != "/var/agent/workspaces/attempt-1/repo" {
		t.Fatalf("workdir must be the final chdir: %v", args[len(args)-2:])
	}
	for _, op := range []string{"--bind", "--ro-bind", "--ro-bind-try"} {
		if indexOfTriple(args, op, "/etc", "/etc") >= 0 {
			t.Fatalf("%s /etc must never be bound wholesale", op)
		}
	}
}

func TestArgsNetworkPolicy(t *testing.T) {
	shared := agentSpec()
	if slices.Contains(Args(shared), "--unshare-net") {
		t.Fatal("an allow list degrades to a shared network under bwrap, it must not unshare")
	}
	off := agentSpec()
	off.Network = NetworkPolicy{Mode: NetworkOff}
	if !slices.Contains(Args(off), "--unshare-net") {
		t.Fatal("network off must unshare the network namespace")
	}
}

func TestParseModeAndResolve(t *testing.T) {
	for input, want := range map[string]Mode{"": ModeAuto, "auto": ModeAuto, " Bwrap ": ModeBwrap, "off": ModeOff} {
		mode, err := ParseMode(input)
		if err != nil || mode != want {
			t.Errorf("ParseMode(%q) = %s, %v", input, mode, err)
		}
	}
	if _, err := ParseMode("podman"); !errors.Is(err, ErrUnknownMode) || !strings.Contains(err.Error(), "auto, bwrap, docker, or off") {
		t.Fatalf("ParseMode(podman) = %v", err)
	}
	if mode, err := ParseMode("docker"); err != nil || mode != ModeDocker {
		t.Fatalf("ParseMode(docker) = %v, %v", mode, err)
	}

	probe := Spec{Command: []string{"true"}}
	if _, err := Resolve(context.Background(), "docker", probe); err == nil || !strings.Contains(err.Error(), "agent runner") {
		t.Fatalf("Resolve(docker) = %v", err)
	}
	if mode, err := Resolve(context.Background(), "off", probe); err != nil || mode != ModeOff {
		t.Fatalf("off = %s, %v", mode, err)
	}
	if runtime.GOOS != "linux" {
		if mode, err := Resolve(context.Background(), "auto", probe); err != nil || mode != ModeOff {
			t.Fatalf("auto on %s must degrade to off, got %s, %v", runtime.GOOS, mode, err)
		}
		if _, err := Resolve(context.Background(), "bwrap", probe); err == nil || !strings.Contains(err.Error(), "Linux-only") {
			t.Fatalf("explicit bwrap on %s must fail fast, got %v", runtime.GOOS, err)
		}
	}
	if ForMode(ModeOff).Name() != "off" || ForMode(ModeBwrap).Name() != "bwrap" {
		t.Fatal("ForMode names")
	}
}

func TestOffBackendRunsWithSpecEnvWorkdirAndWallClock(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	dir := t.TempDir()
	spec := Spec{
		Command: []string{"sh", "-c", "pwd; echo $ONLY; echo ${HOME:-nohome}"},
		Workdir: dir,
		Env:     map[string]string{"ONLY": "allowlisted", "PATH": "/usr/bin:/bin", "bad=name": "x", "": "y"},
	}
	cmd, cleanup, err := (Off{}).Build(context.Background(), spec)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) != 3 || !strings.HasSuffix(lines[0], strings.TrimPrefix(dir, "/private")) || lines[1] != "allowlisted" || lines[2] != "nohome" {
		t.Fatalf("output = %q", out)
	}
	if len(cmd.Env) != 2 {
		t.Fatalf("malformed names must be dropped from the environment: %v", cmd.Env)
	}

	slow := Spec{Command: []string{"sh", "-c", "sleep 5; echo done"}, Limits: Limits{WallClock: 200 * time.Millisecond}}
	cmd, cleanup, err = (Off{}).Build(context.Background(), slow)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	started := time.Now()
	if out, err := cmd.CombinedOutput(); err == nil || strings.Contains(string(out), "done") {
		t.Fatalf("the wall clock limit must kill the process, got %v %q", err, out)
	}
	if time.Since(started) > 3*time.Second {
		t.Fatal("the kill took longer than the limit")
	}

	if _, _, err := (Off{}).Build(context.Background(), Spec{}); err == nil {
		t.Fatal("an empty command must be refused")
	}
}

func TestCgroupWrapperNeedsLimitsAndSystemd(t *testing.T) {
	if wrapper := cgroupWrapper(Limits{}); wrapper != nil {
		t.Fatalf("no limits means no wrapper, got %v", wrapper)
	}
	wrapper := cgroupWrapper(Limits{CPU: 1.5, MemoryMB: 2048, PIDs: 256})
	if _, err := exec.LookPath("systemd-run"); err != nil || runtime.GOOS != "linux" {
		if wrapper != nil {
			t.Fatalf("a host without systemd must run unlimited, got %v", wrapper)
		}
		return
	}
	if len(wrapper) == 0 {
		return
	}
	joined := strings.Join(wrapper, " ")
	for _, want := range []string{"CPUQuota=150%", "MemoryMax=2048M", "TasksMax=256"} {
		if !strings.Contains(joined, want) {
			t.Errorf("wrapper lacks %s: %s", want, joined)
		}
	}
}
