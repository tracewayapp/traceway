package seed

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// The native /api/report protocol is what the browser and mobile SDKs speak:
// exceptions with their own stack format, sessions, and session recordings
// whose actions carry the network requests and navigations the SDK captured.
// Only what those SDKs really send is modelled here.

type nativeFrame struct {
	appVersion string
	serverName string
	exceptions []nativeException
	sessions   []nativeSession
	recordings []nativeRecording
}

type nativeException struct {
	Stack       string            `json:"stackTrace"`
	RecordedAt  string            `json:"recordedAt"`
	Attributes  map[string]string `json:"attributes"`
	IsMessage   bool              `json:"isMessage"`
	IsTask      bool              `json:"isTask"`
	SessionID   string            `json:"sessionId,omitempty"`
	RecordingID string            `json:"sessionRecordingId,omitempty"`
}

type nativeSession struct {
	ID         string            `json:"id"`
	StartedAt  string            `json:"startedAt"`
	EndedAt    string            `json:"endedAt,omitempty"`
	ClientIP   string            `json:"clientIP"`
	Attributes map[string]string `json:"attributes"`
}

type nativeRecording struct {
	ExceptionID string           `json:"exceptionId"`
	SessionID   string           `json:"sessionId,omitempty"`
	Events      []any            `json:"events"`
	Logs        []map[string]any `json:"logs,omitempty"`
	Actions     []map[string]any `json:"actions,omitempty"`
	StartedAt   string           `json:"startedAt,omitempty"`
	EndedAt     string           `json:"endedAt,omitempty"`
}

// Crash is one exception reported by a browser or mobile SDK.
type Crash struct {
	At         time.Time
	Stack      string
	Attributes map[string]string
	IsMessage  bool
	AppVersion string
	// Device is the SDK's serverName (the device or browser identity).
	Device string
	// SessionID links the crash to a browser session.
	SessionID string
	// Actions are what the SDK captured before the crash (network,
	// navigation, custom), shown with the occurrence.
	Actions []Action
	// Logs are console/logcat lines captured before the crash.
	Logs []ActionLog
}

// Action is one captured client action.
type Action struct {
	Type       string // network | navigation | custom
	At         time.Time
	Method     string
	URL        string
	DurationMs float64
	Status     int
	Error      string
	From, To   string
	RespBytes  int
}

// ActionLog is one captured client log line.
type ActionLog struct {
	At      time.Time
	Level   string
	Message string
}

func (a Action) wire() map[string]any {
	m := map[string]any{"type": a.Type, "timestamp": a.At.UTC().Format(time.RFC3339Nano)}
	switch a.Type {
	case "network":
		m["method"], m["url"], m["durationMs"] = a.Method, a.URL, a.DurationMs
		if a.Status != 0 {
			m["statusCode"] = a.Status
		}
		if a.Error != "" {
			m["error"] = a.Error
		}
		if a.RespBytes != 0 {
			m["responseBytes"] = a.RespBytes
		}
	case "navigation":
		m["action"] = "push"
		m["from"], m["to"] = a.From, a.To
	default:
		m["category"], m["name"] = "app", a.URL
	}
	return m
}

// Crash queues a native SDK exception for the project.
func (p *Project) Crash(c Crash) {
	f := nativeFrame{appVersion: c.AppVersion, serverName: c.Device}
	ex := nativeException{
		Stack: c.Stack, RecordedAt: c.At.UTC().Format(time.RFC3339Nano),
		Attributes: c.Attributes, IsMessage: c.IsMessage, SessionID: c.SessionID,
	}
	if ex.Attributes == nil {
		ex.Attributes = map[string]string{}
	}
	if len(c.Actions) > 0 || len(c.Logs) > 0 {
		ex.RecordingID = uuid.NewString()
		rec := nativeRecording{ExceptionID: ex.RecordingID, SessionID: c.SessionID, Events: []any{}}
		first, last := c.At, c.At
		for _, a := range c.Actions {
			rec.Actions = append(rec.Actions, a.wire())
			if a.At.Before(first) {
				first = a.At
			}
			p.s.record(a.URL, a.Error)
		}
		for _, l := range c.Logs {
			rec.Logs = append(rec.Logs, map[string]any{"timestamp": l.At.UTC().Format(time.RFC3339Nano), "level": l.Level, "message": l.Message})
			p.s.record(l.Message)
		}
		rec.StartedAt, rec.EndedAt = first.UTC().Format(time.RFC3339Nano), last.UTC().Format(time.RFC3339Nano)
		f.recordings = append(f.recordings, rec)
	}
	f.exceptions = append(f.exceptions, ex)
	p.native = append(p.native, f)
	p.s.record(c.Stack, c.Device)
	for _, v := range c.Attributes {
		p.s.record(v)
	}
}

// Session queues a browser session (only the browser SDK reports sessions).
func (p *Project) Session(id string, started, ended time.Time, attrs map[string]string, appVersion string) {
	if attrs == nil {
		attrs = map[string]string{}
	}
	p.native = append(p.native, nativeFrame{appVersion: appVersion, sessions: []nativeSession{{
		ID: id, StartedAt: started.UTC().Format(time.RFC3339Nano), EndedAt: ended.UTC().Format(time.RFC3339Nano),
		ClientIP: "203.0.113.24", Attributes: attrs,
	}}})
}

// nativeBodies groups queued frames by (appVersion, serverName) into
// /api/report bodies of at most 200 exceptions each.
func (p *Project) nativeBodies() ([][]byte, error) {
	type key struct{ v, d string }
	groups := map[key]*nativeFrame{}
	var order []key
	for _, f := range p.native {
		k := key{f.appVersion, f.serverName}
		g := groups[k]
		if g == nil {
			g = &nativeFrame{appVersion: f.appVersion, serverName: f.serverName}
			groups[k] = g
			order = append(order, k)
		}
		g.exceptions = append(g.exceptions, f.exceptions...)
		g.sessions = append(g.sessions, f.sessions...)
		g.recordings = append(g.recordings, f.recordings...)
	}
	var out [][]byte
	for _, k := range order {
		g := groups[k]
		frame := map[string]any{
			"traces": []any{}, "metrics": []any{},
			"stackTraces": g.exceptions, "sessions": g.sessions, "sessionRecordings": g.recordings,
		}
		if g.exceptions == nil {
			frame["stackTraces"] = []any{}
		}
		if g.sessions == nil {
			frame["sessions"] = []any{}
		}
		if g.recordings == nil {
			frame["sessionRecordings"] = []any{}
		}
		body, err := json.Marshal(map[string]any{
			"appVersion": g.appVersion, "serverName": g.serverName,
			"collectionFrames": []any{frame},
		})
		if err != nil {
			return nil, fmt.Errorf("native frame: %w", err)
		}
		out = append(out, body)
	}
	return out, nil
}
