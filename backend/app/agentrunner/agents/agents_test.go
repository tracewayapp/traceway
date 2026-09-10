package agents

import (
	"bufio"
	"errors"
	"os"
	"strings"
	"testing"
)

func TestParseReport(t *testing.T) {
	result, err := ParseReport("some chatter\nSTATUS: Fixed\nSUBJECT: abc\n\nbody line 1\nbody line 2")
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != StatusFixed || result.Subject != "abc" || result.Report != "body line 1\nbody line 2" {
		t.Fatalf("result = %+v", result)
	}
	for _, bad := range []string{"", "no status here", "STATUS: fixed", "STATUS: fixed\nHASH: abc", "STATUS: done\nSUBJECT: x"} {
		if _, err := ParseReport(bad); !errors.Is(err, ErrMalformedReport) {
			t.Errorf("%q must be malformed, got %v", bad, err)
		}
	}
	question, _ := ParseReport("STATUS: question\nSUBJECT: abc\nWhich database backs checkout?")
	if question.Status != StatusQuestion || question.Report != "Which database backs checkout?" {
		t.Fatalf("question = %+v", question)
	}
}

func TestClaudeStreamNormalizesRecordedRun(t *testing.T) {
	file, err := os.Open("testdata/claude-stream.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	parser := ClaudeCode{}.NewParser()
	var kinds []string
	var malformed int
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		events, err := parser.Parse(scanner.Bytes())
		if err != nil {
			malformed++
			continue
		}
		for _, event := range events {
			if event.SchemaVersion != EventSchemaVersion || event.At.IsZero() {
				t.Fatalf("event lacks the envelope: %+v", event)
			}
			kinds = append(kinds, event.Kind)
		}
	}
	if malformed != 1 {
		t.Fatalf("the non-JSON line must be reported once, got %d", malformed)
	}
	want := "assistant_text tool_call tool_result tool_call tool_result usage result"
	if got := strings.Join(kinds, " "); got != want {
		t.Fatalf("kinds = %s, want %s", got, want)
	}
	result := parser.Result()
	if result.Status != StatusFixed || result.Subject != "0123456789abcdef" || result.SessionId != "sess-123" {
		t.Fatalf("result = %+v", result)
	}
	if !strings.HasPrefix(result.Report, "## Root cause") {
		t.Fatalf("the chatter before STATUS must be dropped: %q", result.Report)
	}
	if result.Usage.CostUSD != 0.1834 || result.Usage.Turns != 4 || result.Usage.InputTokens != 2700 || result.Usage.OutputTokens != 200 {
		t.Fatalf("usage = %+v", result.Usage)
	}
}

func TestClaudeStreamQuestionErrorAndMissingResult(t *testing.T) {
	parser := ClaudeCode{}.NewParser()
	events, err := parser.Parse([]byte(`{"type":"result","subtype":"success","result":"STATUS: question\nSUBJECT: x\nWhich env?","session_id":"s1","num_turns":2}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 3 || events[1].Kind != EventQuestion || events[1].Text != "Which env?" {
		t.Fatalf("question events = %+v", events)
	}
	if result := parser.Result(); result.Status != StatusQuestion || result.SessionId != "s1" {
		t.Fatalf("result = %+v", result)
	}

	failed := ClaudeCode{}.NewParser()
	if _, err := failed.Parse([]byte(`{"type":"result","subtype":"error_max_turns","is_error":true,"result":"","session_id":"s2","num_turns":60}`)); err != nil {
		t.Fatal(err)
	}
	if result := failed.Result(); result.Status != StatusError || !strings.Contains(result.Error, "error_max_turns") {
		t.Fatalf("error result = %+v", result)
	}

	noReport := ClaudeCode{}.NewParser()
	noReport.Parse([]byte(`{"type":"result","subtype":"success","result":"I could not decide.","session_id":"s3"}`))
	if result := noReport.Result(); result.Status != StatusError || !strings.Contains(result.Error, "STATUS") {
		t.Fatalf("a result without the report format must be an error: %+v", result)
	}

	empty := ClaudeCode{}.NewParser()
	if result := empty.Result(); result.Status != StatusError || !strings.Contains(result.Error, "without a result") {
		t.Fatalf("no result line = %+v", result)
	}
}

func TestClaudeCodeCommandEnvAndEgress(t *testing.T) {
	inv := Invocation{Profile: Profile{Model: "claude-opus-5", MaxTurns: 20, Credential: "sk-ant-x", AllowedTools: []string{"Read", "Edit"}}, SessionId: "sess"}
	argv := ClaudeCode{}.Command(inv)
	joined := strings.Join(argv, " ")
	for _, want := range []string{"claude -p", "--output-format stream-json", "--allowedTools Read,Edit", "--max-turns 20", "--model claude-opus-5", "--resume sess"} {
		if !strings.Contains(joined, want) {
			t.Errorf("argv lacks %q: %s", want, joined)
		}
	}
	env := ClaudeCode{}.Env(inv)
	if env["ANTHROPIC_API_KEY"] != "sk-ant-x" || env["DISABLE_TELEMETRY"] != "1" {
		t.Fatalf("env = %v", env)
	}
	if hosts := (ClaudeCode{}).Egress(inv); len(hosts) != 1 || hosts[0].Host != "api.anthropic.com" || hosts[0].Port != 443 {
		t.Fatalf("egress = %v", hosts)
	}
	inv.Profile.BaseURL = "http://gateway.internal:8080/v1"
	if hosts := (ClaudeCode{}).Egress(inv); hosts[0].Host != "gateway.internal" || hosts[0].Port != 8080 {
		t.Fatalf("egress with base url = %v", hosts)
	}
	if env := (ClaudeCode{}).Env(inv); env["ANTHROPIC_BASE_URL"] != "http://gateway.internal:8080/v1" {
		t.Fatalf("base url env = %v", env)
	}
	if got := (ClaudeCode{}).Command(Invocation{Profile: Profile{MaxTurns: 9999}}); !strings.Contains(strings.Join(got, " "), "--max-turns 500") {
		t.Fatalf("turns must be capped: %v", got)
	}
}

func TestCommandStreamSplitsTranscriptAndReport(t *testing.T) {
	parser := Command{}.NewParser()
	var kinds []string
	for _, line := range []string{"looking around", "", "---REPORT---", "STATUS: analysis", "SUBJECT: ref", "Nothing to fix."} {
		events, err := parser.Parse([]byte(line))
		if err != nil {
			t.Fatal(err)
		}
		for _, event := range events {
			kinds = append(kinds, event.Kind+":"+event.Text)
		}
	}
	if strings.Join(kinds, ",") != "assistant_text:looking around" {
		t.Fatalf("events = %v", kinds)
	}
	if result := parser.Result(); result.Status != StatusAnalysis || result.Report != "Nothing to fix." {
		t.Fatalf("result = %+v", result)
	}
	if result := (Command{}).NewParser().Result(); result.Status != StatusError {
		t.Fatalf("no report must be an error: %+v", result)
	}
}
