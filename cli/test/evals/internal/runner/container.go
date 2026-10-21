package runner

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/catalog"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/env"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/grade"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/ops"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/record"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/tools"
)

// EnsureSandboxImage builds testdata/sandbox-image once per Dockerfile hash
// and returns its tag.
func EnsureSandboxImage(ctx context.Context, evalsRoot string) (string, error) {
	dir := filepath.Join(evalsRoot, "testdata", "sandbox-image")
	raw, err := os.ReadFile(filepath.Join(dir, "Dockerfile"))
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	tag := "twevals-sandbox:" + hex.EncodeToString(sum[:])[:12]
	if exec.CommandContext(ctx, "docker", "image", "inspect", tag).Run() == nil {
		return tag, nil
	}
	cmd := exec.CommandContext(ctx, "docker", "build", "-q", "-t", tag, dir)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("docker build %s: %w\n%s", tag, err, out)
	}
	return tag, nil
}

// ContainerShell is the setup suite's shell: a real bash inside a sandbox
// container (toolchains and package registries, no secrets), with the
// workspace bind-mounted and the backend reachable on the host network.
// traceway invocations are recorded from a static parse of each command.
type ContainerShell struct {
	name    string
	env     []string
	timeout time.Duration
	// host runs commands on the host instead (oracle runs only, when
	// Docker is unavailable: the commands are the case's own).
	host bool
	dir  string
	path string
	// pgids are the host-mode process groups, killed on Close so a
	// background process the commands started cannot outlive the shell.
	mu     sync.Mutex
	pgids  []int
	closed bool
}

// workdir is where the repository is mounted for the shell.
func (c *ContainerShell) workdir() string {
	if c.host {
		return c.dir
	}
	return "/work"
}

func setupEnv(c *catalog.Case, inst *env.Instance, projects []env.Project, port int) ([]string, error) {
	if c.Setup == nil {
		return nil, nil
	}
	var out []string
	keys := make([]string, 0, len(c.Setup.Env))
	for k := range c.Setup.Env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v := c.Setup.Env[k]
		switch {
		case v == "@url":
			v = inst.Backend.URL
		case v == "@port":
			v = strconv.Itoa(port)
		case strings.HasPrefix(v, "@token:"):
			name := strings.TrimPrefix(v, "@token:")
			found := false
			for _, p := range projects {
				if p.Name == name {
					v, found = p.Token, true
				}
			}
			if !found {
				return nil, fmt.Errorf("setup.env %s names project %q, which the seed did not create", k, name)
			}
		}
		out = append(out, k+"="+v)
	}
	return out, nil
}

func startContainerShell(ctx context.Context, opt Options, work string, surf surface) (*ContainerShell, error) {
	if opt.SandboxImage == "" {
		if !opt.Oracle {
			return nil, fmt.Errorf("the setup suite needs Docker and the sandbox image (a model's commands never run on the host)")
		}
		bin := filepath.Join(filepath.Dir(work), ".twevals-bin")
		if err := os.MkdirAll(bin, 0o755); err != nil {
			return nil, err
		}
		_ = os.Remove(filepath.Join(bin, "traceway"))
		if err := os.Symlink(surf.cliBin, filepath.Join(bin, "traceway")); err != nil {
			return nil, err
		}
		home := filepath.Join(filepath.Dir(work), ".twevals-home")
		_ = os.MkdirAll(home, 0o755)
		return &ContainerShell{host: true, dir: work, timeout: 10 * time.Minute, env: surf.containerEnv,
			path: bin + string(os.PathListSeparator) + os.Getenv("PATH")}, nil
	}
	name := "twevals-" + filepath.Base(filepath.Dir(filepath.Dir(work))) + "-" + strconv.FormatInt(time.Now().UnixNano()%1e9, 36)
	args := []string{"run", "-d", "--rm", "--name", name, "--network", "host",
		"--user", fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid()),
		"-e", "HOME=/tmp/home", "-e", "GOPATH=/tmp/go", "-e", "GOCACHE=/tmp/gocache", "-e", "npm_config_cache=/tmp/npm",
		"-v", work + ":/work", "-v", surf.cliBin + ":/usr/local/bin/traceway:ro", "-w", "/work",
		opt.SandboxImage, "sh", "-c", "mkdir -p /tmp/home && sleep infinity"}
	if out, err := exec.CommandContext(ctx, "docker", args...).CombinedOutput(); err != nil {
		return nil, fmt.Errorf("docker run: %w: %s", err, out)
	}
	return &ContainerShell{name: name, timeout: 10 * time.Minute, env: surf.containerEnv}, nil
}

