package env

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestWriteGate is the Phase 1 acceptance check for writes: a write token is
// refused while the organization's agent write opt-in is off and accepted once
// it is on, a read token is refused, members join through invitations, and the
// agent activity log records the refused and the accepted write.
func TestWriteGate(t *testing.T) {
	bins := BinariesForTest(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	dir := t.TempDir()
	b, err := Boot(ctx, bins.Backend, BootOptions{Dir: filepath.Join(dir, "backend")})
	if err != nil {
		t.Fatal(err)
	}
	defer b.Stop()
	admin, err := Register(ctx, b, "admin@example.com", "eval-password-1", "Eval Org")
	if err != nil {
		t.Fatal(err)
	}
	projects, err := CreateProjects(ctx, b, admin, []Project{{Name: "api", Framework: "opentelemetry"}})
	if err != nil {
		t.Fatal(err)
	}
	pid := projects[0].ID
	readPAT, err := CreatePAT(ctx, b, admin.JWT, "eval-read", "read", 0)
	if err != nil {
		t.Fatal(err)
	}
	writePAT, err := CreatePAT(ctx, b, admin.JWT, "eval-write", "write", 1)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(writePAT, "tww_") {
		t.Errorf("unexpected write token prefix %q", writePAT[:4])
	}

	write := b.API().WithBearer(writePAT)
	slow := map[string]any{"endpoint": "GET /x", "offsetMs": 100, "reason": "test"}
	res, err := write.Do(ctx, http.MethodPost, "/api/endpoints/slow", ProjectQuery(pid), slow)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != http.StatusForbidden || !strings.Contains(string(res.Body), "Agent write access is turned off") {
		t.Errorf("write with opt-in off: want 403 opt-in message, got %d %s", res.Status, res.Body)
	}
	if err := SetAgentAccess(ctx, b, admin, true); err != nil {
		t.Fatal(err)
	}
	res, err = write.Do(ctx, http.MethodPost, "/api/endpoints/slow", ProjectQuery(pid), slow)
	if err != nil || res.Status != http.StatusOK {
		t.Errorf("write with opt-in on: want 200, got %v %d %s", err, res.Status, res.Body)
	}
	res, _ = b.API().WithBearer(readPAT).Do(ctx, http.MethodPost, "/api/endpoints/slow", ProjectQuery(pid), slow)
	if res.Status != http.StatusForbidden || !strings.Contains(string(res.Body), "read-only") {
		t.Errorf("read token write: want 403 read-only, got %d %s", res.Status, res.Body)
	}

	m, err := AddMember(ctx, b, admin, "viewer@example.com", "readonly")
	if err != nil {
		t.Fatal(err)
	}
	if m.UserID == 0 || m.JWT == "" {
		t.Errorf("member without id or jwt: %+v", m)
	}

	act, err := b.API().WithBearer(admin.JWT).MustOK(ctx, http.MethodGet, "/api/organizations/"+strconv.Itoa(admin.OrgID)+"/agent-activity", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	var page struct {
		Data []struct {
			StatusCode int `json:"statusCode"`
		} `json:"data"`
	}
	_ = json.Unmarshal(act.Body, &page)
	if len(page.Data) != 2 {
		t.Errorf("agent activity: want the refused and the accepted write, got %s", act.Body)
	}
}
