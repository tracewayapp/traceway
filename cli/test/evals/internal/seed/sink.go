package seed

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/env"
)

// Sink is where a scenario's telemetry and objects go: a live backend, or a
// dry recorder used by the leakage lint and unit tests.
type Sink interface {
	CreateProject(name, framework string) (id, token string, err error)
	// Ingest posts a body to an ingest path with a project token and
	// returns the response body.
	Ingest(token, path, contentType string, body []byte, gz bool) ([]byte, error)
	// Admin calls the dashboard API as the instance owner.
	Admin(method, path string, query url.Values, body any) (*env.Response, error)
	// Dry reports a recorder that has no backend behind it.
	Dry() bool
	// Org is the instance's organization and owner.
	Org() (orgID, adminUserID int)
	// Stub is the switchable HTTP target for monitors (nil when absent).
	Stub() StubControl
}

// StubControl is the part of env.Stub the seed uses.
type StubControl interface {
	SetHealthy(bool)
	Healthy() bool
}

// LiveSink sends to a running backend.
type LiveSink struct {
	Ctx     context.Context
	Backend *env.Backend
	Admin_  *env.Admin
	HTTP    *http.Client

	StubSrv *env.Stub

	mu       sync.Mutex
	projects []env.Project
}

// NewLiveSink returns a sink for a booted, registered instance.
func NewLiveSink(ctx context.Context, b *env.Backend, a *env.Admin) *LiveSink {
	return &LiveSink{Ctx: ctx, Backend: b, Admin_: a, HTTP: &http.Client{Timeout: 2 * time.Minute}}
}

// Projects returns the projects created so far, tokens included.
func (l *LiveSink) Projects() []env.Project {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]env.Project{}, l.projects...)
}

// CreateProject creates a project in the admin's organization.
func (l *LiveSink) CreateProject(name, framework string) (string, string, error) {
	ps, err := env.CreateProjects(l.Ctx, l.Backend, l.Admin_, []env.Project{{Name: name, Framework: framework}})
	if err != nil {
		return "", "", err
	}
	l.mu.Lock()
	l.projects = append(l.projects, ps[0])
	l.mu.Unlock()
	return ps[0].ID, ps[0].Token, nil
}

// Ingest posts with retries on the admission gate's 503.
func (l *LiveSink) Ingest(token, path, contentType string, body []byte, gz bool) ([]byte, error) {
	payload := body
	if gz {
		var buf bytes.Buffer
		w := gzip.NewWriter(&buf)
		_, _ = w.Write(body)
		_ = w.Close()
		payload = buf.Bytes()
	}
	var last error
	for attempt := 0; attempt < 6; attempt++ {
		req, err := http.NewRequestWithContext(l.Ctx, http.MethodPost, l.Backend.URL+path, bytes.NewReader(payload))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", contentType)
		if gz {
			req.Header.Set("Content-Encoding", "gzip")
		}
		resp, err := l.HTTP.Do(req)
		if err != nil {
			last = err
		} else {
			var msg bytes.Buffer
			_, _ = msg.ReadFrom(resp.Body)
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return msg.Bytes(), nil
			}
			last = fmt.Errorf("POST %s: HTTP %d: %s", path, resp.StatusCode, msg.String())
			if resp.StatusCode != http.StatusServiceUnavailable && resp.StatusCode != http.StatusTooManyRequests {
				return nil, last
			}
		}
		time.Sleep(time.Duration(attempt+1) * 500 * time.Millisecond)
	}
	return nil, last
}

// Admin calls the dashboard API with the owner's session.
func (l *LiveSink) Admin(method, path string, query url.Values, body any) (*env.Response, error) {
	return l.Backend.API().WithBearer(l.Admin_.JWT).Do(l.Ctx, method, path, query, body)
}

// Dry reports false.
func (l *LiveSink) Dry() bool { return false }

// Org returns the organization and owner ids.
func (l *LiveSink) Org() (int, int) { return l.Admin_.OrgID, l.Admin_.UserID }

// Stub returns the monitor target.
func (l *LiveSink) Stub() StubControl {
	if l.StubSrv == nil {
		return nil
	}
	return l.StubSrv
}

// StubURL returns the stub's base URL.
func (l *LiveSink) StubURL() string {
	if l.StubSrv == nil {
		return ""
	}
	return l.StubSrv.URL
}

// DrySink records what would be sent.
type DrySink struct {
	mu      sync.Mutex
	nextID  int
	Bodies  map[string]int // path -> count
	Calls   []string
	project int
}

// NewDrySink returns an empty recorder.
func NewDrySink() *DrySink { return &DrySink{Bodies: map[string]int{}} }

// CreateProject returns a synthetic id and token.
func (d *DrySink) CreateProject(name, framework string) (string, string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.project++
	return fmt.Sprintf("00000000-0000-4000-8000-%012d", d.project), fmt.Sprintf("dry-token-%d", d.project), nil
}

// Ingest counts the body.
func (d *DrySink) Ingest(_, path, _ string, _ []byte, _ bool) ([]byte, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.Bodies[path]++
	return nil, nil
}

// Admin answers creates with a synthetic id and everything else with {}.
func (d *DrySink) Admin(method, path string, _ url.Values, _ any) (*env.Response, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.Calls = append(d.Calls, method+" "+path)
	d.nextID++
	body, _ := json.Marshal(map[string]any{"id": d.nextID, "updatedAt": time.Unix(0, 0).UTC().Format(time.RFC3339Nano)})
	return &env.Response{Status: http.StatusOK, Body: body}, nil
}

// Dry reports true.
func (d *DrySink) Dry() bool { return true }

// Org returns fixed ids.
func (d *DrySink) Org() (int, int) { return 1, 1 }

// Stub returns a stub that only remembers its state.
func (d *DrySink) Stub() StubControl { return &dryStub{} }

type dryStub struct{ healthy bool }

func (s *dryStub) SetHealthy(v bool) { s.healthy = v }
func (s *dryStub) Healthy() bool     { return s.healthy }
