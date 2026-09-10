package agents

import (
	"strings"

	"github.com/tracewayapp/traceway/backend/app/sandbox"
)

const CommandName = "command"

// Command runs any executable as the agent: the prompt arrives on stdin,
// every stdout line before the report marker is assistant text, and what
// follows the marker is the report. It has no session to resume, so a
// resumed attempt is a fresh run with the thread in the prompt. The
// pipeline tests script it with shell; the runtime is what a marketplace
// package with runtime "command" gets.
type Command struct {
	Argv   []string
	Hosts  []sandbox.HostPort
	Extra  map[string]string
	Marker string
}

const DefaultReportMarker = "---REPORT---"

func (Command) Name() string { return CommandName }

func (c Command) Command(Invocation) []string { return c.Argv }

func (c Command) Env(inv Invocation) map[string]string {
	env := map[string]string{}
	for name, value := range c.Extra {
		env[name] = value
	}
	if inv.Profile.Credential != "" {
		env["AGENT_CREDENTIAL"] = inv.Profile.Credential
	}
	return env
}

func (c Command) Egress(Invocation) []sandbox.HostPort { return c.Hosts }

func (c Command) NewParser() Parser {
	marker := c.Marker
	if marker == "" {
		marker = DefaultReportMarker
	}
	return &commandStream{marker: marker}
}

type commandStream struct {
	marker    string
	inReport  bool
	report    []string
	sawOutput bool
}

func (s *commandStream) Parse(line []byte) ([]Event, error) {
	text := strings.TrimRight(string(line), "\r\n")
	s.sawOutput = true
	if s.inReport {
		s.report = append(s.report, text)
		return nil, nil
	}
	if strings.TrimSpace(text) == s.marker {
		s.inReport = true
		return nil, nil
	}
	if strings.TrimSpace(text) == "" {
		return nil, nil
	}
	event := newEvent(EventAssistantText)
	event.Text = text
	return []Event{event}, nil
}

func (s *commandStream) Result() Result {
	if !s.inReport {
		return Result{Status: StatusError, Error: "the command produced no report"}
	}
	result, err := ParseReport(strings.Join(s.report, "\n"))
	if err != nil {
		return Result{Status: StatusError, Error: err.Error(), Report: strings.Join(s.report, "\n")}
	}
	return result
}
