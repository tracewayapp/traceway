package sandbox

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/tracewayapp/traceway/backend/app/config"
)

// Docker confines the process in a container: every capability dropped,
// no new privileges, a read-only root filesystem with the spec's writable
// binds mounted read-write and a sized tmpfs for scratch, the caller's uid,
// cgroup limits natively, and a network the policy decides. An allow-list
// egress needs a per-run bridge network plus iptables rules in the host's
// DOCKER-USER chain, which only a root runner on Linux can install; the
// probe finds out and NetworkAllow fails closed otherwise.
type Docker struct {
	// Image is the fallback when the spec names none.
	Image string
	// Runtime is passed as --runtime when set (gVisor's runsc, for one).
	Runtime string
	// ScratchMB sizes the /tmp tmpfs; zero means 512.
	ScratchMB int

	probeOnce sync.Once
	egress    bool
}

const (
	dockerCommandTimeout = 30 * time.Second
	dockerNamePrefix     = "traceway-sandbox-"
	defaultScratchMB     = 512
)

func (*Docker) Name() string { return string(ModeDocker) }

// Probe runs the spec in the image and checks the host for what an egress
// allow-list needs.
func (d *Docker) Probe(ctx context.Context, spec Spec) error {
	if _, err := exec.LookPath("docker"); err != nil {
		return fmt.Errorf("docker is not on PATH: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, probeTimeout*4)
	defer cancel()
	if out, err := exec.CommandContext(ctx, "docker", "version", "--format", "{{.Server.Version}}").CombinedOutput(); err != nil {
		return fmt.Errorf("the docker daemon is unreachable (%v): %s", err, firstLines(string(out), 3))
	}
	if d.image(spec) == "" {
		return errors.New("no sandbox image: set AGENT_DEFAULT_IMAGE")
	}
	d.probeOnce.Do(func() {
		d.egress = runtime.GOOS == "linux" && os.Geteuid() == 0 && lookPathOk("iptables")
		if d.egress {
			d.egress = exec.CommandContext(ctx, "iptables", "-S", "DOCKER-USER").Run() == nil
		}
	})
	cmd, cleanup, err := d.Build(ctx, spec)
	if err != nil {
		return err
	}
	defer cleanup()
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("the docker probe failed (%v): %s", err, firstLines(string(output), 3))
	}
	return nil
}

func lookPathOk(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// EgressEnforced reports whether NetworkAllow is a real allow-list on this
// host, known after Probe.
func (d *Docker) EgressEnforced() bool { return d.egress }

func (d *Docker) image(spec Spec) string {
	if spec.Image != "" {
		return spec.Image
	}
	return d.Image
}

func (d *Docker) Build(ctx context.Context, spec Spec) (*exec.Cmd, Cleanup, error) {
	if len(spec.Command) == 0 {
		return nil, nil, errors.New("sandbox: spec has no command")
	}
	image := d.image(spec)
	if image == "" {
		return nil, nil, errors.New("sandbox: no image for the docker backend")
	}
	name := dockerNamePrefix + randomSuffix()
	scratch := d.ScratchMB
	if scratch <= 0 {
		scratch = defaultScratchMB
	}
	args := []string{
		"run", "--rm", "-i", "--name", name,
		"--cap-drop", "ALL",
		"--security-opt", "no-new-privileges",
		"--read-only",
		"--tmpfs", fmt.Sprintf("/tmp:rw,size=%dm", scratch),
		"--tmpfs", "/dev/shm:rw,size=64m",
		"--user", fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid()),
	}
	if spec.Workdir != "" {
		args = append(args, "-w", spec.Workdir)
	}
	for _, bind := range spec.ReadOnly {
		if bind.Optional && !pathExists(bind.Path) {
			continue
		}
		args = append(args, "-v", bind.Path+":"+bind.Path+":ro")
	}
	for _, bind := range spec.Writable {
		if bind.Optional && !pathExists(bind.Path) {
			continue
		}
		args = append(args, "-v", bind.Path+":"+bind.Path)
	}
	for _, pair := range environ(spec.Env) {
		args = append(args, "-e", pair)
	}
	if spec.Limits.CPU > 0 {
		args = append(args, "--cpus", strconv.FormatFloat(spec.Limits.CPU, 'f', -1, 64))
	}
	if spec.Limits.MemoryMB > 0 {
		args = append(args, "--memory", fmt.Sprintf("%dm", spec.Limits.MemoryMB))
	}
	if spec.Limits.PIDs > 0 {
		args = append(args, "--pids-limit", strconv.Itoa(spec.Limits.PIDs))
	}
	if d.Runtime != "" {
		args = append(args, "--runtime", d.Runtime)
	}

	var release func()
	switch spec.Network.Mode {
	case NetworkOff:
		args = append(args, "--network", "none")
	case NetworkAllow:
		if !d.egress {
			return nil, nil, errors.New("sandbox: Docker NetworkAllow requires a Linux host with root and iptables; refusing unrestricted networking")
		}
		{
			network, undo, err := d.allowlistNetwork(ctx, name, spec.Network.Allow)
			if err != nil {
				return nil, nil, err
			}
			release = undo
			args = append(args, "--network", network)
		}
	}
	args = append(args, "--entrypoint", spec.Command[0], image)
	args = append(args, spec.Command[1:]...)

	ctx, cancel := withWallClock(ctx, spec.Limits.WallClock)
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Env = append(os.Environ(), "DOCKER_CLI_HINTS=false")
	cmd.Cancel = func() error {
		kill := exec.Command("docker", "kill", name)
		_ = kill.Run()
		return cmd.Process.Kill()
	}
	cmd.WaitDelay = 10 * time.Second
	var once sync.Once
	cleanup := func() {
		once.Do(func() {
			cancel()
			remove := exec.Command("docker", "rm", "-f", name)
			_ = remove.Run()
			if release != nil {
				release()
			}
		})
	}
	return cmd, cleanup, nil
}

// allowlistNetwork creates a bridge for one run and installs DOCKER-USER
// rules so its subnet reaches only the allowed host:port pairs (and DNS).
func (d *Docker) allowlistNetwork(ctx context.Context, name string, allow []HostPort) (string, func(), error) {
	network := name + "-net"
	if out, err := dockerCommand(ctx, "network", "create", "--driver", "bridge", network); err != nil {
		return "", nil, fmt.Errorf("create sandbox network: %v: %s", err, out)
	}
	undoNetwork := func() {
		remove := exec.Command("docker", "network", "rm", network)
		_ = remove.Run()
	}
	subnet, err := dockerCommand(ctx, "network", "inspect", "-f", "{{(index .IPAM.Config 0).Subnet}}", network)
	if err != nil {
		undoNetwork()
		return "", nil, fmt.Errorf("inspect sandbox network: %v: %s", err, subnet)
	}
	subnet = strings.TrimSpace(subnet)

	var rules [][]string
	rules = append(rules, []string{"DOCKER-USER", "-s", subnet, "-j", "DROP"})
	// Traffic to the runner host traverses INPUT, not DOCKER-USER.
	rules = append(rules, []string{"INPUT", "-s", subnet, "-j", "DROP"})
	rules = append(rules, []string{"DOCKER-USER", "-s", subnet, "-p", "udp", "--dport", "53", "-j", "ACCEPT"})
	rules = append(rules, []string{"DOCKER-USER", "-s", subnet, "-p", "tcp", "--dport", "53", "-j", "ACCEPT"})
	for _, target := range allow {
		ips, err := net.DefaultResolver.LookupIP(ctx, "ip4", target.Host)
		if err != nil {
			config.Logf("sandbox: cannot resolve allowed host %s: %v", target.Host, err)
			continue
		}
		for _, ip := range ips {
			rules = append(rules, []string{"DOCKER-USER", "-s", subnet, "-d", ip.String(), "-p", "tcp", "--dport", strconv.Itoa(target.Port), "-j", "ACCEPT"})
			rules = append(rules, []string{"INPUT", "-s", subnet, "-d", ip.String(), "-p", "tcp", "--dport", strconv.Itoa(target.Port), "-j", "ACCEPT"})
		}
	}
	var installed [][]string
	undoRules := func() {
		for i := len(installed) - 1; i >= 0; i-- {
			remove := exec.Command("iptables", append([]string{"-D"}, installed[i]...)...)
			_ = remove.Run()
		}
	}
	for _, rule := range rules {
		insert := exec.CommandContext(ctx, "iptables", append([]string{"-I"}, rule...)...)
		if out, err := insert.CombinedOutput(); err != nil {
			undoRules()
			undoNetwork()
			return "", nil, fmt.Errorf("install egress rule %v: %v: %s", rule, err, strings.TrimSpace(string(out)))
		}
		installed = append(installed, rule)
	}
	return network, func() {
		undoRules()
		undoNetwork()
	}, nil
}

func dockerCommand(ctx context.Context, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, dockerCommandTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "docker", args...).CombinedOutput()
	return string(out), err
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func randomSuffix() string {
	var buf [6]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 36)
	}
	return hex.EncodeToString(buf[:])
}

var _ Backend = (*Docker)(nil)
