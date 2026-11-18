package agent

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/catalog"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/tools"
)

// scripted is a Chatter that returns canned responses and records requests.
type scripted struct {
	mu    sync.Mutex
	resps []*Response
	reqs  []*Request
}

func (s *scripted) Chat(_ context.Context, req *Request) (*Response, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reqs = append(s.reqs, req)
	if len(s.resps) == 0 {
		return nil, errors.New("script exhausted")
	}
	r := s.resps[0]
	s.resps = s.resps[1:]
	return r, nil
}

func toolResp(name, args string, cost float64) *Response {
	r := &Response{Provider: "test"}
	r.Choices = append(r.Choices, struct {
		Message      Message `json:"message"`
		FinishReason string  `json:"finish_reason"`
	}{Message: Message{Role: "assistant", ToolCalls: []ToolCall{{ID: "t1", Type: "function", Function: FunctionCall{Name: name, Arguments: args}}}}})
	r.Usage.Cost = cost
	r.Usage.PromptTokens = 1000
	return r
}

func textResp(text string) *Response {
	r := &Response{}
	r.Choices = append(r.Choices, struct {
		Message      Message `json:"message"`
		FinishReason string  `json:"finish_reason"`
	}{Message: Message{Role: "assistant", Content: text}})
	return r
}

type echoTool struct{}

func (echoTool) Spec() tools.Spec {
	return tools.Spec{Name: "echo", Parameters: map[string]any{"type": "object"}}
}
func (echoTool) Call(context.Context, json.RawMessage) tools.Result {
	return tools.Result{Output: "echoed", OK: true}
}

// submission is a valid submit_perf_diagnosis payload whose report is r.
func submission(r string) string {
	return `{"checklist_item":"none_found","locus":{},"evidence":[{"call_id":"c01","claim":"x"}],"fix":"none","confidence":"low","report":"` + r + `"}`
}

func cfg(s *scripted) Config {
	return Config{
		Model: &catalog.Model{Key: "m", ID: "x/y", Caching: "anthropic"}, Chat: s, System: "sys", Prompt: "do it", SubmitTool: tools.SubmitPerf,
		Tools:  []tools.Tool{echoTool{}, &tools.Submit{Kind: tools.SubmitPerf, MaxRepairs: 2}},
		Budget: catalog.Budget{MaxTurns: 5, MaxToolCalls: 3},
	}
}

func TestLoopSubmits(t *testing.T) {
	s := &scripted{resps: []*Response{toolResp("echo", `{}`, 0.01), toolResp(tools.SubmitPerf, submission("done"), 0.01)}}
	run := Run(context.Background(), cfg(s))
	if run.StopReason != StopSubmitted || run.Submission["report"] != "done" || len(run.Calls) != 2 || run.Calls[0].ID != "c01" {
		t.Fatalf("run = %+v", run)
	}
	if run.Usage.CostUSD != 0.02 {
		t.Fatalf("cost %v", run.Usage.CostUSD)
	}
	// The second request carries the assistant tool call before its result,
	// and the result starts with the call id the model cites.
	msgs := s.reqs[1].Messages
	if msgs[2].Role != "assistant" || len(msgs[2].ToolCalls) != 1 || msgs[3].Role != "tool" {
		t.Fatalf("history order: %+v", msgs)
	}
	parts, ok := msgs[3].Content.([]Part)
	if !ok || !strings.HasPrefix(parts[0].Text, "[call c01]") || parts[0].CacheControl == nil {
		t.Fatalf("the newest tool result must carry the call id and a cache breakpoint: %#v", msgs[3].Content)
	}
	if sys, ok := msgs[0].Content.([]Part); !ok || sys[0].CacheControl == nil {
		t.Fatal("the system prompt must carry a cache breakpoint for anthropic models")
	}
	if _, ok := msgs[1].Content.(string); !ok {
		t.Fatal("only the system prompt and the newest message get breakpoints")
	}
}

func TestLoopNudgesOnceThenStops(t *testing.T) {
	s := &scripted{resps: []*Response{textResp("I think it is fine."), textResp("Really.")}}
	run := Run(context.Background(), cfg(s))
	if run.StopReason != StopNoSubmit || len(s.reqs) != 2 {
		t.Fatalf("stop %s after %d requests", run.StopReason, len(s.reqs))
	}
	last := s.reqs[1].Messages
	if !strings.Contains(textOf(last[len(last)-1]), "call "+tools.SubmitPerf) {
		t.Fatal("the nudge must name the submit tool")
	}
}

func textOf(m Message) string {
	if s, ok := m.Content.(string); ok {
		return s
	}
	if ps, ok := m.Content.([]Part); ok && len(ps) > 0 {
		return ps[0].Text
	}
	return ""
}

func TestLoopBudgets(t *testing.T) {
	var resps []*Response
	for i := 0; i < 10; i++ {
		resps = append(resps, toolResp("echo", `{}`, 0))
	}
	run := Run(context.Background(), cfg(&scripted{resps: resps}))
	if run.StopReason != StopBudget {
		t.Fatalf("stop %s", run.StopReason)
	}
	rejected := 0
	for _, c := range run.Calls {
		if c.Rejected {
			rejected++
		}
	}
	if rejected == 0 {
		t.Fatal("calls past max_tool_calls must be refused")
	}

	c := cfg(&scripted{resps: []*Response{toolResp("echo", `{}`, 2), toolResp("echo", `{}`, 2)}})
	ledger := NewLedger(10)
	c.Hold = ledger.Reserve(3)
	run = Run(context.Background(), c)
	if run.StopReason != StopCeiling || !strings.Contains(run.StopDetail, "cost cap") {
		t.Fatalf("run cap: %s %s", run.StopReason, run.StopDetail)
	}
	if run.Turns != 1 || ledger.Spent() != 2 {
		t.Fatalf("the run must stop before a second $2 call would cross its $3 cap: %d turns, $%v", run.Turns, ledger.Spent())
	}
	c = cfg(&scripted{resps: []*Response{toolResp("echo", `{}`, 2)}})
	c.Ledger = NewLedger(1)
	c.Ledger.Add("other", 1)
	if run := Run(context.Background(), c); run.StopReason != StopCeiling {
		t.Fatalf("ceiling: %s", run.StopReason)
	}
}

