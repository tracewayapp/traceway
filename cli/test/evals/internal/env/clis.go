package env

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// CLIHome is the isolated home of one CLI edition: its XDG dirs live outside
// the agent's workspace so the agent cannot read the stored token.
type CLIHome struct {
	Bin   string
	Home  string
	XDGCf string
	XDGSt string
}

// NewCLIHome prepares dirs under root for a binary.
func NewCLIHome(root, bin string) (*CLIHome, error) {
	h := &CLIHome{Bin: bin, Home: filepath.Join(root, "home"), XDGCf: filepath.Join(root, "xdg", "config"), XDGSt: filepath.Join(root, "xdg", "state")}
	for _, d := range []string{h.Home, h.XDGCf, h.XDGSt} {
		if err := os.MkdirAll(d, 0o700); err != nil {
			return nil, err
		}
	}
	return h, nil
}

// Env is the minimal environment every tool subprocess gets. Nothing from the
// harness's own environment (API keys, GitHub tokens, JWT secrets) is passed.
func (h *CLIHome) Env(path string) []string {
	return []string{
		"HOME=" + h.Home,
		"XDG_CONFIG_HOME=" + h.XDGCf,
		"XDG_STATE_HOME=" + h.XDGSt,
		"XDG_CACHE_HOME=" + filepath.Join(h.Home, ".cache"),
		"PATH=" + path,
		"TZ=UTC",
		"LANG=C.UTF-8",
	}
}

// Run executes the CLI with args and optional stdin.
func (h *CLIHome) Run(ctx context.Context, stdin string, args ...string) (string, string, error) {
	cmd := exec.CommandContext(ctx, h.Bin, args...)
	cmd.Env = h.Env(filepath.Dir(h.Bin))
	cmd.Dir = h.Home
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	var so, se bytes.Buffer
	cmd.Stdout, cmd.Stderr = &so, &se
	err := cmd.Run()
	return so.String(), se.String(), err
}

// Login stores a token for the instance and selects a project, the way a
// user would: `login --url U --token-stdin` then `projects use <id>`.
func (h *CLIHome) Login(ctx context.Context, url, token, projectID string) error {
	if so, se, err := h.Run(ctx, token+"\n", "login", "--url", url, "--token-stdin"); err != nil {
		return fmt.Errorf("%s login: %w\n%s%s", filepath.Base(h.Bin), err, so, se)
	}
	if projectID == "" {
		return nil
	}
	if so, se, err := h.Run(ctx, "", "projects", "use", projectID); err != nil {
		return fmt.Errorf("%s projects use: %w\n%s%s", filepath.Base(h.Bin), err, so, se)
	}
	return nil
}
