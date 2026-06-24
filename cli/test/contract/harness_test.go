// Package contract boots the Traceway backend in SQLite mode, seeds telemetry
// fixtures, and asserts that every endpoint the CLI calls keeps a stable wire
// contract. Unlike the unit tests (which run against hand-written httptest
// mocks that encode the CLI's own assumptions), this exercises the real server,
// so a backend field rename, casing change, or route change is caught here.
//
// It is a separate Go module (see go.mod) so the main cli module's lint/vuln
// graph stays free of the backend's dependency set. Run it with:
//
//	cd cli/test/contract && go test ./...
//
// Regenerate the golden wire-shape fingerprints after an intentional contract
// change with:
//
//	go test ./... -update
package contract

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	tracewaybackend "github.com/tracewayapp/traceway/backend"
	client "github.com/tracewayapp/traceway/cli/pkg/client"
)

const (
	seedEmail    = "contract@example.com"
	seedPassword = "contract-password-123"
	seedProject  = "Contract"
	seedToken    = "contract-project-token"
)

var update = flag.Bool("update", false, "update golden wire-shape fingerprints in testdata/")

var (
	baseURL   string    // http://127.0.0.1:<port> (no /api suffix)
	jwtToken  string    // app JWT for the seeded user
	projectID string    // UUID of the seeded project
	cliBin    string    // path to the built traceway binary
	xdgConfig string    // XDG_CONFIG_HOME for the CLI under test
	xdgState  string    // XDG_STATE_HOME for the CLI under test
	seedAt    time.Time // reference instant telemetry is seeded around
)

func TestMain(m *testing.M) {
	flag.Parse()

	if err := bootBackend(); err != nil {
		fmt.Fprintln(os.Stderr, "contract harness:", err)
		os.Exit(1)
	}

	code := m.Run()
	_ = os.RemoveAll("storage") // storage.Init creates this next to the test
	os.Exit(code)
}

// bootBackend starts the backend in SQLite mode, seeds a user/project plus
// telemetry, builds the CLI binary, and logs it in. Everything lives in temp
// dirs that are intentionally left for the OS to reclaim (the process is short
// lived); the local ./storage dir storage.Init creates is removed explicitly.
func bootBackend() error {
	port, err := freePort()
	if err != nil {
		return fmt.Errorf("free port: %w", err)
	}
	baseURL = fmt.Sprintf("http://127.0.0.1:%d", port)

	tmp, err := os.MkdirTemp("", "tw-contract-")
	if err != nil {
		return err
	}
	dbPath := filepath.Join(tmp, "traceway.db")

	go tracewaybackend.Run(
		tracewaybackend.WithPort(port),
		tracewaybackend.WithSQLitePath(dbPath),
		tracewaybackend.WithServerURL(baseURL),
		tracewaybackend.WithDefaultUser(seedEmail, seedPassword),
		tracewaybackend.WithDefaultProject(seedProject, "gin", seedToken),
		tracewaybackend.DisableLogging(),
	)

	if err := waitReady(baseURL, 30*time.Second); err != nil {
		return fmt.Errorf("backend never became ready: %w", err)
	}

	jwtToken, projectID, err = appLogin(baseURL, seedEmail, seedPassword)
	if err != nil {
		return fmt.Errorf("login: %w", err)
	}

	seedAt = time.Now().UTC().Add(-time.Hour)
	if err := seedTelemetry(context.Background(), projectID, seedAt); err != nil {
		return fmt.Errorf("seed telemetry: %w", err)
	}
	if err := waitForData(15 * time.Second); err != nil {
		return fmt.Errorf("seeded telemetry never became visible: %w", err)
	}

	cliBin = filepath.Join(tmp, "traceway")
	if err := buildCLI(cliBin); err != nil {
		return fmt.Errorf("build cli: %w", err)
	}

	xdgConfig, err = os.MkdirTemp("", "tw-cfg-")
	if err != nil {
		return err
	}
	xdgState, err = os.MkdirTemp("", "tw-state-")
	if err != nil {
		return err
	}
	if err := cliLogin(); err != nil {
		return fmt.Errorf("cli login: %w", err)
	}
	return nil
}

// freePort asks the OS for an unused TCP port, then releases it for the backend
// to bind. The small window between close and re-bind is acceptable for a test.
func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

