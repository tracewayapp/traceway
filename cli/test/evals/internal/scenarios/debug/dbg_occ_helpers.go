package debug

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/env"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/seed"
)

// dbgOcc is one exception occurrence read back through the dashboard API.
type dbgOcc struct {
	ID         string            `json:"id"`
	RecordedAt time.Time         `json:"recordedAt"`
	StackTrace string            `json:"stackTrace"`
	Attributes map[string]string `json:"attributes"`
	AppVersion string            `json:"appVersion"`
	ServerName string            `json:"serverName"`
	TraceID    string            `json:"traceId"`
}

// dbgOccurrences returns up to n occurrences of a group, newest first. In a
// dry sink it returns one synthetic occurrence so seeds stay lintable.
func dbgOccurrences(s *seed.Scenario, p *seed.Project, hash string, n int) ([]dbgOcc, error) {
	if s.Sink.Dry() {
		return []dbgOcc{{ID: "6ba7b810-9dad-11d1-80b4-00c04fd430c8", RecordedAt: s.Ago(3 * time.Hour), StackTrace: "dry", TraceID: strings.Repeat("ab", 16)}}, nil
	}
	res, err := s.Sink.Admin(http.MethodPost, "/api/exception-stack-traces/"+hash, env.ProjectQuery(p.ID),
		map[string]any{"pagination": map[string]int{"page": 1, "pageSize": n}})
	if err != nil {
		return nil, err
	}
	if err := res.Err("exception group " + hash); err != nil {
		return nil, err
	}
	var out struct {
		Occurrences []dbgOcc `json:"occurrences"`
	}
	if err := json.Unmarshal(res.Body, &out); err != nil {
		return nil, err
	}
	sort.SliceStable(out.Occurrences, func(i, j int) bool { return out.Occurrences[i].RecordedAt.After(out.Occurrences[j].RecordedAt) })
	return out.Occurrences, nil
}

// dbgOccRef is what seed.json carries for an occurrence: its id, its
// recordedAt as the API returns it, the URL-encoded form a dashboard link
// carries in ?t=, and the notification email's "Occurred at" form.
func dbgOccRef(o dbgOcc) map[string]any {
	at := o.RecordedAt.UTC().Format(time.RFC3339Nano)
	return map[string]any{
		"id":         o.ID,
		"recordedAt": at,
		"t":          url.QueryEscape(at),
		"second":     o.RecordedAt.UTC().Format("2006-01-02T15:04:05"),
		"email":      o.RecordedAt.UTC().Format("2006-01-02 15:04:05") + " UTC",
		"traceId":    o.TraceID,
		"appVersion": o.AppVersion,
	}
}

// dbgOtelStack is the exception.stacktrace the Go SDK records for
// span.RecordError(err, trace.WithStackTrace(true)): runtime.Stack from
// inside the SDK, so the SDK's own frames come first, then the caller's,
// into a 2048-byte buffer that cuts the stack off.
func dbgOtelStack(goroutine int, frames ...string) string {
	s := fmt.Sprintf("goroutine %d [running]:\n", goroutine)
	s += "go.opentelemetry.io/otel/sdk/trace.recordStackTrace(...)\n\t/go/pkg/mod/go.opentelemetry.io/otel/sdk@v1.38.0/trace/span.go:570\n"
	s += "go.opentelemetry.io/otel/sdk/trace.(*recordingSpan).RecordError(0xc0004a2f00, {0x1b6e0a0, 0xc0003f81e0}, {0xc000519b98, 0x1, 0x1})\n\t/go/pkg/mod/go.opentelemetry.io/otel/sdk@v1.38.0/trace/span.go:552 +0x585\n"
	for _, f := range frames {
		s += f + "\n"
	}
	return s[:min(len(s), 2048)]
}

// dbgRandID fills a span id from the scenario's RNG, the way the DSL does,
// for spans a seed builds by hand.
func dbgRandID(s *seed.Scenario) [8]byte {
	var b [8]byte
	v := s.RNG.Uint64() | 1
	for i := range b {
		b[i] = byte(v >> (8 * i))
	}
	return b
}

// dbgDownstream appends, under a CLIENT span, the SERVER span of another
// service that handled it (used when the client gave up before the server
// finished, which Req.Call cannot express).
func dbgDownstream(s *seed.Scenario, client *seed.Span, target *seed.Service, method, route, path string, start, dur time.Duration, status int) *seed.Span {
	sp := &seed.Span{
		Name: method + " " + route, Kind: seed.KindServer, Start: start, Dur: dur, Scope: target.Stack.ServerScope,
		Service: target, Host: target.Hosts[0], SpanID: dbgRandID(s),
		Attrs: map[string]any{"http.request.method": method, "http.route": route, "url.path": path, "url.scheme": "http", "http.response.status_code": status},
	}
	client.Children = append(client.Children, sp)
	return sp
}

// dbgSpanError records an exception event on a database span the way
// otelsql does (span.RecordError without a stack trace, an error status
// without a description) and keeps the request's full tree.
func dbgSpanError(r *seed.Req, sp *seed.Span, e seed.Exc) {
	attrs := map[string]any{"exception.type": e.Type, "exception.message": e.Message}
	sp.Events = append(sp.Events, seed.Event{Name: "exception", At: sp.Dur, Attrs: attrs})
	sp.Status = seed.StatusError
	r.KeepDetail()
	dbgRecord(r, e.Type, e.Message)
}

// dbgRecord hands strings a seed builds by hand to the leakage lint (the
// scenario's recorder is unexported; Req.Attr records string values).
func dbgRecord(r *seed.Req, strs ...string) {
	const k = "twevals.lint"
	cur, had := r.Request().Root.Attrs[k]
	for _, x := range strs {
		r.RootAttr(k, x)
	}
	if had {
		r.Request().Root.Attrs[k] = cur
	} else {
		delete(r.Request().Root.Attrs, k)
	}
}
