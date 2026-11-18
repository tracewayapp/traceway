package guard

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestEvalsWorkflowIsDispatchOnly is the manual-only trigger test: the
// repository's cli-evals.yml has workflow_dispatch as its only trigger and no
// other workflow references it or starts a paid run.
func TestEvalsWorkflowIsDispatchOnly(t *testing.T) {
	dir := filepath.Join("..", "..", "..", "..", "..", ".github", "workflows")
	if err := CheckWorkflows(dir); err != nil {
		t.Fatal(err)
	}
}

// TestJustfileNeverRunsPaidEvals holds cli/justfile to the same rule: no
// recipe reachable from default, check or test runs or dispatches the evals.
func TestJustfileNeverRunsPaidEvals(t *testing.T) {
	if err := CheckJustfile(filepath.Join("..", "..", "..", "..", "justfile")); err != nil {
		t.Fatal(err)
	}
}

func TestCheckJustfileRejectsPaidRecipes(t *testing.T) {
	base := "default: test\n\ntest:\n    go test ./...\n\neval-local *args:\n    cd test/evals && go run ./cmd/twevals run {{args}}\n\neval-dispatch *args:\n    gh workflow run cli-evals.yml {{args}}\n"
	cases := map[string]struct {
		file string
		want string
	}{
		"good":            {file: base},
		"dependency":      {file: strings.Replace(base, "test:\n", "test: eval-local\n", 1), want: "reaches the recipe eval-local"},
		"nested just":     {file: strings.Replace(base, "    go test ./...\n", "    go test ./...\n    just eval-dispatch -f max_cost_usd=5\n", 1), want: "reaches the recipe test"},
		"inline paid run": {file: base + "\ncheck: lint\n\nlint:\n    cd test/evals && go run ./cmd/twevals \\\n      run --max-cost-usd 5\n", want: "reaches the recipe lint"},
		"alias":           {file: base + "\nalias el := eval-local\n\ncheck: el\n", want: "reaches the recipe eval-local"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "justfile")
			if err := os.WriteFile(path, []byte(c.file), 0o644); err != nil {
				t.Fatal(err)
			}
			err := CheckJustfile(path)
			if c.want == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("want error containing %q, got %v", c.want, err)
			}
		})
	}
}

func TestCheckWorkflowsRejectsAutomation(t *testing.T) {
	good := `name: CLI Evals
on:
  workflow_dispatch:
    inputs:
      max_cost_usd: {type: string, required: true}
permissions:
  contents: read
jobs:
  preflight:
    runs-on: ubuntu-latest
    steps:
      - run: go run ./cmd/twevals plan --max-cost-usd 1
  evals:
    environment: evals
    runs-on: ubuntu-latest
    env:
      TWEVALS_APPROVED: "1"
    steps:
      - run: go run ./cmd/twevals run --yes --max-cost-usd 1
`
	cases := map[string]struct {
		evals  string
		others map[string]string
		want   string
	}{
		"good":             {evals: good},
		"push trigger":     {evals: strings.Replace(good, "on:\n", "on:\n  push:\n", 1), want: "exactly one trigger"},
		"schedule":         {evals: strings.Replace(good, "on:\n", "on:\n  schedule:\n    - cron: '0 0 * * *'\n", 1), want: "exactly one trigger"},
		"workflow_call":    {evals: strings.Replace(good, "on:\n", "on:\n  workflow_call:\n", 1), want: "exactly one trigger"},
		"list of triggers": {evals: strings.Replace(good, "on:\n  workflow_dispatch:\n    inputs:\n      max_cost_usd: {type: string, required: true}\n", "on: [workflow_dispatch, push]\n", 1), want: "exactly [workflow_dispatch]"},
		"ungated paid job": {evals: strings.Replace(good, "    environment: evals\n", "", 1), want: "without environment: evals"},
		"caller":           {evals: good, others: map[string]string{"ci.yml": "jobs:\n  x:\n    uses: ./.github/workflows/cli-evals.yml\n"}, want: "references the evals workflow"},
		"dispatcher":       {evals: good, others: map[string]string{"nightly.yml": "on: schedule\njobs:\n  x:\n    steps:\n      - run: gh workflow run cli-evals.yml\n"}, want: "references the evals workflow"},
		"other paid run":   {evals: good, others: map[string]string{"cli.yml": "jobs:\n  x:\n    steps:\n      - run: cd cli/test/evals && go run ./cmd/twevals run --yes\n"}, want: "starts a paid eval run"},
		"continued line": {evals: strings.Replace(good, "      - run: go run ./cmd/twevals plan --max-cost-usd 1\n",
			"      - run: |\n          go run ./cmd/twevals \\\n            run --yes --max-cost-usd 1\n", 1), want: "without environment: evals"},
		"workflow env": {evals: strings.Replace(good, "permissions:\n", "env:\n  TWEVALS_APPROVED: \"1\"\npermissions:\n", 1), want: "workflow-level env"},
		"duplicate on": {evals: good + "on: push\n", want: "appears twice"},
		"by name":      {evals: good, others: map[string]string{"nightly.yml": "on: schedule\njobs:\n  x:\n    steps:\n      - run: gh workflow run \"CLI Evals\" -f max_cost_usd=5\n"}, want: "references the evals workflow"},
		"by recipe":    {evals: good, others: map[string]string{"nightly.yml": "on: schedule\njobs:\n  x:\n    steps:\n      - run: cd cli && just eval-dispatch -f max_cost_usd=5\n"}, want: "references the evals workflow"},
		"evals env":    {evals: good, others: map[string]string{"other.yml": "on: push\njobs:\n  x:\n    environment:\n      name: evals\n    steps:\n      - run: echo\n"}, want: "uses the evals environment"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "workflows")
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "cli-evals.yml"), []byte(c.evals), 0o644); err != nil {
				t.Fatal(err)
			}
			for f, body := range c.others {
				if err := os.WriteFile(filepath.Join(dir, f), []byte(body), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			err := CheckWorkflows(dir)
			if c.want == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("want error containing %q, got %v", c.want, err)
			}
		})
	}
}
