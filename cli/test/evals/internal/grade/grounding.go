package grade

import (
	"encoding/json"
	"regexp"
	"sort"
	"strings"

	"mvdan.cc/sh/v3/syntax"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/record"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/tools"
)

// Grounding checks that the evidence is real:
//   - every evidence.call_id names a call that exists and succeeded (a shell
//     call succeeded when one of its operations did, even if a later command
//     in it, such as jq, failed);
//   - every grounding key appears in the output of at least one cited call
//     (what the model saw, or the raw traceway output inside that call). A
//     call whose output was cut at the cap is evidence for the part the
//     model saw, and for nothing past the cut;
//   - every UUID, 32-hex trace id or 16-hex hash in the answer appears in
//     some tool output or the prompt: no invented ids.
func Grounding(in *Input) *Verdict {
	v := newVerdict("grounding", true)
	run := in.Run
	sub := run.Submission
	if sub == nil {
		v.fail("no submission to ground")
		return v
	}
	var cited []*record.Call
	visibleOnly := map[*record.Call]bool{}
	if ev, ok := sub["evidence"].([]any); ok {
		for i, e := range ev {
			m, _ := e.(map[string]any)
			id, _ := m["call_id"].(string)
			c := run.CallByID(strings.TrimSpace(id))
			switch {
			case c == nil:
				v.fail("evidence[%d] cites %q, which is not a call", i, id)
			case c.Rejected:
				v.fail("evidence[%d] cites %s, which was never executed", i, id)
			case !anyOpOK(c) || (len(c.Ops) == 0 && !c.OK && !grepFoundNothing(c)):
				v.fail("evidence[%d] cites %s, which failed", i, id)
			case c.Truncated:
				v.note("evidence[%d] cites %s, whose output was truncated; only the part the model saw counts", i, id)
				visibleOnly[c] = true
				cited = append(cited, c)
			default:
				cited = append(cited, c)
			}
		}
	} else if in.Case.Suite == "perf" || in.Case.Suite == "debug" {
		v.fail("the submission cites no evidence")
	}
	for _, key := range in.Case.Expect.Grounding {
		found := false
		for _, c := range cited {
			if visibleOnly[c] && strings.Contains(unescapeJSON(c.Output), key) || !visibleOnly[c] && callContains(c, key) {
				found = true
				break
			}
		}
		if !found {
			v.fail("no cited call's output contains %q", key)
		}
	}
	if invented := InventedIDs(run); len(invented) > 0 {
		v.fail("ids in the answer that no tool returned: %s", strings.Join(invented, ", "))
	}
	return v
}

// callContains looks for a grounding key in what the model saw and in the
// raw output of the call's successful operations. When a later command in the
// call failed, only what the model saw counts: the raw output may never have
// reached it. MCP results are JSON, which escapes &, < and > (\u0026, \u003c,
// \u003e), so those are compared unescaped.
func callContains(c *record.Call, key string) bool {
	if !anyOpOK(c) {
		return false
	}
	if strings.Contains(unescapeJSON(c.Output), key) {
		return true
	}
	if !c.OK {
		return false
	}
	for _, op := range c.Ops {
		if op.OK && strings.Contains(unescapeJSON(op.Output), key) {
			return true
		}
	}
	return false
}

// anyOpOK reports whether a call has no operations (a plain shell command)
// or at least one that succeeded. A traceway command that failed inside a
// pipeline does not make the call evidence just because jq exited 0.
func anyOpOK(c *record.Call) bool {
	if len(c.Ops) == 0 {
		return true
	}
	for _, op := range c.Ops {
		if op.OK {
			return true
		}
	}
	return false
}

// grepFoundNothing reports whether a shell call failed only because its last
// command was a grep that matched nothing (exit 1; a grep error is 2). "No
// such code" is evidence too.
func grepFoundNothing(c *record.Call) bool {
	if c.Tool != "shell" || !strings.HasSuffix(strings.TrimSpace(c.Output), "[exit status 1]") {
		return false
	}
	var in struct {
		Command string `json:"command"`
	}
	if json.Unmarshal(c.Args, &in) != nil {
		return false
	}
	file, err := syntax.NewParser(syntax.Variant(syntax.LangBash)).Parse(strings.NewReader(in.Command), "")
	if err != nil || len(file.Stmts) == 0 {
		return false
	}
	cmd := file.Stmts[len(file.Stmts)-1].Cmd
	for {
		b, ok := cmd.(*syntax.BinaryCmd)
		if !ok || (b.Op != syntax.Pipe && b.Op != syntax.PipeAll) {
			break
		}
		cmd = b.Y.Cmd
	}
	call, ok := cmd.(*syntax.CallExpr)
	if !ok || len(call.Args) == 0 {
		return false
	}
	switch call.Args[0].Lit() {
	case "grep", "egrep", "fgrep":
		return true
	}
	return false
}

var jsonEscapes = strings.NewReplacer(`\u0026`, "&", `\u003c`, "<", `\u003e`, ">", `\u0027`, "'")

func unescapeJSON(s string) string {
	if !strings.Contains(s, `\u00`) {
		return s
	}
	return jsonEscapes.Replace(s)
}

var idRe = regexp.MustCompile(`(?i)\b(?:[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}|[0-9a-f]{32}|[0-9a-f]{16})\b`)

// InventedIDs lists ids in the submission (and final text) that appear in no
// tool output and not in the prompt. The submit calls are left out of the
// corpus: their arguments are the submission itself.
func InventedIDs(run *record.Run) []string {
	raw, _ := json.Marshal(run.Submission)
	text := string(raw) + "\n" + run.FinalText
	var corpus strings.Builder
	corpus.WriteString(strings.ToLower(run.Prompt))
	for _, c := range run.Calls {
		if tools.IsSubmit(c.Tool) {
			continue
		}
		corpus.WriteString(strings.ToLower(c.Output))
		corpus.WriteString(strings.ToLower(string(c.Args)))
		for _, op := range c.Ops {
			corpus.WriteString(strings.ToLower(op.Output))
		}
	}
	all := corpus.String()
	seen := map[string]bool{}
	var out []string
	for _, id := range idRe.FindAllString(text, -1) {
		l := strings.ToLower(id)
		if seen[l] || isAllDigits(l) {
			continue
		}
		seen[l] = true
		if !strings.Contains(all, l) && !strings.Contains(all, strings.ReplaceAll(l, "-", "")) {
			out = append(out, id)
		}
	}
	sort.Strings(out)
	return out
}

func isAllDigits(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
