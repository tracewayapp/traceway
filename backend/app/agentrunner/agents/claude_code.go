package agents

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/tracewayapp/traceway/backend/app/sandbox"
)

const (
	ClaudeCodeName        = "claude-code"
	anthropicHost         = "api.anthropic.com"
	defaultClaudeTurns    = 60
	claudeCodeBinary      = "claude"
	defaultAllowedTools   = "Read,Edit,Write,Glob,Grep,Bash,MultiEdit"
	claudeMaxTurnsCeiling = 500
)

// ClaudeCode drives the Claude Code CLI headless: one prompt on stdin,
// stream-json on stdout, a session id to resume with.
type ClaudeCode struct{}

func (ClaudeCode) Name() string { return ClaudeCodeName }

func (ClaudeCode) Command(inv Invocation) []string {
	turns := inv.Profile.MaxTurns
	if turns <= 0 {
		turns = defaultClaudeTurns
	}
	if turns > claudeMaxTurnsCeiling {
		turns = claudeMaxTurnsCeiling
	}
	tools := defaultAllowedTools
	if len(inv.Profile.AllowedTools) > 0 {
		tools = strings.Join(inv.Profile.AllowedTools, ",")
	}
	argv := []string{claudeCodeBinary, "-p", "--output-format", "stream-json", "--verbose", "--allowedTools", tools, "--max-turns", strconv.Itoa(turns)}
	if inv.Profile.Model != "" {
		argv = append(argv, "--model", inv.Profile.Model)
	}
	if inv.Profile.BudgetUSD > 0 {
		argv = append(argv, "--max-budget-usd", strconv.FormatFloat(inv.Profile.BudgetUSD, 'f', -1, 64))
	}
	if inv.SessionId != "" {
		argv = append(argv, "--resume", inv.SessionId)
	}
	return argv
}

func (ClaudeCode) Env(inv Invocation) map[string]string {
	env := map[string]string{
		"ANTHROPIC_API_KEY":                        inv.Profile.Credential,
		"CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC": "1",
		"DISABLE_TELEMETRY":                        "1",
		"DISABLE_AUTOUPDATER":                      "1",
	}
	if inv.Profile.BaseURL != "" {
		env["ANTHROPIC_BASE_URL"] = inv.Profile.BaseURL
	}
	return env
}

func (ClaudeCode) Egress(inv Invocation) []sandbox.HostPort {
	host, port := anthropicHost, 443
	if inv.Profile.BaseURL != "" {
		if parsed, err := url.Parse(inv.Profile.BaseURL); err == nil && parsed.Hostname() != "" {
			host = parsed.Hostname()
			if p := parsed.Port(); p != "" {
				port, _ = strconv.Atoi(p)
			} else if parsed.Scheme == "http" {
				port = 80
			}
		}
	}
	return []sandbox.HostPort{{Host: host, Port: port}}
}

func (ClaudeCode) NewParser() Parser { return &claudeStream{} }

// claudeStream reads Claude Code's stream-json: system/init with the session
// id, assistant messages with text and tool_use blocks, user messages with
// tool_result blocks, and a final result carrying the text, usage and cost.
type claudeStream struct {
	sessionId string
	result    *Result
	usage     Usage
}

type claudeLine struct {
	Type      string          `json:"type"`
	Subtype   string          `json:"subtype"`
	SessionId string          `json:"session_id"`
	Message   *claudeMessage  `json:"message"`
	Result    string          `json:"result"`
	IsError   bool            `json:"is_error"`
	CostUSD   float64         `json:"total_cost_usd"`
	NumTurns  int             `json:"num_turns"`
	Usage     *claudeUsage    `json:"usage"`
	Error     json.RawMessage `json:"error"`
}

type claudeMessage struct {
	Content []claudeBlock `json:"content"`
	Usage   *claudeUsage  `json:"usage"`
}

type claudeBlock struct {
	Type    string          `json:"type"`
	Text    string          `json:"text"`
	Name    string          `json:"name"`
	Input   json.RawMessage `json:"input"`
	Content json.RawMessage `json:"content"`
}

type claudeUsage struct {
	InputTokens  int64 `json:"input_tokens"`
	OutputTokens int64 `json:"output_tokens"`
}

