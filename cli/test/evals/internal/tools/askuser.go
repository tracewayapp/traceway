package tools

import (
	"context"
	"encoding/json"
	"regexp"
	"sync"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/catalog"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/ops"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/record"
)

// AskUser answers questions from the case's scripted user rules: the first
// rule whose regexp matches the question wins, a default rule matches
// anything, and with no rule the user says they have nothing to add.
type AskUser struct {
	Rules []catalog.UserRule

	mu   sync.Mutex
	used map[int]bool
	Log  []record.Question
}

func (t *AskUser) Spec() Spec {
	return Spec{Name: "ask_user", Description: "Ask the user a question and wait for the answer. Use it for decisions only the user can make (confirming a change, choosing between candidates, approving a draft).",
		Parameters: obj(map[string]any{"question": strProp("The question, including any draft or options the user must see.")}, "question")}
}

func (t *AskUser) Call(_ context.Context, raw json.RawMessage) Result {
	var in struct {
		Question string `json:"question"`
	}
	if err := decodeArgs(raw, &in); err != nil {
		return errResult("%v", err)
	}
	answer := t.Answer(in.Question)
	t.mu.Lock()
	t.Log = append(t.Log, record.Question{Question: in.Question, Answer: answer})
	t.mu.Unlock()
	return Result{Output: answer, OK: true, Ops: []record.OpRecord{{Op: ops.Op{Op: ops.OpAskUser, Args: map[string]any{"question": in.Question}}, OK: true, Output: answer}}}
}

// Answer picks the scripted reply for a question.
func (t *AskUser) Answer(q string) string {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.used == nil {
		t.used = map[int]bool{}
	}
	for i, r := range t.Rules {
		if r.Once && t.used[i] {
			continue
		}
		if r.Match != "" {
			re, err := regexp.Compile("(?is)" + r.Match)
			if err != nil || !re.MatchString(q) {
				continue
			}
			t.used[i] = true
			return r.Reply
		}
		if r.Default != "" {
			t.used[i] = true
			return r.Default
		}
	}
	return "I don't have anything to add. Please continue."
}

// Questions returns the exchanges so far.
func (t *AskUser) Questions() []record.Question {
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([]record.Question{}, t.Log...)
}
