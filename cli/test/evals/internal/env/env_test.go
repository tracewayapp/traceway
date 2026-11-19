package env

import (
	"context"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestTwoInstances boots two backends in parallel, registers each, and logs
// the CLI in with a read token.
func TestTwoInstances(t *testing.T) {
	bins := BinariesForTest(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			checkInstance(t, ctx, bins)
		}()
	}
	wg.Wait()
}

func checkInstance(t *testing.T, ctx context.Context, bins *Binaries) {
	dir := t.TempDir()
	b, err := Boot(ctx, bins.Backend, BootOptions{Dir: filepath.Join(dir, "backend")})
	if err != nil {
		t.Error(err)
		return
	}
	defer b.Stop()

	admin, err := Register(ctx, b, "admin@example.com", "eval-password-1", "Eval Org")
	if err != nil {
		t.Error(err)
		return
	}
	projects, err := CreateProjects(ctx, b, admin, []Project{{Name: "api", Framework: "opentelemetry"}})
	if err != nil {
		t.Error(err)
		return
	}
	pid := projects[0].ID

	readPAT, err := CreatePAT(ctx, b, admin.JWT, "eval-read", "read", 0)
	if err != nil {
		t.Error(err)
		return
	}
	if !strings.HasPrefix(readPAT, "twp_") {
		t.Errorf("unexpected read token prefix %q", readPAT[:4])
	}

	cli, err := NewCLIHome(filepath.Join(dir, "cli"), bins.CLI)
	if err != nil {
		t.Error(err)
		return
	}
	if err := cli.Login(ctx, b.URL, readPAT, pid); err != nil {
		t.Error(err)
		return
	}
	out, _, err := cli.Run(ctx, "", "projects", "list", "--output", "json")
	if err != nil || !strings.Contains(out, pid) {
		t.Errorf("projects list after login: %v %s", err, out)
	}

}

func TestStubSwitches(t *testing.T) {
	s, err := StartStub(false)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	res, err := http.Get(s.URL + "/health")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 503 {
		t.Fatalf("unhealthy stub answered %d", res.StatusCode)
	}
	s.SetHealthy(true)
	res, _ = http.Get(s.URL + "/health")
	res.Body.Close()
	if res.StatusCode != 200 || s.Hits() != 2 {
		t.Fatalf("healthy stub answered %d after %d hits", res.StatusCode, s.Hits())
	}
}
