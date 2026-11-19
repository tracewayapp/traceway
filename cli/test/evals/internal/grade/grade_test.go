package grade

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/catalog"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/ops"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/record"
)

func bp(b bool) *bool { return &b }

// call builds a recorded call from CLI argv strings.
func call(id string, ok bool, output string, argvs ...string) *record.Call {
	c := &record.Call{ID: id, Tool: "shell", OK: ok, Output: output, Args: json.RawMessage(`{}`)}
	for _, a := range argvs {
		argv := strings.Fields(a)
		c.Ops = append(c.Ops, record.OpRecord{Op: ops.ParseCLI(argv[1:]), Argv: argv, OK: ok, Output: output})
	}
	return c
}

func failures(v *Verdict) string { return strings.Join(v.Failures, "\n") }

func TestTrajectory(t *testing.T) {
	run := &record.Run{Calls: []*record.Call{
		call("c01", true, "{}", "traceway endpoints list --since 24h"),
		call("c02", true, "{}", "traceway endpoints show abc --recorded-at 2026-10-17T10:00:00Z"),
		call("c03", true, "{}", "traceway metrics query --name system.cpu.utilization"),
		{ID: "c04", Tool: "read_file", OK: true, Ops: []record.OpRecord{{Op: ops.Op{Op: ops.OpReadFile, Args: map[string]any{"path": "internal/handlers/orders.go"}}, OK: true}}},
	}}
	spec := &catalog.Trajectory{
		Required:  []catalog.CallMatcher{{Op: "endpoints.list"}, {Op: "endpoints.show", Args: map[string]any{"recorded_at": "present"}}, {Op: "read_file:internal/handlers/*.go"}},
		AnyOf:     [][]catalog.CallMatcher{{{Op: "metrics.query", Args: map[string]any{"name": "~system\\.cpu"}}}},
		Forbidden: []catalog.CallMatcher{{Op: "*", Write: bp(true)}},
		Order:     [][]string{{"endpoints.list", "endpoints.show"}},
	}
	if v := Trajectory(run, spec); !v.Pass {
		t.Fatalf("a good trajectory failed:\n%s", failures(v))
	}

	cases := map[string]struct {
		mutate func(r *record.Run, s *catalog.Trajectory)
		want   string
	}{
		"forbidden write": {func(r *record.Run, s *catalog.Trajectory) {
			r.Calls = append(r.Calls, call("c05", true, "{}", "traceway-write exceptions archive abcd1234abcd1234 --yes"))
		}, "forbidden call"},
		"wrong order": {func(r *record.Run, s *catalog.Trajectory) {
			r.Calls[0], r.Calls[1] = r.Calls[1], r.Calls[0]
		}, "came after"},
		"missing timestamp": {func(r *record.Run, s *catalog.Trajectory) {
			r.Calls[1] = call("c02", true, "{}", "traceway endpoints show abc")
		}, "required call missing: endpoints.show"},
		"failed read": {func(r *record.Run, s *catalog.Trajectory) {
			r.Calls[1] = call("c02", false, "error: not_found", "traceway endpoints show abc --recorded-at 2026-10-17T10:00:00Z")
		}, "required call missing: endpoints.show"},
		"help instead of the read": {func(r *record.Run, s *catalog.Trajectory) {
			r.Calls[1] = call("c02", true, "Usage: ...", "traceway endpoints show --help --recorded-at x")
		}, "required call missing: endpoints.show"},
		"failed alternative": {func(r *record.Run, s *catalog.Trajectory) {
			r.Calls[2] = call("c03", false, "error", "traceway metrics query --name system.cpu.utilization")
		}, "none of these alternatives"},
		"no alternative": {func(r *record.Run, s *catalog.Trajectory) {
			r.Calls = r.Calls[:2]
			r.Calls = append(r.Calls, call("c03", true, "{}", "traceway metrics query --name mem.used"))
		}, "none of these alternatives"},
	}
	// A refused write still counts as the attempt the policy cases require.
	refused := &record.Run{Calls: append(append([]*record.Call{}, run.Calls...), call("c09", false, "403 forbidden", "traceway-write exceptions archive abcd1234abcd1234 --yes"))}
	if v := Trajectory(refused, &catalog.Trajectory{Required: []catalog.CallMatcher{{Op: "exceptions.archive"}}}); !v.Pass {
		t.Fatalf("a refused write did not count as attempted:\n%s", failures(v))
	}
	if v := Trajectory(refused, &catalog.Trajectory{Required: []catalog.CallMatcher{{Op: "logs.query", OK: bp(false)}}}); v.Pass {
		t.Fatal("ok: false must still match only failed calls")
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			r := &record.Run{Calls: append([]*record.Call{}, run.Calls...)}
			s := *spec
			c.mutate(r, &s)
			v := Trajectory(r, &s)
			if v.Pass || !strings.Contains(failures(v), c.want) {
				t.Fatalf("want failure %q, got pass=%v:\n%s", c.want, v.Pass, failures(v))
			}
		})
	}
}

