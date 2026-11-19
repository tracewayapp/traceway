package runner

import (
	"context"
	"testing"
	"time"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/catalog"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/env"
	_ "github.com/tracewayapp/traceway/cli/test/evals/internal/scenarios/all"
)

// TestOracleNPlusOne is the Phase 3 acceptance check: the N+1 case runs end
// to end with the fake model (backend, seed, CLI, shell, graders), with no
// network call apart from the local backend.
func TestOracleNPlusOne(t *testing.T) {
	bins := env.BinariesForTest(t)
	root := env.EvalsRoot()
	repo, err := env.RepoRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	c, err := catalog.LoadCase(root + "/cases/perf/perf-db-n-plus-1.yaml")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	model := &catalog.Model{Key: "oracle", ID: "fake/oracle", Caching: "none"}
	opt := Options{EvalsRoot: root, RepoRoot: repo, OutDir: t.TempDir(), Bins: bins, Models: []*catalog.Model{model}, Runs: 1, Jobs: 1, Oracle: true}
	res := Execute(ctx, opt, Jobs([]catalog.Unit{{Case: c, Arm: catalog.ArmCLISkill}}, opt.Models, 1))
	if len(res) != 1 || res[0].Status != "pass" {
		for _, r := range res {
			t.Logf("%s %s %s", r.Key(), r.Status, r.Error)
			for _, g := range r.Graders {
				t.Logf("  %s pass=%v %v", g.Grader, g.Pass, g.Failures)
			}
		}
		t.Fatal("the N+1 oracle must pass")
	}
}
