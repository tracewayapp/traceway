// Package apifixture serves recorded Traceway API responses over httptest so
// the access layer, the MCP server and the commands can be exercised against
// one consistent instance without booting a backend. Every id the responses
// contain is exported through Fixtures.
package apifixture

import (
	"embed"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

//go:embed responses/*.json
var responses embed.FS

// Fixtures names the records the recorded responses contain, so a test can
// ask for them and expect them back.
type Fixtures struct {
	ProjectID     string
	From, To      time.Time
	At            time.Time
	ExceptionHash string
	OccurrenceID  uuid.UUID
	RequestID     uuid.UUID
	EndpointName  string
	MetricName    string
	TaskID        uuid.UUID
	AITraceID     uuid.UUID
	SessionID     uuid.UUID
	TraceID       uuid.UUID
	UnknownHash   string
	UnknownID     uuid.UUID
	AttemptID     uuid.UUID
}

var Known = Fixtures{
	ProjectID:     "11111111-1111-4111-8111-111111111111",
	From:          time.Date(2026, 6, 1, 11, 0, 0, 0, time.UTC),
	To:            time.Date(2026, 6, 1, 13, 0, 0, 0, time.UTC),
	At:            time.Date(2026, 6, 1, 12, 5, 0, 0, time.UTC),
	ExceptionHash: "0123456789abcdef",
	OccurrenceID:  uuid.MustParse("bbbbbbbb-0000-4000-8000-000000000001"),
	RequestID:     uuid.MustParse("cccccccc-0000-4000-8000-000000000001"),
	EndpointName:  "POST /api/checkout",
	MetricName:    "system.cpu.utilization",
	TaskID:        uuid.MustParse("99999999-0000-4000-8000-000000000001"),
	AITraceID:     uuid.MustParse("88888888-0000-4000-8000-000000000001"),
	SessionID:     uuid.MustParse("eeeeeeee-0000-4000-8000-000000000001"),
	TraceID:       uuid.MustParse("dddddddd-0000-4000-8000-000000000001"),
	UnknownHash:   "ffffffffffffffff",
	UnknownID:     uuid.MustParse("00000000-0000-4000-8000-00000000dead"),
	AttemptID:     uuid.MustParse("aaaaaaaa-0000-4000-8000-000000000001"),
}

// Server serves the recorded responses. Requests it records are available
// through Requests for assertions on the wire shape a caller produced.
type Server struct {
	*httptest.Server
	mu       sync.Mutex
	requests []Request
}

type Request struct {
	Method string
	Path   string
	Query  string
	Body   string
}

func New(t *testing.T) *Server {
	t.Helper()
	s := &Server{}
	s.Server = httptest.NewServer(http.HandlerFunc(s.serve))
	t.Cleanup(s.Close)
	return s
}

func (s *Server) Requests() []Request {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Request(nil), s.requests...)
}

func (s *Server) serve(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	s.mu.Lock()
	s.requests = append(s.requests, Request{Method: r.Method, Path: r.URL.Path, Query: r.URL.RawQuery, Body: string(body)})
	s.mu.Unlock()

	file, ok := responseFor(r.URL.Path)
	if !ok {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}
	body, err := responses.ReadFile("responses/" + file)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(body)
}

func responseFor(path string) (string, bool) {
	known := Known
	switch {
	case path == "/api/projects":
		return "projects.json", true
	case path == "/api/logs":
		return "logs.json", true
	case path == "/api/exception-stack-traces":
		return "exceptions-list.json", true
	case path == "/api/exception-stack-traces/archive", path == "/api/exception-stack-traces/unarchive":
		return "success.json", true
	case path == "/api/exception-stack-traces/by-id/"+known.OccurrenceID.String():
		return "occurrence.json", true
	case path == "/api/exception-stack-traces/"+known.ExceptionHash:
		return "exception-detail.json", true
	case path == "/api/endpoints/grouped":
		return "endpoints.json", true
	case path == "/api/endpoints/chart":
		return "chart.json", true
	case path == "/api/endpoints/slow":
		return "slow.json", true
	case path == "/api/endpoints/"+known.RequestID.String():
		return "request.json", true
	case path == "/api/metrics/query":
		return "metrics.json", true
	case path == "/api/metrics/discover":
		return "discover.json", true
	case path == "/api/tasks/"+known.TaskID.String():
		return "task.json", true
	case path == "/api/ai-traces/"+known.AITraceID.String():
		return "aitrace.json", true
	case path == "/api/sessions/"+known.SessionID.String():
		return "session.json", true
	case path == "/api/distributed-traces/"+known.TraceID.String():
		return "trace.json", true
	case path == "/api/agent/attempts/list":
		return "attempts-list.json", true
	case path == "/api/agent/attempts/report":
		return "attempt-report.json", true
	case path == "/api/agent/attempts/"+known.AttemptID.String():
		return "attempt.json", true
	case path == "/api/agent/attempts/"+known.AttemptID.String()+"/events":
		return "attempt-events.json", true
	}
	return "", false
}