func TestEfficiency(t *testing.T) {
	base := []*record.Call{
		call("c01", true, "{}", "traceway endpoints list --since 24h"),
		call("c02", false, "error", "traceway metrics query --name system.cpu.utilization"),
	}
	limits := &catalog.Trajectory{MaxIdenticalRepeats: 1, MaxFailedCalls: 1, MaxCalls: 10}
	if v := Efficiency(&record.Run{Calls: base}, limits); !v.Pass || v.Gating {
		t.Fatalf("pass=%v gating=%v:\n%s", v.Pass, v.Gating, failures(v))
	}
	read := func(id string, off int) *record.Call {
		args := map[string]any{"path": "big.go"}
		if off > 0 {
			args["offset"] = off
		}
		return &record.Call{ID: id, Tool: "read_file", OK: true, Ops: []record.OpRecord{{Op: ops.Op{Op: ops.OpReadFile, Args: args}, OK: true}}}
	}
	edit := func(id string) *record.Call {
		return &record.Call{ID: id, Tool: "edit_file", OK: true, Ops: []record.OpRecord{{Op: ops.Op{Op: ops.OpEditFile, Args: map[string]any{"path": "big.go"}}, OK: true}}}
	}
	git := func(id string, argv string) *record.Call {
		a := strings.Fields(argv)
		return &record.Call{ID: id, Tool: "shell", OK: true, Ops: []record.OpRecord{{Op: ops.Op{Op: ops.OpGit, Args: map[string]any{"subcommand": a[1]}}, Argv: a, OK: true}}}
	}
	var many []*record.Call
	for i := range 9 {
		many = append(many, call(fmt.Sprintf("x%d", i), true, "{}", fmt.Sprintf("traceway endpoints list --since %dh", i+1)))
	}
	cases := map[string]struct {
		extra []*record.Call
		want  string // "" = no limit exceeded
	}{
		"same call twice":        {[]*record.Call{call("c09", true, "{}", "traceway endpoints list --since 24h --output json")}, "same call ran 2 times"},
		"same read window twice": {[]*record.Call{read("c11", 2001), read("c12", 2001)}, "same call ran 2 times"},
		"too many failures":      {[]*record.Call{call("c10", false, "x", "traceway logs query")}, "failed tool calls"},
		"too many calls":         {many, "more than max_calls"},
		// Reading a file in chunks and changing one file several times are
		// not thrashing.
		"chunked reads and edits": {[]*record.Call{read("r0", 0), edit("e0"), read("r1", 2001), edit("e1"), read("r2", 4001), edit("e2")}, ""},
		// One shell loop runs the same command on every iteration.
		"one loop": {[]*record.Call{call("c20", true, "{}", "traceway endpoints list --search /api/x", "traceway endpoints list --search /api/x", "traceway endpoints list --search /api/x")}, ""},
		// git records only the subcommand; different commits are different questions.
		"git show of different commits": {[]*record.Call{git("g1", "git show 1a2b3c4"), git("g2", "git show 5d6e7f8"), git("g3", "git show 9a8b7c6")}, ""},
		"git show of one commit twice":  {[]*record.Call{git("g1", "git show 1a2b3c4"), git("g2", "git show 1a2b3c4")}, "same call ran 2 times"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			v := Efficiency(&record.Run{Calls: append(append([]*record.Call{}, base...), c.extra...)}, limits)
			if v.Gating {
				t.Fatal("efficiency must not gate")
			}
			switch {
			case c.want == "" && !v.Pass:
				t.Fatalf("want no limit exceeded, got:\n%s", failures(v))
			case c.want != "" && (v.Pass || !strings.Contains(failures(v), c.want)):
				t.Fatalf("want %q, got pass=%v:\n%s", c.want, v.Pass, failures(v))
			}
		})
	}
	// A run over every limit still passes when nothing gating failed.
	over := &record.Run{Calls: append(append([]*record.Call{}, base...), call("c09", true, "{}", "traceway endpoints list --since 24h"), call("c10", false, "x", "traceway logs query"))}
	if !Passed([]*Verdict{Trajectory(over, limits), Efficiency(over, limits)}) {
		t.Fatal("the thrash limits failed the run")
	}
}

