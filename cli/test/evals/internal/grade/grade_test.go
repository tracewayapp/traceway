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
		call("c03", false, "error", "traceway metrics query --name system.cpu.utilization"),
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
		"no alternative": {func(r *record.Run, s *catalog.Trajectory) {
			r.Calls = r.Calls[:2]
			r.Calls = append(r.Calls, call("c03", true, "{}", "traceway metrics query --name mem.used"))
		}, "none of these alternatives"},
		"thrash": {func(r *record.Run, s *catalog.Trajectory) {
			s.MaxIdenticalRepeats = 1
			r.Calls = append(r.Calls, call("c09", true, "{}", "traceway endpoints list --since 24h --output json"))
		}, "same call ran 2 times"},
		"thrash, read in chunks of the same window": {func(r *record.Run, s *catalog.Trajectory) {
			s.MaxIdenticalRepeats = 1
			read := func(id string) *record.Call {
				return &record.Call{ID: id, Tool: "read_file", OK: true, Ops: []record.OpRecord{{Op: ops.Op{Op: ops.OpReadFile, Args: map[string]any{"path": "big.go", "offset": 2001}}, OK: true}}}
			}
			r.Calls = append(r.Calls, read("c11"), read("c12"))
		}, "same call ran 2 times"},
		"too many failures": {func(r *record.Run, s *catalog.Trajectory) {
			s.MaxFailedCalls = 0
			s.MaxFailedCalls = 1
			r.Calls = append(r.Calls, call("c10", false, "x", "traceway logs query"))
		}, "failed tool calls"},
	}
	// Reading a file in chunks and changing one file several times are not
	// thrashing.
	chunked := &record.Run{Calls: append([]*record.Call{}, run.Calls...)}
	for i, off := range []int{0, 2001, 4001} {
		args := map[string]any{"path": "big.go"}
		if off > 0 {
			args["offset"] = off
		}
		chunked.Calls = append(chunked.Calls,
			&record.Call{ID: fmt.Sprintf("r%d", i), Tool: "read_file", OK: true, Ops: []record.OpRecord{{Op: ops.Op{Op: ops.OpReadFile, Args: args}, OK: true}}},
			&record.Call{ID: fmt.Sprintf("e%d", i), Tool: "edit_file", OK: true, Ops: []record.OpRecord{{Op: ops.Op{Op: ops.OpEditFile, Args: map[string]any{"path": "big.go"}}, OK: true}}})
	}
	repeats := *spec
	repeats.MaxIdenticalRepeats = 1
	if v := Trajectory(chunked, &repeats); !v.Pass {
		t.Fatalf("chunked reads and repeated edits counted as thrashing:\n%s", failures(v))
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
	tests := map[string]struct {
		in   *Input
		want string
	}{
		"unknown call":   {perfInput(sub("c07", ""), c1), "not a call"},
		"failed call":    {perfInput(sub("c01", ""), call("c01", false, out, "traceway endpoints show x")), "which failed"},
		"truncated call": {perfInput(sub("c01", ""), &record.Call{ID: "c01", OK: true, Truncated: true, Output: out}), "truncated"},
		"invented id":    {perfInput(sub("c01", "and trace 4bf92f3577b34da6a3ce929d0e0e4736"), c1), "no tool returned"},
		// The submit call itself is recorded with the submission as its
		// arguments; that must not vouch for the ids in it.
		"invented id, submit recorded": {perfInput(sub("c01", "and trace 4bf92f3577b34da6a3ce929d0e0e4736"), c1,
			&record.Call{ID: "c02", Tool: "submit_perf_diagnosis", OK: true, Args: json.RawMessage(`{"report":"trace 4bf92f3577b34da6a3ce929d0e0e4736"}`)}), "no tool returned"},
		"missing key": {perfInput(sub("c02", ""), c1, call("c02", true, `{"data":[]}`, "traceway endpoints list")), "contains"},
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
