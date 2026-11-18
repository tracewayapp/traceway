package guard

import (
	"math"
	"strings"
	"testing"
)

func envOf(kv map[string]string) func(string) string {
	return func(k string) string { return kv[k] }
}

func TestDecide(t *testing.T) {
	dispatched := map[string]string{
		"GITHUB_ACTIONS":      "true",
		"CI":                  "true",
		"GITHUB_EVENT_NAME":   "workflow_dispatch",
		"GITHUB_WORKFLOW_REF": "tracewayapp/traceway/.github/workflows/cli-evals.yml@refs/heads/main",
		"TWEVALS_APPROVED":    "1",
	}
	with := func(over map[string]string) map[string]string {
		m := map[string]string{}
		for k, v := range dispatched {
			m[k] = v
		}
		for k, v := range over {
			m[k] = v
		}
		return m
	}
	cases := []struct {
		name    string
		env     map[string]string
		tty     bool
		yes     bool
		cost    float64
		wantErr string
		want    Decision
	}{
		{name: "dispatched and approved", env: dispatched, yes: true, cost: 10, want: Decision{CI: true}},
		{name: "no cap", env: dispatched, yes: true, cost: 0, wantErr: "--max-cost-usd is required"},
		{name: "push event", env: with(map[string]string{"GITHUB_EVENT_NAME": "push"}), yes: true, cost: 10, wantErr: "workflow_dispatch"},
		{name: "pull request", env: with(map[string]string{"GITHUB_EVENT_NAME": "pull_request"}), yes: true, cost: 10, wantErr: "workflow_dispatch"},
		{name: "schedule", env: with(map[string]string{"GITHUB_EVENT_NAME": "schedule"}), yes: true, cost: 10, wantErr: "workflow_dispatch"},
		{name: "other workflow", env: with(map[string]string{"GITHUB_WORKFLOW_REF": "tracewayapp/traceway/.github/workflows/cli.yml@refs/heads/main"}), yes: true, cost: 10, wantErr: "only .github/workflows/cli-evals.yml"},
		{name: "not behind the gate", env: with(map[string]string{"TWEVALS_APPROVED": ""}), yes: true, cost: 10, wantErr: "evals environment gate"},
		{name: "generic CI", env: map[string]string{"CI": "true"}, yes: true, cost: 10, wantErr: "CI="},
		{name: "buildkite", env: map[string]string{"BUILDKITE": "true"}, yes: true, cost: 10, wantErr: "BUILDKITE"},
		{name: "gitlab", env: map[string]string{"GITLAB_CI": "true"}, yes: true, cost: 10, wantErr: "GITLAB_CI"},
		{name: "jenkins", env: map[string]string{"JENKINS_URL": "http://ci"}, yes: true, cost: 10, wantErr: "JENKINS_URL"},
		{name: "NaN cap", env: dispatched, yes: true, cost: math.NaN(), wantErr: "finite number"},
		{name: "Inf cap", env: dispatched, yes: true, cost: math.Inf(1), wantErr: "finite number"},
		{name: "negative cap", env: map[string]string{}, tty: true, cost: -1, wantErr: "--max-cost-usd is required"},
		{name: "local non-tty without yes", env: map[string]string{}, cost: 10, wantErr: "not a terminal"},
		{name: "local non-tty with yes", env: map[string]string{}, yes: true, cost: 10, wantErr: "not a terminal"},
		{name: "local tty asks", env: map[string]string{}, tty: true, cost: 10, want: Decision{Confirm: true}},
		{name: "local tty asks even with yes", env: map[string]string{}, tty: true, yes: true, cost: 10, want: Decision{Confirm: true}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d, err := Decide(Request{Command: "run", MaxCostUSD: c.cost, Yes: c.yes, StdinTTY: c.tty, Getenv: envOf(c.env)})
			if c.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), c.wantErr) {
					t.Fatalf("want error containing %q, got %v", c.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if d != c.want {
				t.Fatalf("decision = %+v, want %+v", d, c.want)
			}
		})
	}
}

func TestLiveIsNeverOpenUnderGoTest(t *testing.T) {
	var out strings.Builder
	err := Authorize(Request{Command: "run", MaxCostUSD: 1, StdinTTY: true, Getenv: envOf(map[string]string{}), In: strings.NewReader("yes\n"), Out: &out})
	if err != nil {
		t.Fatal(err)
	}
	if Live() {
		t.Fatal("Live() must stay false inside go test")
	}
}

func TestConfirmationRequiresTypedYes(t *testing.T) {
	var out strings.Builder
	err := Authorize(Request{Command: "run", MaxCostUSD: 1, StdinTTY: true, Getenv: envOf(map[string]string{}), In: strings.NewReader("y\n"), Out: &out, Preflight: "PREFLIGHT"})
	if err == nil || !strings.Contains(err.Error(), "not confirmed") {
		t.Fatalf("a bare y must not confirm, got %v", err)
	}
	if !strings.Contains(out.String(), "PREFLIGHT") {
		t.Fatal("the preflight must be shown before asking")
	}
}
