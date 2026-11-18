package grade

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/catalog"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/ops"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/record"
)

const renderBefore = `package invoices

func formatAddress(a *Address) string {
	s := a.Line1
	if a.Line2 != "" {
		s += "\n" + a.Line2
	}
	return s
}
`

func TestMapLineBack(t *testing.T) {
	guard := strings.Replace(renderBefore, "\ts := a.Line1", "\tif a == nil {\n\t\treturn \"\"\n\t}\n\ts := a.Line1", 1)
	comment := strings.Replace(renderBefore, "func formatAddress", "// formatAddress renders a billing address.\n// A nil address renders as \"\".\nfunc formatAddress", 1)
	changed := strings.Replace(renderBefore, "\ts := a.Line1", "\ts := safeLine1(a)", 1)
	deleted := strings.Replace(renderBefore, "package invoices\n\n", "", 1)
	for name, tc := range map[string]struct {
		after string
		line  int
		want  int
	}{
		"statement below an inserted guard":   {guard, 7, 4},
		"the inserted guard itself":           {guard, 4, 4},
		"statement below an inserted comment": {comment, 6, 4},
		"the changed statement":               {changed, 4, 4},
		"statement above the edit":            {guard, 3, 3},
		"statement below a deletion":          {deleted, 2, 4},
		"end of file":                         {guard, 12, 9},
	} {
		got, ok := mapLineBack(renderBefore, tc.after, tc.line)
		if !ok || got != tc.want {
			t.Errorf("%s: mapLineBack(line %d) = %d, %v, want %d", name, tc.line, got, ok, tc.want)
		}
	}
	if _, ok := mapLineBack(renderBefore, guard, 40); ok {
		t.Error("a line past the end of the edited file mapped")
	}
}

func TestCulpritLineThroughTheEdit(t *testing.T) {
	root := t.TempDir()
	rel := "internal/invoices/render.go"
	path := filepath.Join(root, "testdata", "repos", "billing-api-go", rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(renderBefore), 0o644); err != nil {
		t.Fatal(err)
	}
	// The guard is four lines, so the statement moves past the ±3 tolerance.
	editArgs, _ := json.Marshal(map[string]any{"path": rel, "old_string": "\ts := a.Line1", "new_string": "\tif a == nil {\n\t\t// Digital-only customers never enter one.\n\t\treturn \"\"\n\t}\n\ts := a.Line1"})
	edit := &record.Call{ID: "c03", Tool: "edit_file", OK: true, Args: editArgs,
		Ops: []record.OpRecord{{Op: ops.Op{Op: ops.OpEditFile, Args: map[string]any{"path": rel}, Write: true}, OK: true}}}
	exp := &catalog.Answer{IssueHash: "e4c2373afd52ef13", Category: "null_reference", Culprit: &catalog.Culprit{File: rel, Symbol: "formatAddress", Line: 4}}
	input := func(calls []*record.Call, sub map[string]any, prompt string) *Input {
		return &Input{EvalsRoot: root, Case: &catalog.Case{ID: "debug-x", Suite: "debug", Workspace: catalog.Workspace{Repo: "billing-api-go"}, Expect: catalog.Expect{Answer: exp}},
			Run: &record.Run{Prompt: prompt, Submission: sub, Calls: calls}}
	}
	sub := func(line int, hash string) map[string]any {
		return map[string]any{"issue_hash": hash, "category": "null_reference", "culprit": map[string]any{"file": rel, "symbol": "formatAddress", "line": line},
			"evidence": []any{map[string]any{"call_id": "c01", "claim": "nil pointer in formatAddress"}}, "report": "formatAddress dereferences a nil address"}
	}
	// Line 8 is the statement's line after the guard went in above it.
	if v := Answer(input([]*record.Call{edit}, sub(8, "e4c2373afd52ef13"), "/traceway debug e4c2373afd52ef13")); !v.Pass {
		t.Fatalf("the line in the edited file failed: %s", failures(v))
	} else if len(v.Notes) == 0 {
		t.Fatal("mapping through the edit should be noted")
	}
	// The same line with no recorded edit is just wrong.
	if v := Answer(input(nil, sub(8, "e4c2373afd52ef13"), "")); v.Pass {
		t.Fatal("line 8 passed without an edit to explain it")
	}
	// A shell redirection wrote the file: its content is unknown, so the line is wrong.
	shellWrite := &record.Call{ID: "c03", Tool: "shell", OK: true, Args: json.RawMessage(`{"command":"sed -i x render.go"}`),
		Ops: []record.OpRecord{{Op: ops.Op{Op: ops.OpWriteFile, Args: map[string]any{"path": rel}}, OK: true}}}
	if v := Answer(input([]*record.Call{shellWrite}, sub(8, "e4c2373afd52ef13"), "")); v.Pass {
		t.Fatal("line 8 passed after an unrecorded shell write")
	}
	// A line far from both files' statement stays wrong.
	if v := Answer(input([]*record.Call{edit}, sub(30, "e4c2373afd52ef13"), "")); v.Pass {
		t.Fatal("line 30 passed")
	}
	// issue_hash: empty is fine when the prompt or the answer names the hash.
	if v := Answer(input([]*record.Call{edit}, sub(4, ""), "/traceway debug e4c2373afd52ef13")); !v.Pass {
		t.Fatalf("an empty issue_hash with the hash in the prompt failed: %s", failures(v))
	}
	named := sub(4, "")
	named["report"] = "Issue e4c2373afd52ef13: formatAddress dereferences a nil address"
	if v := Answer(input([]*record.Call{edit}, named, "the invoice page is broken")); !v.Pass {
		t.Fatalf("an empty issue_hash with the hash in the report failed: %s", failures(v))
	}
	if v := Answer(input([]*record.Call{edit}, sub(4, ""), "the invoice page is broken")); v.Pass {
		t.Fatal("an empty issue_hash passed with the hash nowhere")
	}
	if v := Answer(input([]*record.Call{edit}, sub(4, "aac163b08f71ab3f"), "/traceway debug e4c2373afd52ef13")); v.Pass {
		t.Fatal("another group's hash passed")
	}
}