func (s *claudeStream) Parse(line []byte) ([]Event, error) {
	trimmed := strings.TrimSpace(string(line))
	if trimmed == "" {
		return nil, nil
	}
	var entry claudeLine
	if err := json.Unmarshal([]byte(trimmed), &entry); err != nil {
		return nil, fmt.Errorf("claude-code stream: not JSON: %.120s", trimmed)
	}
	if entry.SessionId != "" {
		s.sessionId = entry.SessionId
	}
	switch entry.Type {
	case "assistant":
		return s.assistantEvents(entry), nil
	case "user":
		return s.toolResultEvents(entry), nil
	case "result":
		return s.resultEvents(entry), nil
	}
	return nil, nil
}

func (s *claudeStream) assistantEvents(entry claudeLine) []Event {
	if entry.Message == nil {
		return nil
	}
	var events []Event
	for _, block := range entry.Message.Content {
		switch block.Type {
		case "text":
			if strings.TrimSpace(block.Text) != "" {
				event := newEvent(EventAssistantText)
				event.Text = block.Text
				events = append(events, event)
			}
		case "tool_use":
			event := newEvent(EventToolCall)
			event.Tool = block.Name
			event.Input = trimInput(block.Input)
			events = append(events, event)
		}
	}
	if entry.Message.Usage != nil {
		s.usage.InputTokens += entry.Message.Usage.InputTokens
		s.usage.OutputTokens += entry.Message.Usage.OutputTokens
	}
	return events
}

func (s *claudeStream) toolResultEvents(entry claudeLine) []Event {
	if entry.Message == nil {
		return nil
	}
	var events []Event
	for _, block := range entry.Message.Content {
		if block.Type != "tool_result" {
			continue
		}
		event := newEvent(EventToolResult)
		event.Text = summarizeToolResult(block.Content)
		events = append(events, event)
	}
	return events
}

func (s *claudeStream) resultEvents(entry claudeLine) []Event {
	usage := s.usage
	if entry.Usage != nil {
		usage.InputTokens = entry.Usage.InputTokens
		usage.OutputTokens = entry.Usage.OutputTokens
	}
	usage.CostUSD = entry.CostUSD
	usage.Turns = entry.NumTurns
	s.usage = usage

	result := Result{SessionId: s.sessionId, Usage: usage}
	if entry.IsError || entry.Subtype != "success" {
		result.Status = StatusError
		result.Error = strings.TrimSpace(entry.Result)
		if result.Error == "" {
			result.Error = "claude-code ended with " + entry.Subtype
		}
	} else if parsed, err := ParseReport(entry.Result); err != nil {
		result.Status = StatusError
		result.Error = err.Error()
		result.Report = entry.Result
	} else {
		parsed.SessionId = s.sessionId
		parsed.Usage = usage
		result = parsed
	}
	s.result = &result

	usageEvent := newEvent(EventUsage)
	usageEvent.Usage = &usage
	events := []Event{usageEvent}
	if result.Status == StatusQuestion {
		question := newEvent(EventQuestion)
		question.Text = result.Report
		events = append(events, question)
	}
	final := newEvent(EventResult)
	final.SessionId = s.sessionId
	final.Text = result.Status
	if result.Error != "" {
		final.Text = result.Error
	}
	events = append(events, final)
	return events
}

func (s *claudeStream) Result() Result {
	if s.result != nil {
		return *s.result
	}
	return Result{Status: StatusError, Error: "claude-code exited without a result", SessionId: s.sessionId, Usage: s.usage}
}

const (
	toolInputLimit  = 2000
	toolResultLimit = 500
)

func trimInput(input json.RawMessage) json.RawMessage {
	if len(input) <= toolInputLimit {
		return input
	}
	summary, _ := json.Marshal(map[string]string{"truncated": string(input[:toolInputLimit])})
	return summary
}

func summarizeToolResult(content json.RawMessage) string {
	var text string
	if err := json.Unmarshal(content, &text); err != nil {
		var blocks []claudeBlock
		if err := json.Unmarshal(content, &blocks); err == nil {
			var parts []string
			for _, block := range blocks {
				if block.Text != "" {
					parts = append(parts, block.Text)
				}
			}
			text = strings.Join(parts, "\n")
		} else {
			text = string(content)
		}
	}
	if len(text) > toolResultLimit {
		text = text[:toolResultLimit] + "…"
	}
	return text
}