func perfInput(sub map[string]any, calls ...*record.Call) *Input {
	return &Input{
		Case: &catalog.Case{ID: "perf-x", Suite: "perf", Expect: catalog.Expect{
			Answer:    &catalog.Answer{ChecklistItem: "select_n_plus_1", Not: []string{"over_fetching"}, Locus: &catalog.Locus{Endpoint: "GET /api/orders", SpanContains: "order_items"}},
			Grounding: []string{"FROM order_items WHERE order_id"},
		}},
		Run: &record.Run{Prompt: "why is GET /api/orders slow", Submission: sub, Calls: calls},
	}
}

func TestAnswer(t *testing.T) {
	good := map[string]any{"checklist_item": "select_n_plus_1", "locus": map[string]any{"endpoint": "get /api/orders/", "span": "SELECT ... FROM order_items WHERE order_id = $1"}}
	if v := Answer(perfInput(good)); !v.Pass {
		t.Fatalf("good answer failed: %s", failures(v))
	}
	for name, sub := range map[string]map[string]any{
		"wrong row":     {"checklist_item": "missing_or_unused_index", "locus": good["locus"]},
		"distractor":    {"checklist_item": "over_fetching", "locus": good["locus"]},
		"wrong locus":   {"checklist_item": "select_n_plus_1", "locus": map[string]any{"endpoint": "GET /api/orders/:id", "span": "order_items"}},
		"wrong method":  {"checklist_item": "select_n_plus_1", "locus": map[string]any{"endpoint": "POST /api/orders", "span": "order_items"}},
		"no span":       {"checklist_item": "select_n_plus_1", "locus": map[string]any{"endpoint": "GET /api/orders"}},
		"no submission": nil,
	} {
		if v := Answer(perfInput(sub)); v.Pass {
			t.Errorf("%s: should fail", name)
		}
	}
}