// Close removes the container (on the host: kills every process group the
// shell started). It may be called more than once.
func (c *ContainerShell) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return
	}
	c.closed = true
	if c.host {
		for _, g := range c.pgids {
			_ = syscall.Kill(-g, syscall.SIGKILL)
		}
		return
	}
	_ = exec.Command("docker", "rm", "-f", c.name).Run()
}

// Spec describes the tool.
func (c *ContainerShell) Spec() tools.Spec {
	return tools.Spec{Name: "shell", Description: "Run a bash command in the repository (the current directory) on the developer's machine. go, node, npm, python3, pip, git, curl and the traceway CLI are installed; package registries are reachable. Start long-running processes in the background (nohup ... &). Output is capped at 30000 characters.",
		Parameters: map[string]any{"type": "object", "properties": map[string]any{"command": map[string]any{"type": "string", "description": "The command line to run."}}, "required": []string{"command"}, "additionalProperties": false}}
}

// Exec runs a command in the container.
func (c *ContainerShell) Exec(ctx context.Context, cmdline string, timeout time.Duration) (string, int) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	var cmd *exec.Cmd
	if c.host {
		cmd = exec.CommandContext(ctx, "bash", "-c", cmdline)
		cmd.Dir = c.dir
		home := filepath.Join(filepath.Dir(c.dir), ".twevals-home")
		cmd.Env = append([]string{"HOME=" + home, "PATH=" + c.path, "TZ=UTC", "LANG=C.UTF-8", "GOFLAGS=-mod=mod", "GOTOOLCHAIN=local"}, c.env...)
		for _, k := range []string{"GOPATH", "GOMODCACHE", "GOCACHE", "GOPROXY"} {
			if v := os.Getenv(k); v != "" {
				cmd.Env = append(cmd.Env, k+"="+v)
			}
		}
		if os.Getenv("GOMODCACHE") == "" && os.Getenv("GOPATH") == "" {
			if h, err := os.UserHomeDir(); err == nil {
				cmd.Env = append(cmd.Env, "GOMODCACHE="+filepath.Join(h, "go", "pkg", "mod"))
			}
		}
		if os.Getenv("GOCACHE") == "" {
			if d, err := os.UserCacheDir(); err == nil {
				cmd.Env = append(cmd.Env, "GOCACHE="+filepath.Join(d, "go-build"))
			}
		}
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	} else {
		args := []string{"exec", "-i", "-w", "/work"}
		for _, e := range c.env {
			args = append(args, "-e", e)
		}
		args = append(args, c.name, "bash", "-c", cmdline)
		cmd = exec.CommandContext(ctx, "docker", args...)
	}
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	err := cmd.Start()
	if err == nil {
		if c.host {
			c.mu.Lock()
			c.pgids = append(c.pgids, cmd.Process.Pid)
			c.mu.Unlock()
		}
		err = cmd.Wait()
	}
	code := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else {
			code = 1
		}
		if ctx.Err() != nil {
			out.WriteString(fmt.Sprintf("\n[command timed out after %s]", timeout))
			code = 124
		}
	}
	return out.String(), code
}

// Call runs one command line.
func (c *ContainerShell) Call(ctx context.Context, raw json.RawMessage) tools.Result {
	var in struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal(raw, &in); err != nil || strings.TrimSpace(in.Command) == "" {
		return tools.Result{Output: "error: command is required"}
	}
	out, code := c.Exec(ctx, in.Command, c.timeout)
	if code != 0 {
		out += fmt.Sprintf("\n[exit status %d]", code)
	}
	capped, trunc := tools.Cap(out)
	return tools.Result{Output: capped, OK: code == 0, Truncated: trunc, Ops: staticOps(in.Command, code, out)}
}

// staticOps records the traceway invocations a command line contains.
func staticOps(cmdline string, code int, output string) []record.OpRecord {
	inv, err := ops.Invocations(cmdline)
	if err != nil {
		return nil
	}
	var out []record.OpRecord
	for _, words := range inv {
		if words[0] == "traceway" || words[0] == "traceway-write" {
			out = append(out, record.OpRecord{Op: ops.ParseCLI(words[1:]), Argv: words, Exit: code, OK: code == 0, Output: output})
		}
		// The setup skill's default plan path submits with curl; record it
		// as the same operation as `traceway setup apply`.
		if words[0] == "curl" && submitsSetupPlan(words) {
			out = append(out, record.OpRecord{Op: ops.Op{Op: "setup.apply", Args: map[string]any{"via": "curl"}}, Argv: words, Exit: code, OK: code == 0, Output: output})
		}
	}
	return out
}

func submitsSetupPlan(words []string) bool {
	hasPlan, isPut := false, false
	for i, w := range words {
		if strings.Contains(w, "/api/setup/plan") {
			hasPlan = true
		}
		if (w == "-X" || w == "--request") && i+1 < len(words) && strings.EqualFold(words[i+1], "PUT") || strings.EqualFold(w, "-XPUT") {
			isPut = true
		}
	}
	return hasPlan && isPut
}