// waitReady polls the unauthenticated has-organizations endpoint until the
// server answers (status < 500) or the deadline passes.
func waitReady(base string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	hc := &http.Client{Timeout: 2 * time.Second}
	for time.Now().Before(deadline) {
		resp, err := hc.Get(base + "/api/has-organizations")
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode < 500 {
				return nil
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	return errors.New("timed out")
}

// appLogin posts the seeded credentials and returns (jwt, projectID).
func appLogin(base, email, password string) (string, string, error) {
	body, _ := json.Marshal(map[string]string{"email": email, "password": password})
	resp, err := http.Post(base+"/api/login", "application/json", bytes.NewReader(body))
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("status %d: %s", resp.StatusCode, raw)
	}
	var parsed struct {
		Token    string `json:"token"`
		Projects []struct {
			Id string `json:"id"`
		} `json:"projects"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", "", fmt.Errorf("decode login: %w", err)
	}
	if parsed.Token == "" || len(parsed.Projects) == 0 {
		return "", "", fmt.Errorf("login returned no token/project: %s", raw)
	}
	return parsed.Token, parsed.Projects[0].Id, nil
}

// waitForData polls endpoints/grouped until the seeded endpoint is visible,
// absorbing any insert lag before the assertions run.
func waitForData(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	c := client.New(baseURL, client.WithJWT(jwtToken))
	tr := client.TimeRangeFromExplicit(seedAt.Add(-time.Hour), seedAt.Add(time.Hour))
	for time.Now().Before(deadline) {
		resp, err := c.ListEndpoints(context.Background(), projectID, client.ListEndpointsRequest{
			TimeRange:  tr,
			Pagination: client.PaginationParams{Page: 1, PageSize: 50},
		})
		if err == nil && len(resp.Data) > 0 {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return errors.New("timed out")
}

// buildCLI compiles the traceway binary from the sibling cli module.
func buildCLI(out string) error {
	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	cliRoot := filepath.Join(wd, "..", "..") // cli/test/contract -> cli
	cmd := exec.Command("go", "build", "-o", out, "./cmd/traceway")
	cmd.Dir = cliRoot
	if combined, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%w: %s", err, combined)
	}
	return nil
}

// cliLogin runs the real login + projects-use flow so the CLI binary is
// authenticated against the booted server for the round-trip assertions.
func cliLogin() error {
	if out, _, code := runCLI(nil, seedPassword+"\n", "login", "--url", baseURL, "--username", seedEmail, "--password-stdin"); code != 0 {
		return fmt.Errorf("login exit %d: %s", code, out)
	}
	if out, _, code := runCLI(nil, "", "projects", "use", projectID); code != 0 {
		return fmt.Errorf("projects use exit %d: %s", code, out)
	}
	return nil
}

// runCLI executes the built binary with the CLI-under-test XDG dirs. t may be
// nil (used during boot before any test is running); on a nil t an exec failure
// returns code -1 instead of failing a test.
func runCLI(t *testing.T, stdin string, args ...string) (stdout, stderr string, code int) {
	if t != nil {
		t.Helper()
	}
	cmd := exec.Command(cliBin, args...)
	cmd.Env = append(os.Environ(),
		"XDG_CONFIG_HOME="+xdgConfig,
		"XDG_STATE_HOME="+xdgState,
	)
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	var so, se bytes.Buffer
	cmd.Stdout = &so
	cmd.Stderr = &se
	err := cmd.Run()
	code = 0
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			code = ee.ExitCode()
		} else if t != nil {
			t.Fatalf("runCLI %v: %v", args, err)
		} else {
			return so.String(), se.String(), -1
		}
	}
	return so.String(), se.String(), code
}

// capturingRT tees each response body so a test can snapshot the raw wire bytes
// while the typed client still decodes the (replayed) body normally.
type capturingRT struct {
	body []byte
}

func (c *capturingRT) RoundTrip(r *http.Request) (*http.Response, error) {
	resp, err := http.DefaultTransport.RoundTrip(r)
	if err != nil {
		return nil, err
	}
	b, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	c.body = b
	resp.Body = io.NopCloser(bytes.NewReader(b))
	return resp, nil
}

// capturedClient returns a client whose transport records the raw response of
// the next request, so callers get both the typed result (request-encoding
// contract) and the raw wire bytes (response-shape contract) from one call.
func capturedClient() (*client.Client, *capturingRT) {
	rt := &capturingRT{}
	c := client.New(baseURL,
		client.WithJWT(jwtToken),
		client.WithHTTPClient(&http.Client{Transport: rt, Timeout: 30 * time.Second}),
	)
	return c, rt
}

// goldenAssert reduces body to a value-independent wire-shape fingerprint and
// compares it against testdata/<name>.schema.txt (or rewrites it under -update).
func goldenAssert(t *testing.T, name string, body []byte) {
	t.Helper()
	got, err := fingerprint(body)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	path := filepath.Join("testdata", name+".schema.txt")
	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%s: missing golden %s (regenerate with: go test ./... -update): %v", name, path, err)
	}
	if string(want) != got {
		t.Errorf("%s: wire schema drift vs %s\n--- want ---\n%s\n--- got ---\n%s", name, path, want, got)
	}
}

// fingerprint maps a JSON document to a stable schema: the sorted set of
// "path: type" lines. Array indices collapse to [], so element order and count
// don't matter; object keys are kept literally, so a rename or casing change
// (e.g. Timestamp -> timestamp) shows up as a diff.
func fingerprint(body []byte) (string, error) {
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return "", fmt.Errorf("response is not JSON: %w (body: %s)", err, clip(body))
	}
	set := map[string]struct{}{}
	walk("", v, set)
	lines := make([]string, 0, len(set))
	for l := range set {
		lines = append(lines, l)
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n") + "\n", nil
}

func walk(path string, v any, set map[string]struct{}) {
	switch t := v.(type) {
	case map[string]any:
		if len(t) == 0 {
			set[path+": object{}"] = struct{}{}
			return
		}
		for k, child := range t {
			p := k
			if path != "" {
				p = path + "." + k
			}
			walk(p, child, set)
		}
	case []any:
		if len(t) == 0 {
			set[path+"[]: empty"] = struct{}{}
			return
		}
		for _, e := range t {
			walk(path+"[]", e, set)
		}
	case json.Number:
		set[path+": number"] = struct{}{}
	case string:
		set[path+": string"] = struct{}{}
	case bool:
		set[path+": bool"] = struct{}{}
	case nil:
		set[path+": null"] = struct{}{}
	}
}

func clip(b []byte) string {
	const max = 200
	if len(b) > max {
		return string(b[:max]) + "..."
	}
	return string(b)
}

// sortStrings is a tiny insertion-free sort wrapper kept local so the file has
// no dependency beyond the standard library's sort.
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j-1] > s[j]; j-- {
			s[j-1], s[j] = s[j], s[j-1]
		}
	}
}