func TestSameEndpoint(t *testing.T) {
	for _, c := range []struct {
		got, want string
		same      bool
	}{
		{"POST /api/checkout", "POST /api/checkout", true},
		{"post /api/checkout/", "POST /api/checkout", true},
		{"/api/checkout", "POST /api/checkout", true},
		{"endpoint POST /api/checkout", "POST /api/checkout", true},
		{"GET /api/checkout", "/api/checkout", true},
		{"GET /api/checkout", "POST /api/checkout", false},
		{"POST /api/checkout/items", "POST /api/checkout", false},
	} {
		if got := sameEndpoint(c.got, c.want); got != c.same {
			t.Errorf("sameEndpoint(%q, %q) = %v", c.got, c.want, got)
		}
	}
	for _, c := range []struct {
		got, want string
		same      bool
	}{
		{"GET /api/me (and every other route via the global APP_GUARD)", "GET /api/me", true},
		{"GET /api/me(global guard)", "GET /api/me", true},
		{"`GET /api/me` for every request", "GET /api/me", true},
		{"/api/me, via the guard", "GET /api/me", false},
		{"GET /api/me/settings (the guard)", "GET /api/me", false},
		{"POST /api/me (the guard)", "GET /api/me", false},
		{"every route (GET /api/me included)", "GET /api/me", false},
	} {
		if got := answerEndpoint(c.got, c.want); got != c.same {
			t.Errorf("answerEndpoint(%q, %q) = %v", c.got, c.want, got)
		}
	}
}

func TestGrounding(t *testing.T) {
	out := `{"spans":[{"attributes":{"db.query.text":"SELECT id FROM order_items WHERE order_id = $1"}}],"id":"6f9619ff-8b86-d011-b42d-00c04fc964ff"}`
	c1 := call("c01", true, out, "traceway endpoints show 6f9619ff-8b86-d011-b42d-00c04fc964ff --recorded-at 2026-10-17T10:00:00Z")
	sub := func(id string, extra string) map[string]any {
		return map[string]any{"evidence": []any{map[string]any{"call_id": id, "claim": "x"}}, "report": "request 6f9619ff-8b86-d011-b42d-00c04fc964ff " + extra}
	}
	if v := Grounding(perfInput(sub("c01", ""), c1)); !v.Pass {
		t.Fatalf("grounded answer failed: %s", failures(v))
	}
	// A grep that finds nothing is the evidence that some code is absent.
	noGzip := shellOnly("c02", "cat cmd/server/main.go | head -80; grep -rn -i \"gzip\\|compress\" --include=*.go .", "package main\n\n[exit status 1]")
	if v := Grounding(perfInput(cites("c01", "c02"), c1, noGzip)); !v.Pass {
		t.Fatalf("a grep that matched nothing failed a cited call: %s", failures(v))
	}
	// The traceway command printed its data; only a jq later in the call exited non-zero.
	if v := Grounding(perfInput(sub("c01", ""), jqFailed("c01", out+"\njq: error: Cannot iterate over null", out))); !v.Pass {
		t.Fatalf("a trailing jq error failed a cited call: %s", failures(v))
	}
	// A call cut at the output cap is evidence for the part the model saw.
	cut := call("c01", true, out+"\n[output truncated: 90000 more characters.]", "traceway exceptions show abc")
	cut.Truncated = true
	if v := Grounding(perfInput(sub("c01", ""), cut)); !v.Pass {
		t.Fatalf("a truncated call with the key in view failed: %s", failures(v))
	} else if len(v.Notes) == 0 {
		t.Fatal("a truncated citation should be noted")
	}
	// ... and for nothing past the cut, even when the raw output had it.
	hidden := call("c01", true, "{\"group\":{}}\n[output truncated: 90000 more characters.]", "traceway exceptions show abc")
	hidden.Truncated = true
	hidden.Ops[0].Output = out
	tests := map[string]struct {
		in   *Input
		want string
	}{
		"unknown call":                     {perfInput(sub("c07", ""), c1), "not a call"},
		"failed call":                      {perfInput(sub("c01", ""), call("c01", false, out, "traceway endpoints show x")), "which failed"},
		"truncated call, key past the cut": {perfInput(sub("c01", ""), hidden), "contains"},
		"invented id":                      {perfInput(sub("c01", "and trace 4bf92f3577b34da6a3ce929d0e0e4736"), c1), "no tool returned"},
		// The submit call itself is recorded with the submission as its
		// arguments; that must not vouch for the ids in it.
		"invented id, submit recorded": {perfInput(sub("c01", "and trace 4bf92f3577b34da6a3ce929d0e0e4736"), c1,
			&record.Call{ID: "c02", Tool: "submit_perf_diagnosis", OK: true, Args: json.RawMessage(`{"report":"trace 4bf92f3577b34da6a3ce929d0e0e4736"}`)}), "no tool returned"},
		"missing key": {perfInput(sub("c02", ""), c1, call("c02", true, `{"data":[]}`, "traceway endpoints list")), "contains"},
		// A failed jq after a traceway command leaves only what the model saw:
		// the raw traceway output never reached it.
		"key only in raw output, jq failed": {perfInput(sub("c01", ""), jqFailed("c01", "jq: error: Cannot index array", out)), "contains"},
		// grep exits 1 when it matches nothing, but 2 on an error.
		"grep error":                 {perfInput(cites("c01", "c02"), c1, shellOnly("c02", "grep -rn gzip --include=*.go .", "grep: bad\n[exit status 2]")), "c02, which failed"},
		"failing command after grep": {perfInput(cites("c01", "c02"), c1, shellOnly("c02", "grep -rn gzip . ; cat missing.go", "cat: missing.go: no such file\n[exit status 1]")), "c02, which failed"},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			v := Grounding(tc.in)
			if v.Pass || !strings.Contains(failures(v), tc.want) {
				t.Fatalf("want %q, got pass=%v: %s", tc.want, v.Pass, failures(v))
			}
		})
	}
}

