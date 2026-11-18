package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/catalog"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/env"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/report"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/runner"
	_ "github.com/tracewayapp/traceway/cli/test/evals/internal/scenarios/all"
)

type workspace struct {
	root    string // cli/test/evals
	repo    string
	cases   []*catalog.Case
	presets *catalog.Presets
	models  *catalog.Models
}

func load() (*workspace, error) {
	root := os.Getenv("TWEVALS_ROOT")
	if root == "" {
		wd, _ := os.Getwd()
		for d := wd; ; d = filepath.Dir(d) {
			if _, err := os.Stat(filepath.Join(d, "models.yaml")); err == nil {
				if _, err := os.Stat(filepath.Join(d, "presets.yaml")); err == nil {
					root = d
					break
				}
			}
			if filepath.Dir(d) == d {
				return nil, errors.New("run twevals from cli/test/evals (or set TWEVALS_ROOT)")
			}
		}
	}
	repo, err := env.RepoRoot(root)
	if err != nil {
		return nil, err
	}
	cases, err := catalog.LoadCases(root)
	if err != nil {
		return nil, err
	}
	presets, err := catalog.LoadPresets(root)
	if err != nil {
		return nil, err
	}
	models, err := catalog.LoadModels(root)
	if err != nil {
		return nil, err
	}
	return &workspace{root: root, repo: repo, cases: cases, presets: presets, models: models}, nil
}

// parseInterleaved parses flags that may come after positional arguments.
func parseInterleaved(fs *flag.FlagSet, args []string) []string {
	var pos []string
	for {
		_ = fs.Parse(args)
		if fs.NArg() == 0 {
			return pos
		}
		pos = append(pos, fs.Arg(0))
		args = fs.Args()[1:]
	}
}

type selFlags struct{ sel catalog.Selection }

func (s *selFlags) bind(fs *flag.FlagSet) {
	fs.StringVar(&s.sel.Preset, "preset", "", "smoke | core | full")
	fs.StringVar(&s.sel.Cases, "cases", "", "comma-separated case ids or globs (overrides the preset)")
	fs.StringVar(&s.sel.Suites, "suites", "", "comma-separated suites")
	fs.StringVar(&s.sel.Tags, "tags", "", "comma-separated tags, all required")
	fs.StringVar(&s.sel.Arms, "arms", "", "comma-separated arms")
}

func cmdList(args []string) error {
	fs := flag.NewFlagSet("list", flag.ExitOnError)
	var sf selFlags
	sf.bind(fs)
	_ = fs.Parse(args)
	ws, err := load()
	if err != nil {
		return err
	}
	sel := sf.sel
	if sel.Preset == "" && sel.Cases == "" {
		sel.Preset = "full"
	}
	units, err := catalog.Select(ws.cases, ws.presets, sel)
	if err != nil {
		return err
	}
	tw := tabwriter.NewWriter(os.Stdout, 2, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "CASE\tARM\tTAGS\tTITLE")
	for _, u := range units {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", u.Case.ID, u.Arm, strings.Join(u.Case.Tags, ","), u.Case.Title)
	}
	tw.Flush()
	fmt.Printf("\n%d runs per model\n", len(units))
	return nil
}

func cmdOracle(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("oracle", flag.ExitOnError)
	all := fs.Bool("all", false, "every case")
	arms := fs.String("arms", "", "comma-separated arms (default: every arm the case lists)")
	jobs := fs.Int("j", 3, "parallel runs")
	keep := fs.Bool("keep", false, "keep run directories")
	suites := fs.String("suites", "", "comma-separated suites")
	out := fs.String("out", "", "results directory")
	ids := parseInterleaved(fs, args)
	ws, err := load()
	if err != nil {
		return err
	}
	sel := catalog.Selection{Preset: "full", Arms: *arms, Suites: *suites}
	if !*all {
		if len(ids) == 0 && *suites == "" {
			return errors.New("name case ids, or pass --all")
		}
		sel.Cases = strings.Join(ids, ",")
	}
	units, err := catalog.Select(ws.cases, ws.presets, sel)
	if err != nil {
		return err
	}
	bins, err := env.BuildBinaries(ws.repo, filepath.Join(ws.root, ".cache"))
	if err != nil {
		return err
	}
	outDir := *out
	if outDir == "" {
		outDir = filepath.Join(ws.root, ".cache", "oracle", time.Now().UTC().Format("20060102-150405"))
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	oracleModel := &catalog.Model{Key: "oracle", ID: "fake/oracle", Caching: "none"}
	opt := runner.Options{EvalsRoot: ws.root, RepoRoot: ws.repo, OutDir: outDir, Bins: bins, Models: []*catalog.Model{oracleModel},
		Runs: 1, Jobs: *jobs, Keep: *keep, Oracle: true, Log: os.Stderr}
	opt.SandboxImage = sandboxImage(ctx, ws, units)
	header := report.NewHeader(ws.repo, filepath.Base(outDir), ws.models, bins.Hash, os.Args)
	header.Oracle = true
	header.Models = map[string]string{oracleModel.Key: oracleModel.ID}
	results := runner.Execute(ctx, opt, runner.Jobs(units, opt.Models, 1))
	header.Finished = time.Now().UTC()
	if err := report.Save(outDir, &report.Results{Header: header, Runs: results}); err != nil {
		return err
	}
	failed := 0
	for _, r := range results {
		if r.Status != "pass" {
			failed++
			fmt.Fprintf(os.Stderr, "FAIL %s: %s\n", r.Key(), oneLine(r))
		}
	}
	fmt.Fprintf(os.Stderr, "\noracle: %d/%d passed (%s)\n", len(results)-failed, len(results), outDir)
	if failed > 0 {
		return &exitError{1, ""}
	}
	return nil
}

func oneLine(r *runner.Result) string {
	if r.Error != "" {
		return r.Status + ": " + r.Error
	}
	for _, g := range r.Graders {
		if g.Gating && !g.Skipped && !g.Pass {
			return r.Status + ": " + g.Grader + ": " + strings.Join(g.Failures, "; ")
		}
	}
	return r.Status
}
