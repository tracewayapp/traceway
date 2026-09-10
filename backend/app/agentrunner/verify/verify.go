// Package verify runs the repository's test command against the agent's
// change, inside the sandbox with the network off, and returns what a
// reviewer would look at: whether it passed and the tail of its output.
package verify

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"time"

	"github.com/tracewayapp/traceway/backend/app/sandbox"
)

const (
	defaultTimeout = 20 * time.Minute
	outputTailSize = 8 << 10
)

type outputTail struct{ bytes.Buffer }

func (b *outputTail) Write(p []byte) (int, error) {
	n := len(p)
	if len(p) > outputTailSize {
		p = p[len(p)-outputTailSize:]
	}
	if b.Len()+len(p) > outputTailSize {
		b.Next(b.Len() + len(p) - outputTailSize)
	}
	_, err := b.Buffer.Write(p)
	return n, err
}

// Outcome is one test run.
type Outcome struct {
	Passed bool
	Output string
	// Skipped is set when the repository has no test command.
	Skipped bool
}

// Run executes the test command with the workspace writable (test suites
// write caches and build output) and every other bind read-only.
func Run(ctx context.Context, backend sandbox.Backend, image string, testCommand string, workspace string, readOnly []sandbox.Bind, env map[string]string, timeout time.Duration) (Outcome, error) {
	if strings.TrimSpace(testCommand) == "" {
		return Outcome{Passed: true, Skipped: true}, nil
	}
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	spec := sandbox.Spec{
		Image:    image,
		Command:  []string{"/bin/sh", "-lc", testCommand},
		Workdir:  workspace,
		ReadOnly: readOnly,
		Writable: []sandbox.Bind{{Path: workspace}},
		Env:      env,
		Limits:   sandbox.Limits{WallClock: timeout, CPU: 2, MemoryMB: 2048, PIDs: 256},
		Network:  sandbox.NetworkPolicy{Mode: sandbox.NetworkOff},
	}
	cmd, cleanup, err := backend.Build(ctx, spec)
	if err != nil {
		return Outcome{}, err
	}
	defer cleanup()
	var output outputTail
	cmd.Stdout = &output
	cmd.Stderr = &output
	cmd.WaitDelay = 10 * time.Second
	runErr := cmd.Run()
	tail := output.String()
	if len(tail) > outputTailSize {
		tail = tail[len(tail)-outputTailSize:]
	}
	if runErr == nil {
		return Outcome{Passed: true, Output: tail}, nil
	}
	var exitErr interface{ ExitCode() int }
	if errors.As(runErr, &exitErr) || ctx.Err() != nil || strings.Contains(runErr.Error(), "signal") {
		return Outcome{Passed: false, Output: tail}, nil
	}
	return Outcome{}, runErr
}
