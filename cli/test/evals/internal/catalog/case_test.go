package catalog

import (
	"strings"
	"testing"
)

// TestInvokeMatchesPrompt: the runner preloads the skill for invoke: slash,
// so a case's invoke and its prompt must agree.
func TestInvokeMatchesPrompt(t *testing.T) {
	for _, c := range []struct {
		invoke, prompt string
		ok             bool
	}{
		{"slash", "/traceway perf GET /api/orders", true},
		{"natural", "Checkout got slow this week.", true},
		{"", "Checkout got slow this week.", true},
		{"slash", "Checkout got slow this week.", false},
		{"natural", "/traceway perf GET /api/orders", false},
	} {
		cs := &Case{ID: "perf-x", Suite: "perf", Seed: "perf/x", Arms: []string{ArmCLISkill}, Invoke: c.invoke, Prompt: c.prompt,
			Oracle: []Step{{Shell: "true"}}, Expect: Expect{Answer: &Answer{ChecklistItem: "none_found"}}, Distractor: "x"}
		err := cs.Validate()
		if got := err == nil || !strings.Contains(err.Error(), "invoke: slash"); got != c.ok {
			t.Errorf("invoke %q, prompt %q: %v", c.invoke, c.prompt, err)
		}
	}
}

// TestValidateRejectsCasesTheHarnessCannotRun: arms the suite cannot run,
// unknown hook actions, setup values the runner does not know, and a debug
// answer that checks nothing.
func TestValidateRejectsCasesTheHarnessCannotRun(t *testing.T) {
	base := func() *Case {
		return &Case{ID: "debug-x", Suite: "debug", Seed: "debug/x", Arms: []string{ArmCLISkill}, Prompt: "Fix it.",
			Oracle: []Step{{Shell: "true"}}, Expect: Expect{Answer: &Answer{Category: "logic"}}, Distractor: "x"}
	}
	if err := base().Validate(); err != nil {
		t.Fatal(err)
	}
	for name, c := range map[string]struct {
		edit func(*Case)
		want string
	}{
		"setup arm":    {func(c *Case) { c.Arms = []string{ArmCLISetupSkill} }, "does not run the debug suite"},
		"hook action":  {func(c *Case) { c.Hooks = []Hook{{Action: "edit_postmortem"}} }, "unknown action"},
		"empty answer": {func(c *Case) { c.Expect.Answer = &Answer{} }, "need expect.answer.category"},
		"setup token": {func(c *Case) {
			c.ID, c.Suite, c.Arms = "setup-x", "setup", []string{ArmCLISetupSkill}
			c.Expect = Expect{State: []StateAssert{{Kind: "file"}}}
			c.Setup = &SetupSpec{Token: "write"}
		}, "setup.token must be"},
	} {
		t.Run(name, func(t *testing.T) {
			cs := base()
			c.edit(cs)
			if err := cs.Validate(); err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("want an error containing %q, got %v", c.want, err)
			}
		})
	}
}
