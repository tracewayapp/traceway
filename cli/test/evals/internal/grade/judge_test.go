package grade

import (
	"context"
	"testing"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/agent"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/catalog"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/record"
)

type judgeChat struct{ reply string }

func (j judgeChat) Chat(context.Context, *agent.Request) (*agent.Response, error) {
	r := &agent.Response{}
	r.Choices = append(r.Choices, struct {
		Message      agent.Message `json:"message"`
		FinishReason string        `json:"finish_reason"`
	}{Message: agent.Message{Role: "assistant", Content: j.reply}})
	r.Usage.Cost = 0.002
	return r, nil
}

func TestJudge(t *testing.T) {
	in := &Input{Case: &catalog.Case{Judge: catalog.JudgeSpec{Rubric: []string{"a", "b", "c"}}}, Run: &record.Run{FinalText: "report"}}
	ledger := agent.NewLedger(10)
	j := &Judge{Chat: judgeChat{"```json\n{\"criteria\":[{\"id\":1,\"verdict\":\"pass\"},{\"id\":2,\"verdict\":\"fail\",\"reason\":\"no numbers\"},{\"id\":3,\"verdict\":\"unknown\"}]}\n```"}, Model: &catalog.Model{ID: "anthropic/claude-haiku-5.5"}, Ledger: ledger}
	v := j.Grade(context.Background(), in)
	if v.Gating || v.Skipped || v.Score == nil || *v.Score != 0.5 || len(v.Failures) != 1 {
		t.Fatalf("%+v", v)
	}
	if ledger.Spent() != 0.002 {
		t.Fatalf("judge cost must count toward the cap, spent %v", ledger.Spent())
	}
	in.FakeModel = true
	if v := j.Grade(context.Background(), in); !v.Skipped {
		t.Fatal("oracle runs skip the judge")
	}
}
