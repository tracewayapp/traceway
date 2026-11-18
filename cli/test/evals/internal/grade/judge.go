package grade

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/agent"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/catalog"
)

// Judge scores the final report against the case rubric with an LLM (Claude
// Haiku through OpenRouter, no tools, strict JSON). It never gates until
// calibrated against human labels; its cost counts toward the
// dispatch's cap.
type Judge struct {
	Chat   agent.Chatter
	Model  *catalog.Model
	Ledger *agent.Ledger
	// Price estimates the judge's cost when the provider reports none.
	Price catalog.Price
}

const judgeSystem = `You grade an AI agent's work against a rubric. You see the user's request, the expected outcome (hidden from the agent), the agent's final report and structured answer, and a condensed log of the tool calls it made.
For each rubric criterion decide pass, fail, or unknown (the report and evidence do not let you decide). Judge only what the report and evidence show; do not reward confident claims the evidence does not support.
Answer with JSON only, exactly this shape: {"criteria":[{"id":1,"verdict":"pass","reason":"one sentence"}]}`

// Grade runs the judge. A nil judge or an oracle run is skipped.
func (j *Judge) Grade(ctx context.Context, in *Input) *Verdict {
	v := newVerdict("judge", false)
	if j == nil || in.FakeModel {
		v.Skipped = true
		v.note("skipped (no judge for oracle runs, or --judge=false)")
		return v
	}
	if j.Ledger.Exhausted() {
		v.Skipped = true
		v.note("skipped: cost ceiling reached")
		return v
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Case: %s\n\nThe user asked:\n%s\n\n", in.Case.Title, in.Run.Prompt)
	if in.Case.Judge.Expected != "" {
		fmt.Fprintf(&b, "Expected outcome (hidden from the agent):\n%s\n\n", in.Case.Judge.Expected)
	}
	b.WriteString("Rubric:\n")
	for i, r := range in.Case.Judge.Rubric {
		fmt.Fprintf(&b, "%d. %s\n", i+1, r)
	}
	fmt.Fprintf(&b, "\nAgent's final report:\n%s\n", FinalText(in.Run))
	if in.Run.Submission != nil {
		sub := map[string]any{}
		for k, val := range in.Run.Submission {
			if k != "report" && k != "summary" {
				sub[k] = val
			}
		}
		raw, _ := json.MarshalIndent(sub, "", "  ")
		fmt.Fprintf(&b, "\nStructured answer:\n%s\n", raw)
	}
	b.WriteString("\nTool calls (condensed):\n")
	for _, c := range in.Run.Calls {
		out := c.Output
		if len(out) > 700 {
			out = out[:700] + "..."
		}
		var opNames []string
		for _, op := range c.Ops {
			opNames = append(opNames, op.Op.Op)
		}
		fmt.Fprintf(&b, "- %s %s [%s] ok=%v: %s\n", c.ID, c.Tool, strings.Join(opNames, ","), c.OK, strings.ReplaceAll(out, "\n", " "))
	}
	if len(in.Run.Questions) > 0 {
		b.WriteString("\nQuestions the agent asked the user:\n")
		for _, q := range in.Run.Questions {
			fmt.Fprintf(&b, "- Q: %s\n  A: %s\n", clipS(q.Question, 1500), q.Answer)
		}
	}
	req := &agent.Request{
		Model: j.Model.ID,
		Messages: []agent.Message{
			{Role: "system", Content: judgeSystem},
			{Role: "user", Content: b.String()},
		},
		MaxTokens:      j.Model.MaxTokensPerTurn,
		Temperature:    j.Model.Temperature,
		ResponseFormat: map[string]any{"type": "json_object"},
		Usage:          map[string]any{"include": true},
	}
	if len(j.Model.Provider.Order) > 0 {
		req.Provider = &agent.ProviderPrefs{Order: j.Model.Provider.Order, AllowFallbacks: j.Model.Provider.AllowFallbacks}
	}
	resp, err := j.Chat.Chat(ctx, req)
	if err != nil {
		v.Skipped = true
		v.note("judge call failed: %v", err)
		return v
	}
	v.CostUSD = agent.UsageOf(resp, j.Price).CostUSD
	if in.Hold != nil {
		in.Hold.Add("judge", v.CostUSD)
	} else {
		j.Ledger.Add("judge", v.CostUSD)
	}
	text, _ := resp.Choices[0].Message.Content.(string)
	var out struct {
		Criteria []struct {
			ID      int    `json:"id"`
			Verdict string `json:"verdict"`
			Reason  string `json:"reason"`
		} `json:"criteria"`
	}
	if err := json.Unmarshal([]byte(extractJSON(text)), &out); err != nil {
		v.Skipped = true
		v.note("judge returned no JSON: %s", clipS(text, 300))
		return v
	}
	pass, fail := 0, 0
	for _, c := range out.Criteria {
		switch c.Verdict {
		case "pass":
			pass++
		case "fail":
			fail++
			v.Failures = append(v.Failures, fmt.Sprintf("criterion %d: %s", c.ID, c.Reason))
		default:
			v.note("criterion %d unknown: %s", c.ID, c.Reason)
		}
	}
	if pass+fail > 0 {
		s := float64(pass) / float64(pass+fail)
		v.Score = &s
		v.Pass = s >= 0.5
	} else {
		v.Skipped = true
	}
	return v
}

func extractJSON(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	if i := strings.Index(s, "{"); i > 0 {
		s = s[i:]
	}
	return strings.TrimSpace(s)
}
