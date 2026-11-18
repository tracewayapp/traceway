package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"time"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/catalog"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/env"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/report"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/runner"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/scenarios"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/seed"
)

func mergeResume(prev, now []*runner.Result) ([]*runner.Result, float64) {
	byKey := map[string]*runner.Result{}
	for _, r := range prev {
		byKey[r.Key()] = r
	}
	replaced := 0.0
	for _, r := range now {
		if old, ok := byKey[r.Key()]; ok {
			replaced += old.Metrics.CostUSD + old.Metrics.JudgeCostUSD
		}
		byKey[r.Key()] = r
	}
	out := make([]*runner.Result, 0, len(byKey))
	for _, r := range byKey {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key() < out[j].Key() })
	return out, replaced
}

// sandboxImage builds the container image when the selection needs it (the
// setup suite, or hidden tests of a model's patch) and Docker is present.
func sandboxImage(ctx context.Context, ws *workspace, units []catalog.Unit) string {
	need := false
	for _, u := range units {
		if u.Case.Suite == "setup" || (u.Case.Expect.Code != nil && !u.Case.Expect.Code.Static) {
			need = true
		}
	}
	if !need {
		return ""
	}
	if _, err := exec.LookPath("docker"); err != nil {
		fmt.Fprintln(os.Stderr, "note: docker not found; setup cases fail and hidden tests of model patches are skipped")
		return ""
	}
	tag, err := runner.EnsureSandboxImage(ctx, ws.root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "note:", err)
		return ""
	}
	return tag
}

func cmdEnvUp(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("usage: twevals env up <case-id>")
	}
	ws, err := load()
	if err != nil {
		return err
	}
	var c *catalog.Case
	for _, x := range ws.cases {
		if x.ID == args[0] {
			c = x
		}
	}
	if c == nil {
		return fmt.Errorf("no case %q", args[0])
	}
	bins, err := env.BuildBinaries(ws.repo, filepath.Join(ws.root, ".cache"))
	if err != nil {
		return err
	}
	dir, err := os.MkdirTemp("", "twevals-env-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	inst, err := env.NewInstance(ctx, bins, dir)
	if err != nil {
		return err
	}
	defer inst.Close()
	sink := seed.NewLiveSink(ctx, inst.Backend, inst.Admin)
	sink.StubSrv = inst.Stub
	sc := seed.NewScenario(c.ID, time.Now(), sink)
	if err := scenarios.Run(c.Seed, sc); err != nil {
		return err
	}
	if err := sc.Verify(time.Minute); err != nil {
		return err
	}
	exposed, _ := json.MarshalIndent(sc.Exposed(), "", "  ")
	pat, _ := env.CreatePAT(ctx, inst.Backend, inst.Admin.JWT, "env up", "read", 0)
	fmt.Printf("Case %s is seeded (%+v).\n\nDashboard: %s\nLogin:     %s / %s\nRead PAT:  %s\n  %s login --url %s --token %s\n\nseed.json:\n%s\n\nCtrl-C to stop. Data dir: %s\n",
		c.ID, sc.Stats(), inst.Backend.URL, inst.Admin.Email, inst.Admin.Password, pat, bins.CLI, inst.Backend.URL, pat, exposed, dir)
	<-ctx.Done()
	return nil
}

func cmdReport(args []string) error {
	fs := flag.NewFlagSet("report", flag.ExitOnError)
	merge := fs.Bool("merge", false, "merge several run directories (shards) into one")
	replace := fs.Bool("replace", false, "a run that a later directory repeats replaces the earlier one (a rerun) instead of being an error")
	out := fs.String("out", "", "write results.json and summary.md here")
	pos := parseInterleaved(fs, args)
	if len(pos) == 0 {
		return errors.New("report needs at least one run directory")
	}
	var dirs []string
	for _, a := range pos {
		// A shard artifact may nest its run directory; find every results root.
		matches := findRunDirs(a)
		if len(matches) == 0 {
			matches = []string{a}
		}
		dirs = append(dirs, matches...)
	}
	var parts []*report.Results
	for _, d := range dirs {
		r, err := report.Load(d)
		if err != nil {
			return err
		}
		parts = append(parts, r)
	}
	var res *report.Results
	if *merge || len(parts) > 1 {
		if *out == "" {
			return &exitError{2, "merging several run directories needs --out (writing into the first one would overwrite that shard's results)"}
		}
		merged, notes, err := report.Merge(parts, *replace)
		if err != nil {
			return &exitError{2, err.Error()}
		}
		for _, n := range notes {
			fmt.Fprintln(os.Stderr, "note:", n)
		}
		res = merged
	} else {
		res = parts[0]
	}
	target := *out
	if target == "" {
		target = dirs[0]
	}
	if err := os.MkdirAll(target, 0o755); err != nil {
		return err
	}
	if err := report.Save(target, res); err != nil {
		return err
	}
	md, _ := os.ReadFile(filepath.Join(target, "summary.md"))
	fmt.Print(string(md))
	return partialExit(res.Runs)
}

// findRunDirs returns directories under root that hold results.json or a
// runs/ tree.
func findRunDirs(root string) []string {
	var out []string
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return nil
		}
		if d.Name() == "runs" {
			out = append(out, filepath.Dir(p))
			return filepath.SkipDir
		}
		return nil
	})
	return out
}

func cmdCompare(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: twevals compare <run-dir> [baseline.json]")
	}
	ws, err := load()
	if err != nil {
		return err
	}
	res, err := report.Load(args[0])
	if err != nil {
		return err
	}
	basePath := filepath.Join(ws.root, "baseline.json")
	if len(args) > 1 {
		basePath = args[1]
	}
	base, err := report.LoadBaseline(basePath)
	if err != nil {
		return err
	}
	regs := report.Compare(res, base)
	if len(regs) == 0 {
		fmt.Printf("No regressions against %s (%d suite and %d case baselines).\n", basePath, len(base.Suites), len(base.Cases))
		return nil
	}
	fmt.Printf("## Regressions against the baseline\n\n")
	for _, r := range regs {
		fmt.Printf("- %s\n", r.Detail)
	}
	return &exitError{1, fmt.Sprintf("%d regression(s)", len(regs))}
}

func cmdBaselineUpdate(args []string) error {
	fs := flag.NewFlagSet("baseline update", flag.ExitOnError)
	out := fs.String("out", "", "baseline path (default cli/test/evals/baseline.json)")
	pos := parseInterleaved(fs, args)
	if len(pos) == 0 {
		return errors.New("usage: twevals baseline update <run-dir>")
	}
	ws, err := load()
	if err != nil {
		return err
	}
	res, err := report.Load(pos[0])
	if err != nil {
		return err
	}
	if res.Header.Oracle {
		return errors.New("refusing to baseline an oracle run")
	}
	path := *out
	if path == "" {
		path = filepath.Join(ws.root, "baseline.json")
	}
	b := report.NewBaseline(res)
	if err := b.Save(path); err != nil {
		return err
	}
	fmt.Printf("Wrote %s from %s. Review it and commit it yourself; the workflow never commits.\n", path, res.Header.RunID)
	return nil
}
