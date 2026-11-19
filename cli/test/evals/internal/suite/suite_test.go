// Package suite_test checks the case catalog as a whole: every case loads,
// references things that exist, uses valid checklist ids, and seeds data
// that does not give the answer away.
package suite_test

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/catalog"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/checklist"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/env"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/grade"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/scenarios"
	_ "github.com/tracewayapp/traceway/cli/test/evals/internal/scenarios/all"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/seed"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/tools"
)

func loadAll(t *testing.T) []*catalog.Case {
	t.Helper()
	cases, err := catalog.LoadCases(env.EvalsRoot())
	if err != nil {
		t.Fatal(err)
	}
	if len(cases) == 0 {
		t.Fatal("no cases")
	}
	return cases
}

func exists(p string) bool { _, err := os.Stat(p); return err == nil }

func TestCaseReferences(t *testing.T) {
	root := env.EvalsRoot()
	seen := map[string]bool{}
	for _, c := range loadAll(t) {
		t.Run(c.ID, func(t *testing.T) {
			if _, err := scenarios.Get(c.Seed); err != nil {
				t.Error(err)
			}
			seen[c.Seed] = true
			if r := c.Workspace.Repo; r != "" && !exists(filepath.Join(root, "testdata", "repos", r)) && !exists(filepath.Join(root, "testdata", "setup-templates", r)) {
				t.Errorf("fixture %s does not exist", r)
			}
			if cs := c.Expect.Code; cs != nil && cs.HiddenTests != "" && !exists(filepath.Join(root, "testdata", "hidden", cs.HiddenTests)) {
				t.Errorf("hidden tests %s do not exist", cs.HiddenTests)
			}
			checkIDs := func(where string, ids ...string) {
				for _, id := range ids {
					if id != "" && !strings.Contains(id, "{{") && !checklist.IsID(id) {
						t.Errorf("%s: %q is not a checklist id (valid: %s)", where, id, strings.Join(checklist.IDs(), ", "))
					}
				}
			}
			if a := c.Expect.Answer; a != nil {
				checkIDs("expect.answer.checklist_item", a.ChecklistItem)
				checkIDs("expect.answer.any_of", a.AnyOf...)
				checkIDs("expect.answer.not", a.Not...)
				if a.Category != "" && !slices.Contains(tools.DebugCategories, a.Category) {
					t.Errorf("category %q is not a debug category", a.Category)
				}
				if c.Suite == "perf" && a.ChecklistItem == "" && len(a.AnyOf) == 0 {
					t.Error("perf answers need checklist_item or any_of")
				}
			}
			byArm := map[string][]catalog.Step{"oracle": c.Oracle}
			for arm, steps := range c.OracleByArm {
				byArm["oracle_by_arm."+arm] = steps
			}
			for arm, steps := range byArm {
				for i, s := range steps {
					if s.Submit != nil {
						item, _ := s.Submit["checklist_item"].(string)
						checkIDs(fmt.Sprintf("%s[%d].submit", arm, i), item)
					}
					if s.WriteFile != nil && s.WriteFile.From != "" && !exists(filepath.Join(root, "testdata", s.WriteFile.From)) {
						t.Errorf("%s[%d] writes from %s, which does not exist", arm, i, s.WriteFile.From)
					}
				}
			}
			last := c.Oracle[len(c.Oracle)-1]
			if last.Submit == nil && last.Finish == nil {
				t.Error("the oracle must end with submit or finish")
			}
			if c.Suite == "perf" || c.Suite == "debug" {
				if len(c.Judge.Rubric) == 0 {
					t.Error("perf and debug cases need a judge rubric")
				}
			}
		})
	}
}

// TestLeakage seeds every case into a dry sink and lints the strings it
// would send, plus the prompt, for terms that name the answer.
func TestLeakage(t *testing.T) {
	for _, c := range loadAll(t) {
		t.Run(c.ID, func(t *testing.T) {
			s := seed.NewScenario(c.ID, time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC), seed.NewDrySink())
			if err := scenarios.Run(c.Seed, s); err != nil {
				t.Fatal(err)
			}
			for _, f := range grade.Lint(c, s.Strings()) {
				t.Error(f)
			}
			if st := s.Stats(); st.Spans > 25000 {
				t.Errorf("seed sends %d spans, over the ~20k budget", st.Spans)
			}
		})
	}
}

func TestPresets(t *testing.T) {
	cases := loadAll(t)
	presets, err := catalog.LoadPresets(env.EvalsRoot())
	if err != nil {
		t.Fatal(err)
	}
	for name := range presets.Presets {
		units, err := catalog.Select(cases, presets, catalog.Selection{Preset: name})
		if err != nil {
			t.Errorf("preset %s: %v", name, err)
			continue
		}
		for _, u := range units {
			if name != "full" && !slices.Contains(presets.PrimaryArms[u.Case.Suite], u.Arm) && u.Arm != u.Case.Arms[0] {
				t.Errorf("preset %s picked non-primary arm %s for %s", name, u.Arm, u.Case.ID)
			}
		}
	}
	if _, err := catalog.LoadModels(env.EvalsRoot()); err != nil {
		t.Fatal(err)
	}
}

// TestChecklistIsComplete: the enum has every performance.md row.
func TestChecklistIsComplete(t *testing.T) {
	if n := len(checklist.Rows()); n != 26 {
		t.Fatalf("performance.md has %d checklist rows; update the perf catalog when rows change", n)
	}
}