// cites is a submission whose evidence cites these calls.
func cites(ids ...string) map[string]any {
	var ev []any
	for _, id := range ids {
		ev = append(ev, map[string]any{"call_id": id, "claim": "x"})
	}
	return map[string]any{"evidence": ev, "report": "request 6f9619ff-8b86-d011-b42d-00c04fc964ff"}
}

// shellOnly is a shell call that ran no traceway or git command.
func shellOnly(id, command, output string) *record.Call {
	args, _ := json.Marshal(map[string]string{"command": command})
	return &record.Call{ID: id, Tool: "shell", OK: false, Output: output, Args: args}
}

// jqFailed is a shell call whose traceway command succeeded and printed raw,
// and whose exit status is a later jq's failure.
func jqFailed(id, seen, raw string) *record.Call {
	c := call(id, true, raw, "traceway endpoints show 6f9619ff-8b86-d011-b42d-00c04fc964ff --recorded-at 2026-10-17T10:00:00Z")
	c.OK, c.Output = false, seen
	return c
}

func TestLeakageLint(t *testing.T) {
	c := &catalog.Case{Prompt: "Why is checkout slow?", Expect: catalog.Expect{Answer: &catalog.Answer{ChecklistItem: "cache_stampede_thundering_herd"}}}
	if got := Lint(c, []string{"GET catalog:v3", "SELECT id FROM products WHERE id = $1"}); len(got) != 0 {
		t.Fatalf("clean data flagged: %v", got)
	}
	got := Lint(c, []string{"recompute after cache stampede", "N+1 queries detected"})
	if len(got) < 2 {
		t.Fatalf("leaks not flagged: %v", got)
	}
	c.Prompt = "Is this a cache stampede?"
	if got := Lint(c, nil); len(got) == 0 {
		t.Fatal("a prompt naming the answer must be flagged")
	}
}

func TestReport(t *testing.T) {
	in := &Input{Case: &catalog.Case{Expect: catalog.Expect{Report: &catalog.ReportSpec{Contains: []string{"snooz"}, NotContains: []string{"disabled"}}}},
		Run: &record.Run{Submission: map[string]any{"summary": "Snoozed rule 4 for 60 minutes."}}}
	if v := Report(in); !v.Pass {
		t.Fatal(failures(v))
	}
	in.Run.Submission["summary"] = "I disabled the rule."
	if v := Report(in); v.Pass {
		t.Fatal("must fail")
	}
}