// verifySetup builds and starts the instrumented app in a fresh container
// against the backend, then hits its routes, so the state grader can check
// that telemetry actually arrived.
func verifySetup(ctx context.Context, opt Options, c *catalog.Case, work string, inst *env.Instance, projects []env.Project, surf surface) *grade.Verdict {
	v := &grade.Verdict{Grader: "setup_verify", Gating: true, Pass: true}
	vs := c.Setup.Verify
	cs, err := startContainerShell(ctx, opt, work, surf)
	if err != nil {
		v.Pass, v.Failures = false, []string{err.Error()}
		return v
	}
	defer cs.Close()
	// The run's port sat unused while the agent worked, so a parallel run's
	// server may hold it by now; the app gets a fresh one.
	fresh, err := env.FreePort()
	if err != nil {
		v.Pass, v.Failures = false, []string{err.Error()}
		return v
	}
	cs.env = slices.Clone(cs.env)
	for i, e := range cs.env {
		if strings.HasPrefix(e, "PORT=") {
			cs.env[i] = "PORT=" + strconv.Itoa(fresh)
		}
	}
	logPath, pidPath := "/tmp/app.log", "/tmp/app.pid"
	if cs.host {
		logPath, pidPath = filepath.Join(filepath.Dir(work), "app.log"), filepath.Join(filepath.Dir(work), "app.pid")
	}
	defer func() {
		if cs.host {
			// Stop the app (its whole process group) we started on the host.
			_, _ = cs.Exec(context.Background(), "kill -- -$(cat "+pidPath+") 2>/dev/null || true", 10*time.Second)
		}
	}()
	cd := "cd " + cs.workdir()
	if vs.Dir != "" {
		cd += "/" + vs.Dir
	}
	source := ""
	for _, f := range vs.EnvFiles {
		source += fmt.Sprintf("if [ -f %s ]; then set -a; . ./%s; set +a; fi; ", f, f)
	}
	// The harness's variables (URL, token, PORT) win over an env file the
	// agent wrote: the readiness check polls PORT.
	if source != "" {
		for _, e := range cs.env {
			k, v, _ := strings.Cut(e, "=")
			source += "export " + k + "=" + shellQuoteArg(v) + "; "
		}
	}
	if vs.Build != "" {
		out, code := cs.Exec(ctx, cd+" && "+vs.Build, 10*time.Minute)
		if code != 0 {
			v.Pass = false
			v.Failures = append(v.Failures, "build failed: "+tail(out, 2000))
			return v
		}
		v.Notes = append(v.Notes, "build passed")
	}
	if vs.Start != "" {
		_, _ = cs.Exec(ctx, cd+" && "+source+"setsid nohup sh -c "+shellQuoteArg(vs.Start)+" > "+logPath+" 2>&1 < /dev/null & echo $! > "+pidPath, time.Minute)
		port := ""
		for _, e := range cs.env {
			if strings.HasPrefix(e, "PORT=") {
				port = strings.TrimPrefix(e, "PORT=")
			}
		}
		up := false
		for i := 0; i < 120 && !up; i++ {
			_, code := cs.Exec(ctx, "curl -s -o /dev/null http://127.0.0.1:"+port+"/", 10*time.Second)
			up = code == 0
			if !up {
				time.Sleep(time.Second)
			}
		}
		if !up {
			log, _ := cs.Exec(ctx, "cat "+logPath, 10*time.Second)
			v.Pass = false
			v.Failures = append(v.Failures, "the app never listened on :"+port+": "+tail(log, 2000))
			return v
		}
		for _, h := range vs.Hit {
			for k := 0; k < 5; k++ {
				_, _ = cs.Exec(ctx, "curl -s -o /dev/null http://127.0.0.1:"+port+h, 10*time.Second)
			}
		}
		wait := vs.Wait.D()
		if wait == 0 {
			wait = 12 * time.Second
		}
		time.Sleep(wait)
		log, _ := cs.Exec(ctx, "tail -c 3000 "+logPath, 10*time.Second)
		// Whatever answered on the port, the hits only count if they reached
		// the app started here.
		if _, code := cs.Exec(ctx, "kill -0 $(cat "+pidPath+")", 10*time.Second); code != 0 {
			v.Pass = false
			v.Failures = append(v.Failures, "the app exited before the check ended: "+tail(log, 2000))
			return v
		}
		v.Notes = append(v.Notes, "app log tail: "+tail(log, 1500))
	}
	return v
}

func shellQuoteArg(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }

func tail(s string, n int) string {
	if len(s) > n {
		return s[len(s)-n:]
	}
	return s
}
