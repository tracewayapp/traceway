// Package agents adapts coding agents to the harness. An Agent says how to
// invoke the agent's CLI headless and how to read its output stream; the
// harness owns the process itself, the sandbox around it and every
// credential. Claude Code is the reference adapter; command runs any
// executable that speaks the simple stdout protocol, which the pipeline
// tests script.
package agents

import (
	"bufio"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/tracewayapp/traceway/backend/app/sandbox"
)

// EventSchemaVersion is stamped into every normalized event; the stream is
// a wire format read by the run page and recorded for evals.
const EventSchemaVersion = 1

// Profile is the part of an agent profile the adapter needs; the harness
// decrypts the credential and hands it over just for the process.
type Profile struct {
	Model          string   `json:"model,omitempty"`
	Provider       string   `json:"provider,omitempty"`
	BaseURL        string   `json:"baseUrl,omitempty"`
	Credential     string   `json:"credential,omitempty"`
	MaxTurns       int      `json:"maxTurns,omitempty"`
	BudgetUSD      float64  `json:"budgetUsd,omitempty"`
	TimeoutMinutes int      `json:"timeoutMinutes,omitempty"`
	AllowedTools   []string `json:"allowedTools,omitempty"`
}

// Invocation is one run: a fresh session with a prompt, or a resumed one
// with the message that woke it.
type Invocation struct {
	Prompt    string
	SessionId string
	Workdir   string
	Profile   Profile
}

// Agent adapts one coding agent CLI.
type Agent interface {
	Name() string
	// Command is the argv the harness runs inside the sandbox. The prompt
	// arrives on stdin.
	Command(inv Invocation) []string
	// Env is what the agent needs beyond the harness allowlist: the provider
	// key under the name the CLI reads, a base URL, a home for its state.
	Env(inv Invocation) map[string]string
	// Egress lists the hosts the agent must reach, for the network policy.
	Egress(inv Invocation) []sandbox.HostPort
	// NewParser returns a parser for one run's output stream.
	NewParser() Parser
}

// Parser turns the agent's stdout into normalized events and, at the end,
// the run's result.
type Parser interface {
	Parse(line []byte) ([]Event, error)
	Result() Result
}

// Event kinds every adapter normalizes to.
const (
	EventAssistantText = "assistant_text"
	EventToolCall      = "tool_call"
	EventToolResult    = "tool_result"
	EventUsage         = "usage"
	EventQuestion      = "question"
	EventResult        = "result"
)

type Event struct {
	SchemaVersion int             `json:"schemaVersion"`
	Kind          string          `json:"kind"`
	SessionId     string          `json:"sessionId,omitempty"`
	Text          string          `json:"text,omitempty"`
	Tool          string          `json:"tool,omitempty"`
	Input         json.RawMessage `json:"input,omitempty"`
	Usage         *Usage          `json:"usage,omitempty"`
	At            time.Time       `json:"at"`
}

type Usage struct {
	InputTokens  int64   `json:"inputTokens"`
	OutputTokens int64   `json:"outputTokens"`
	CostUSD      float64 `json:"costUsd"`
	Turns        int     `json:"turns"`
}

// Result statuses, the first line of the report.
const (
	StatusFixed    = "fixed"
	StatusAnalysis = "analysis"
	StatusQuestion = "question"
	StatusError    = "error"
)

type Result struct {
	Status    string
	Subject   string
	Report    string
	SessionId string
	Usage     Usage
	Error     string
}

var ErrMalformedReport = errors.New("the agent's report does not start with STATUS and SUBJECT lines")

// ParseReport reads the report format every agent ends with: line 1
// "STATUS: fixed | analysis | question", line 2 "SUBJECT: <ref>", then the
// markdown body. Anything the agent printed before the STATUS line (a
// closing remark) is dropped, so a chatty model still yields a report.
func ParseReport(text string) (Result, error) {
	scanner := bufio.NewScanner(strings.NewReader(text))
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	start := -1
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "STATUS:") {
			start = i
			break
		}
	}
	if start < 0 || start+1 >= len(lines) {
		return Result{}, ErrMalformedReport
	}
	status := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(lines[start]), "STATUS:")))
	subjectLine := strings.TrimSpace(lines[start+1])
	if !strings.HasPrefix(subjectLine, "SUBJECT:") {
		return Result{}, ErrMalformedReport
	}
	switch status {
	case StatusFixed, StatusAnalysis, StatusQuestion:
	default:
		return Result{}, ErrMalformedReport
	}
	return Result{
		Status:  status,
		Subject: strings.TrimSpace(strings.TrimPrefix(subjectLine, "SUBJECT:")),
		Report:  strings.TrimSpace(strings.Join(lines[start+2:], "\n")),
	}, nil
}

func newEvent(kind string) Event {
	return Event{SchemaVersion: EventSchemaVersion, Kind: kind, At: time.Now().UTC()}
}
