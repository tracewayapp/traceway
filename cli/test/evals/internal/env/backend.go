package env

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"sync"
	"syscall"
	"time"
)

// Backend is one running backend subprocess.
type Backend struct {
	URL         string
	Port        int
	Dir         string // run/backend: cwd, SQLite files, ./storage
	DBPath      string
	HealthToken string

	cmd  *exec.Cmd
	log  *syncBuffer
	done chan error
}

// BootOptions tunes the backend.
type BootOptions struct {
	// Dir is the run's backend directory (created if missing).
	Dir string
	// Env adds or overrides variables.
	Env map[string]string
	// Ready bounds how long to wait for /health.
	Ready time.Duration
}

// errPortTaken means another server answered on the port FreePort picked
// before this backend could bind it (a parallel run won the race).
var errPortTaken = errors.New("another server answered on the backend's port")

// Boot starts the backend binary in SQLite mode with fast background workers
// and waits until it answers /health/deep with its own token. A port lost to
// a parallel run is retried on a fresh one.
func Boot(ctx context.Context, bin string, opt BootOptions) (*Backend, error) {
	for attempt := 1; ; attempt++ {
		b, err := boot(ctx, bin, opt)
		if errors.Is(err, errPortTaken) && attempt < 3 {
			continue
		}
		return b, err
	}
}

func boot(ctx context.Context, bin string, opt BootOptions) (*Backend, error) {
	if err := os.MkdirAll(opt.Dir, 0o755); err != nil {
		return nil, err
	}
	port, err := FreePort()
	if err != nil {
		return nil, err
	}
	b := &Backend{
		URL:         fmt.Sprintf("http://127.0.0.1:%d", port),
		Port:        port,
		Dir:         opt.Dir,
		DBPath:      filepath.Join(opt.Dir, "traceway.db"),
		HealthToken: randomHex(16),
		log:         &syncBuffer{},
		done:        make(chan error, 1),
	}
	vars := map[string]string{
		"PATH":                             os.Getenv("PATH"),
		"HOME":                             opt.Dir,
		"TMPDIR":                           os.TempDir(),
		"TZ":                               "UTC",
		"JWT_SECRET":                       randomHex(24), // 48 characters
		"DB_TYPE":                          "sqlite",
		"SQLITE_PATH":                      b.DBPath,
		"STORAGE_TYPE":                     "local",
		"STORAGE_PATH":                     filepath.Join(opt.Dir, "storage"),
		"PORTS":                            strconv.Itoa(port),
		"APP_BASE_URL":                     b.URL,
		"SQLITE_RETENTION_DAYS":            "0",
		"NOTIFICATION_POLL_SECONDS":        "5",
		"ONCALL_POLL_SECONDS":              "5",
		"OUTBOX_POLL_SECONDS":              "5",
		"SYNTHETICS_POLL_SECONDS":          "5",
		"SYNTHETICS_ALLOW_PRIVATE_TARGETS": "true",
		"HEALTH_DEEP_TOKEN":                b.HealthToken,
		"SQLITE_CACHE_SIZE_MB":             "64",
		"INGEST_ADMISSION_WAIT_SECONDS":    "30",
		"REPORT_MAX_BODY_MB":               "64",
	}
	for k, v := range opt.Env {
		vars[k] = v
	}
	cmd := exec.Command(bin)
	cmd.Dir = opt.Dir
	cmd.Env = make([]string, 0, len(vars))
	for k, v := range vars {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	cmd.Stdout = b.log
	cmd.Stderr = b.log
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start backend: %w", err)
	}
	b.cmd = cmd
	go func() { b.done <- cmd.Wait() }()

	ready := opt.Ready
	if ready == 0 {
		ready = 60 * time.Second
	}
	if err := b.waitReady(ctx, ready); err != nil {
		b.Stop()
		if errors.Is(err, errPortTaken) {
			return nil, err
		}
		return nil, fmt.Errorf("%w\n--- backend log ---\n%s", err, b.LogTail(4000))
	}
	return b, nil
}

func (b *Backend) waitReady(ctx context.Context, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	hc := &http.Client{Timeout: 2 * time.Second}
	for time.Now().Before(deadline) {
		select {
		case err := <-b.done:
			b.done <- err
			return fmt.Errorf("backend exited before it was ready: %v", err)
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		// /health/deep needs this backend's own token, so a server that
		// took the port first answers 401 instead of passing for ours.
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, b.URL+"/api/health/deep", nil)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+b.HealthToken)
		if resp, err := hc.Do(req); err == nil {
			_ = resp.Body.Close()
			switch resp.StatusCode {
			case http.StatusOK:
				return nil
			case http.StatusUnauthorized:
				return errPortTaken
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	return errors.New("backend never answered /health")
}

// Stop terminates the backend and its process group.
func (b *Backend) Stop() {
	if b.cmd == nil || b.cmd.Process == nil {
		return
	}
	_ = syscall.Kill(-b.cmd.Process.Pid, syscall.SIGTERM)
	select {
	case <-b.done:
	case <-time.After(10 * time.Second):
		_ = syscall.Kill(-b.cmd.Process.Pid, syscall.SIGKILL)
		<-b.done
	}
	b.cmd = nil
}

// Log returns the backend's combined output so far.
func (b *Backend) Log() string { return b.log.String() }

// LogTail returns the last n bytes of the log.
func (b *Backend) LogTail(n int) string {
	s := b.log.String()
	if len(s) > n {
		return s[len(s)-n:]
	}
	return s
}

// WriteLog saves the log next to the run.
func (b *Backend) WriteLog(path string) error {
	return os.WriteFile(path, []byte(b.log.String()), 0o644)
}

var inviteTokenRe = regexp.MustCompile(`accept-invitation\?token=([0-9a-f-]{36})`)

// InvitationTokens returns every invitation accept token the backend has
// logged so far. With SMTP off the email is written to the log, link included.
func (b *Backend) InvitationTokens() []string {
	var out []string
	for _, m := range inviteTokenRe.FindAllStringSubmatch(b.log.String(), -1) {
		out = append(out, m[1])
	}
	return out
}

// API returns an unauthenticated API client for this backend.
func (b *Backend) API() *API { return &API{Base: b.URL} }

// FreePort asks the OS for an unused TCP port.
func FreePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}
