//go:build smoke

package smoke

import (
	"bufio"
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// The mcp tests need no live instance: fail-fast happens before any network
// call, and the stdio handshake never touches the API.

func TestMcp_failsFastWithoutLogin(t *testing.T) {
	freshXDG(t)
	t.Setenv("TRACEWAY_URL", "")
	t.Setenv("TRACEWAY_TOKEN", "")

	_, stderr, code := runCLI(t, "", "mcp")
	if code != 4 {
		t.Errorf("exit = %d, want 4 (auth)", code)
	}
	if !strings.Contains(stderr, "traceway login") || !strings.Contains(stderr, "reconnect") {
		t.Errorf("stderr should tell the user to log in and reconnect: %q", stderr)
	}
}

func TestMcp_envVarsMustComeTogether(t *testing.T) {
	freshXDG(t)
	t.Setenv("TRACEWAY_URL", "http://localhost:1")
	t.Setenv("TRACEWAY_TOKEN", "")

	_, stderr, code := runCLI(t, "", "mcp")
	if code != 4 {
		t.Errorf("exit = %d, want 4", code)
	}
	if !strings.Contains(stderr, "must be set together") {
		t.Errorf("stderr = %q", stderr)
	}
}

func TestMcp_stdioHandshake(t *testing.T) {
	freshXDG(t)
	t.Setenv("TRACEWAY_URL", "http://localhost:1")
	t.Setenv("TRACEWAY_TOKEN", "twp_dummy")

	cmd := exec.Command(binaryPath, "mcp")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cmd.Process.Kill() }()

	fmt.Fprintln(stdin, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"smoke","version":"0"}}}`)

	line := make(chan string, 1)
	go func() {
		sc := bufio.NewScanner(stdout)
		sc.Buffer(make([]byte, 1024*1024), 1024*1024)
		if sc.Scan() {
			line <- sc.Text()
		}
		close(line)
	}()

	select {
	case resp := <-line:
		if !strings.Contains(resp, `"name":"traceway"`) {
			t.Errorf("initialize response missing server info: %s", resp)
		}
		if !strings.Contains(resp, "Ground rules") {
			t.Errorf("initialize response missing instructions: %s", resp)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("no initialize response within 10s")
	}

	_ = stdin.Close()
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("mcp should exit cleanly on client disconnect, got: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("mcp did not exit after stdin closed")
	}
}