func TestUnknownToolIsAnError(t *testing.T) {
	s := &scripted{resps: []*Response{toolResp("nope", `{}`, 0), toolResp(tools.SubmitPerf, submission("x"), 0)}}
	run := Run(context.Background(), cfg(s))
	if run.Calls[0].OK || !strings.Contains(run.Calls[0].Output, "no tool named") {
		t.Fatalf("%+v", run.Calls[0])
	}
}

func TestClientRefusesRemoteWithoutGuard(t *testing.T) {
	if _, err := NewOpenRouter("sk-or-test"); !errors.Is(err, ErrNotAuthorized) {
		t.Fatalf("NewOpenRouter under go test = %v", err)
	}
	c := &Client{BaseURL: OpenRouterURL, Key: "sk-or-test"}
	if _, err := c.Chat(context.Background(), &Request{Model: "x"}); !errors.Is(err, ErrNotAuthorized) {
		t.Fatalf("a remote call under go test = %v", err)
	}
}

func TestClientWireFormat(t *testing.T) {
	var got map[string]any
	var hdr http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hdr = r.Header
		_ = json.NewDecoder(r.Body).Decode(&got)
		w.Write([]byte(`{"id":"g1","provider":"Anthropic","choices":[{"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":2,"cost":0.001,"prompt_tokens_details":{"cached_tokens":4}}}`))
	}))
	defer srv.Close()
	c := &Client{BaseURL: srv.URL, Key: "k"}
	allow := true
	resp, err := c.Chat(context.Background(), &Request{Model: "anthropic/claude-opus-5.5", Messages: applyCaching("anthropic", []Message{{Role: "system", Content: "s"}, {Role: "user", Content: "u"}}),
		Provider: &ProviderPrefs{Order: []string{"anthropic"}, AllowFallbacks: &allow, RequireParameters: true}, Usage: map[string]any{"include": true}})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Provider != "Anthropic" || resp.Usage.PromptTokensDetails.CachedTokens != 4 {
		t.Fatalf("%+v", resp)
	}
	if hdr.Get("Authorization") != "Bearer k" || hdr.Get("X-Title") != "traceway-evals" || hdr.Get("HTTP-Referer") == "" {
		t.Fatalf("headers %v", hdr)
	}
	prov := got["provider"].(map[string]any)
	if prov["require_parameters"] != true || prov["order"].([]any)[0] != "anthropic" {
		t.Fatalf("provider %v", prov)
	}
	sys := got["messages"].([]any)[0].(map[string]any)["content"].([]any)[0].(map[string]any)
	if sys["cache_control"].(map[string]any)["type"] != "ephemeral" {
		t.Fatalf("system %v", sys)
	}
}

func TestPromptHelpers(t *testing.T) {
	if got := StripSlash("/traceway perf GET /x\nslow"); got != "perf GET /x\nslow" {
		t.Fatal(got)
	}
	if SlashSkill("/traceway debug x") != "traceway" || SlashSkill("why is it slow") != "" {
		t.Fatal("SlashSkill")
	}
	p := SystemPrompt(PromptInput{SubmitTool: tools.SubmitPerf, Skills: []tools.Skill{{Name: "traceway", Description: "d"}}})
	if !strings.Contains(p, "- traceway: d") || !strings.Contains(p, "call "+tools.SubmitPerf) {
		t.Fatal(p)
	}
}

// TestFakeRuntimeTemplates: numbers looked up from a tool result keep the
// text they had there, at any size.
func TestFakeRuntimeTemplates(t *testing.T) {
	msgs := []Message{{Role: "tool", Content: "[call c01]\n{\"d\":45200000,\"t\":1760000000123456789,\"f\":0.25,\"o\":{\"a\":[1,2]}}\n[budget: 3 tool calls and 4 turns left]"}}
	got, err := resolveRuntime("--min {{last.d}} --at {{calls.c01.t}} --r {{last.f}} {{last.o}}", msgs)
	if err != nil || got != `--min 45200000 --at 1760000000123456789 --r 0.25 {"a":[1,2]}` {
		t.Fatalf("got %q, %v", got, err)
	}
	args, err := resolveArgs(map[string]any{"n": "{{last.d}}"}, msgs)
	if raw, _ := json.Marshal(args); err != nil || string(raw) != `{"n":45200000}` {
		t.Fatalf("whole-string template: %s, %v", raw, err)
	}
}

func TestFakeSubmitTool(t *testing.T) {
	if got := submitTool(map[string]bool{"shell": true, tools.SubmitPerf: true, "read_file": true}); got != tools.SubmitPerf {
		t.Fatalf("submitTool = %q", got)
	}
	if got := submitTool(map[string]bool{"shell": true}); got != "" {
		t.Fatalf("submitTool without one = %q", got)
	}
}
