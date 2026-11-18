// Package record holds what one eval run produced: every tool call with the
// operations it performed, the final answer, and usage. The agent loop writes
// it, the graders read it, and transcript.jsonl is its serialized form.
package record

import (
	"encoding/json"
	"time"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/ops"
)

// OpRecord is one operation inside a tool call: a traceway invocation in a
// shell pipeline, an MCP tool call, a file read, ...
type OpRecord struct {
	ops.Op
	Argv      []string `json:"argv,omitempty"`
	Exit      int      `json:"exit"`
	OK        bool     `json:"ok"`
	Output    string   `json:"output,omitempty"` // raw output before any pipe
	Truncated bool     `json:"truncated,omitempty"`
}

// Call is one tool call the model made. Its ID ("c07") is what the model
// cites as evidence.
type Call struct {
	ID         string          `json:"id"`
	Turn       int             `json:"turn"`
	Tool       string          `json:"tool"`
	Args       json.RawMessage `json:"args"`
	Output     string          `json:"output"` // what the model saw
	OK         bool            `json:"ok"`
	Truncated  bool            `json:"truncated,omitempty"`
	Ops        []OpRecord      `json:"ops,omitempty"`
	DurationMs int64           `json:"durationMs"`
	Rejected   bool            `json:"rejected,omitempty"` // schema/budget refusal, not executed
}

// Usage is token and cost accounting for one model response.
type Usage struct {
	InputTokens      int     `json:"inputTokens"`
	CachedTokens     int     `json:"cachedTokens"`
	CacheWriteTokens int     `json:"cacheWriteTokens"`
	OutputTokens     int     `json:"outputTokens"`
	ReasoningTokens  int     `json:"reasoningTokens"`
	CostUSD          float64 `json:"costUsd"`
}

// Add accumulates u2 into u.
func (u *Usage) Add(u2 Usage) {
	u.InputTokens += u2.InputTokens
	u.CachedTokens += u2.CachedTokens
	u.CacheWriteTokens += u2.CacheWriteTokens
	u.OutputTokens += u2.OutputTokens
	u.ReasoningTokens += u2.ReasoningTokens
	u.CostUSD += u2.CostUSD
}

// Question is one ask_user exchange.
type Question struct {
	CallID   string `json:"callId"`
	Question string `json:"question"`
	Answer   string `json:"answer"`
}

// Run is the complete record of one agent run.
type Run struct {
	Calls       []*Call        `json:"calls"`
	Submission  map[string]any `json:"submission,omitempty"`
	SubmitTool  string         `json:"submitTool,omitempty"`
	SubmitCall  string         `json:"submitCall,omitempty"`
	FinalText   string         `json:"finalText,omitempty"`
	Questions   []Question     `json:"questions,omitempty"`
	Usage       Usage          `json:"usage"`
	Turns       int            `json:"turns"`
	StopReason  string         `json:"stopReason"` // submitted | budget_exhausted | no_submit | infra_error | cost_ceiling
	StopDetail  string         `json:"stopDetail,omitempty"`
	SkillLoaded []string       `json:"skillLoaded,omitempty"`
	Providers   []string       `json:"providers,omitempty"`
	Prompt      string         `json:"prompt"`
	Started     time.Time      `json:"started"`
	Elapsed     time.Duration  `json:"elapsed"`
}

// AllOps flattens every operation in call order, each with its call id.
func (r *Run) AllOps() []CallOp {
	var out []CallOp
	for _, c := range r.Calls {
		if c.Rejected {
			continue
		}
		for i := range c.Ops {
			out = append(out, CallOp{Call: c, OpRecord: &c.Ops[i]})
		}
	}
	return out
}

// CallOp pairs an operation with the call that performed it.
type CallOp struct {
	Call *Call
	*OpRecord
}

// Call returns the call with id, or nil.
func (r *Run) CallByID(id string) *Call {
	for _, c := range r.Calls {
		if c.ID == id {
			return c
		}
	}
	return nil
}
