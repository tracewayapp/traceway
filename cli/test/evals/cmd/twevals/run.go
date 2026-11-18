package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"golang.org/x/term"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/agent"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/catalog"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/env"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/grade"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/guard"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/report"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/runner"
)

// Plan is plan.json, the preflight the workflow reads.
type Plan struct {
	Selection  catalog.Selection `json:"selection"`
	Models     []string          `json:"models"`
	Runs       int               `json:"runs"`
	MaxCostUSD float64           `json:"maxCostUsd"`
	Shards     []Shard           `json:"shards"`
	Estimates  []report.Estimate `json:"estimates"`
	Judge      report.Estimate   `json:"judge"`
	Low        float64           `json:"low"`
	High       float64           `json:"high"`
	PriceNote  string            `json:"priceNote,omitempty"`
	Matrix     struct {
		Include []map[string]any `json:"include"`
	} `json:"matrix"`
}

// Shard is one suite's slice of a dispatch.
type Shard struct {
	Suite  string  `json:"suite"`
	Units  int     `json:"units"`
	Runs   int     `json:"runs"`
	Low    float64 `json:"low"`
	High   float64 `json:"high"`
	CapUSD float64 `json:"capUsd"`
}

func buildPlan(ctx context.Context, ws *workspace, sel catalog.Selection, modelList string, runs int, maxCost float64, pricesFile string) (*Plan, []catalog.Unit, []*catalog.Model, error) {
	if runs < 1 || runs > 3 {
		return nil, nil, nil, fmt.Errorf("--runs must be between 1 and 3, got %d", runs)
	}
	units, err := catalog.Select(ws.cases, ws.presets, sel)
	if err != nil {
		return nil, nil, nil, err
	}
	models, err := ws.models.ParseModelList(modelList)
	if err != nil {
		return nil, nil, nil, err
	}
	all := append(append([]*catalog.Model{}, models...), ws.models.Judge)
	prices, perr := report.FetchPrices(ctx, all, pricesFile)
	base, _ := report.LoadBaseline(filepath.Join(ws.root, "baseline.json"))
	p := &Plan{Selection: sel, Runs: runs, MaxCostUSD: maxCost}
	if perr != nil {
		p.PriceNote = "price list problem, using models.yaml fallback prices where needed: " + perr.Error()
	}
	for _, m := range models {
		p.Models = append(p.Models, m.Key)
	}
	bySuite := map[string]int{}
	for _, u := range units {
		bySuite[u.Case.Suite]++
	}
	for _, s := range catalog.Suites {
		n := bySuite[s]
		if n == 0 {
			continue
		}
		sh := Shard{Suite: s, Units: n, Runs: n * runs * len(models)}
		for _, m := range models {
			e := report.EstimateRuns(m, prices[m.ID], n*runs, base)
			sh.Low += e.Low
			sh.High += e.High
		}
		j := report.EstimateRuns(ws.models.Judge, prices[ws.models.Judge.ID], sh.Runs, nil)
		sh.Low += j.Low
		sh.High += j.High
		p.Shards = append(p.Shards, sh)
	}
	total := 0
	for _, m := range models {
		e := report.EstimateRuns(m, prices[m.ID], len(units)*runs, base)
		p.Estimates = append(p.Estimates, e)
		p.Low += e.Low
		p.High += e.High
		total += e.Runs
	}
	p.Judge = report.EstimateRuns(ws.models.Judge, prices[ws.models.Judge.ID], total, nil)
	p.Low += p.Judge.Low
	p.High += p.Judge.High
	mid := 0.0
	for _, s := range p.Shards {
		mid += (s.Low + s.High) / 2
	}
	for i := range p.Shards {
		share := 1 / float64(len(p.Shards))
		if mid > 0 {
			share = (p.Shards[i].Low + p.Shards[i].High) / 2 / mid
		}
		p.Shards[i].CapUSD = float64(int(maxCost*share*100)) / 100
		p.Matrix.Include = append(p.Matrix.Include, map[string]any{"shard": p.Shards[i].Suite, "cap": fmt.Sprintf("%.2f", p.Shards[i].CapUSD)})
	}
	return p, units, models, nil
}

func (p *Plan) text() string {
	var b strings.Builder
	fmt.Fprintf(&b, "## Eval preflight\n\n")
	fmt.Fprintf(&b, "Selection: %s · models %s · %d run(s) each\n\n", describeSel(p.Selection), strings.Join(p.Models, ","), p.Runs)
	b.WriteString("| Shard | Cases × arms | Runs | Estimate | Cap |\n|---|---|---|---|---|\n")
	for _, s := range p.Shards {
		fmt.Fprintf(&b, "| %s | %d | %d | $%.2f – $%.2f | $%.2f |\n", s.Suite, s.Units, s.Runs, s.Low, s.High, s.CapUSD)
	}
	b.WriteString("\n| Model | Runs | Estimate | Worst case (no cache) | Basis | Prices |\n|---|---|---|---|---|---|\n")
	for _, e := range append(append([]report.Estimate{}, p.Estimates...), p.Judge) {
		fmt.Fprintf(&b, "| %s `%s` | %d | $%.2f – $%.2f | $%.2f | %s | %s |\n", e.Model, e.ModelID, e.Runs, e.Low, e.High, e.Worst, e.Basis, e.Source)
	}
	fmt.Fprintf(&b, "\n**Total estimate: $%.2f – $%.2f. Ceiling: $%.2f.**\n", p.Low, p.High, p.MaxCostUSD)
	if p.PriceNote != "" {
		fmt.Fprintf(&b, "\n> %s\n", p.PriceNote)
	}
	return b.String()
}

func describeSel(s catalog.Selection) string {
	var parts []string
	for k, v := range map[string]string{"preset": s.Preset, "cases": s.Cases, "suites": s.Suites, "tags": s.Tags, "arms": s.Arms, "shard": s.Shard} {
		if v != "" {
			parts = append(parts, k+"="+v)
		}
	}
	sort.Strings(parts)
	return strings.Join(parts, " ")
}

func cmdPlan(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("plan", flag.ExitOnError)
	var sf selFlags
	sf.bind(fs)
	models := fs.String("models", "opus,astra", "comma-separated model keys from models.yaml")
	runs := fs.Int("runs", 1, "repeats per case, 1-3")
	maxCost := fs.Float64("max-cost-usd", 0, "hard cap for the whole dispatch (required)")
	_ = fs.Bool("shards", true, "split the cap per suite shard (always on)")
	out := fs.String("out", "", "write plan.json here")
	summary := fs.String("summary", "", "append the preflight table to this file (e.g. $GITHUB_STEP_SUMMARY)")
	prices := fs.String("prices", "", "read prices from a saved /api/v1/models response instead of fetching")
	_ = fs.Parse(args)
	if err := guard.CheckCap(*maxCost); err != nil {
		return &exitError{2, err.Error()}
	}
	ws, err := load()
	if err != nil {
		return err
	}
	p, _, _, err := buildPlan(ctx, ws, sf.selection(), *models, *runs, *maxCost, *prices)
	if err != nil {
		return err
	}
	text := p.text()
	fmt.Print(text)
	if *summary != "" {
		f, err := os.OpenFile(*summary, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err == nil {
			f.WriteString(text + "\n")
			f.Close()
		}
	}
	if *out != "" {
		raw, _ := json.MarshalIndent(p, "", "  ")
		if err := os.WriteFile(*out, raw, 0o644); err != nil {
			return err
		}
	}
	if *maxCost < p.Low {
		return &exitError{1, fmt.Sprintf("the cap $%.2f is below the low end of the estimate ($%.2f): raise --max-cost-usd or select fewer cases", *maxCost, p.Low)}
	}
	return nil
}

// selection is plan's and run's selection: smoke unless one is given, so a
// paid run never defaults to the whole catalog.
func (s *selFlags) selection() catalog.Selection {
	if s.sel.Preset == "" && s.sel.Cases == "" && s.sel.Suites == "" && s.sel.Tags == "" {
		s.sel.Preset = "smoke"
	}
	return s.sel
}

// runConfig is saved next to results so resume continues the same plan.
type runConfig struct {
	Selection catalog.Selection `json:"selection"`
	Models    string            `json:"models"`
	Runs      int               `json:"runs"`
	Judge     bool              `json:"judge"`
	Jobs      int               `json:"jobs"`
}

func cmdRun(ctx context.Context, args []string, resumeDir string) error {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	var sf selFlags
	sf.bind(fs)
	shard := fs.String("shard", "", "run one suite shard")
	models := fs.String("models", "opus,astra", "comma-separated model keys")
	runs := fs.Int("runs", 1, "repeats per case, 1-3")
	maxCost := fs.Float64("max-cost-usd", 0, "hard cap for this invocation (required, no default)")
	jobs := fs.Int("j", 4, "parallel runs")
	keep := fs.Bool("keep", false, "keep every run directory (backend db, workspace)")
	judge := fs.Bool("judge", true, "score reports with the LLM judge (non-gating)")
	yes := fs.Bool("yes", false, "the dispatched workflow's confirmation; a local run always asks you to type yes")
	out := fs.String("out", "", "results directory (default results/<run-id>)")
	_ = fs.Parse(args)

	ws, err := load()
	if err != nil {
		return err
	}
	cfg := runConfig{Selection: sf.selection(), Models: *models, Runs: *runs, Judge: *judge, Jobs: *jobs}
	cfg.Selection.Shard = *shard
	outDir := *out
	skip := map[string]bool{}
	if resumeDir != "" {
		raw, err := os.ReadFile(filepath.Join(resumeDir, "config.json"))
		if err != nil {
			return fmt.Errorf("resume: %w", err)
		}
		if err := json.Unmarshal(raw, &cfg); err != nil {
			return fmt.Errorf("resume: %w", err)
		}
		outDir = resumeDir
		if prev, err := report.Load(resumeDir); err == nil {
			for _, r := range prev.Runs {
				if r.Status != "cost_ceiling" && r.Status != "infra_error" {
					skip[r.Key()] = true
				}
			}
		}
	}
	plan, units, chosen, err := buildPlan(ctx, ws, cfg.Selection, cfg.Models, cfg.Runs, *maxCost, "")
	if err != nil {
		return err
	}
	if err := guard.Authorize(guard.Request{
		Command: map[bool]string{true: "resume", false: "run"}[resumeDir != ""], MaxCostUSD: *maxCost, Yes: *yes,
		StdinTTY: term.IsTerminal(int(os.Stdin.Fd())), Getenv: os.Getenv, Preflight: plan.text(), In: os.Stdin, Out: os.Stderr,
	}); err != nil {
		return &exitError{2, err.Error()}
	}
	client, err := agent.NewOpenRouter(os.Getenv("OPENROUTER_API_KEY"))
	if err != nil {
		return &exitError{2, err.Error()}
	}
	var chatter agent.Chatter = client
	runID := time.Now().UTC().Format("20060102-150405")
	if outDir == "" {
		outDir = filepath.Join(ws.root, "results", runID)
	} else {
		runID = filepath.Base(outDir)
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	cfgRaw, _ := json.MarshalIndent(cfg, "", "  ")
	_ = os.WriteFile(filepath.Join(outDir, "config.json"), cfgRaw, 0o644)
	planRaw, _ := json.MarshalIndent(plan, "", "  ")
	_ = os.WriteFile(filepath.Join(outDir, "plan.json"), planRaw, 0o644)

	ledger := agent.NewLedger(*maxCost)
	bins, err := env.BuildBinaries(ws.repo, filepath.Join(ws.root, ".cache"))
	if err != nil {
		return err
	}
	priced := chosen
	if cfg.Judge && ws.models.Judge != nil {
		priced = append(slices.Clone(chosen), ws.models.Judge)
	}
	prices := pricesByKey(ctx, ws, priced)
	opt := runner.Options{EvalsRoot: ws.root, RepoRoot: ws.repo, OutDir: outDir, Bins: bins, Models: chosen, Runs: cfg.Runs, Jobs: cfg.Jobs,
		Keep: *keep, Chat: func(*catalog.Model) agent.Chatter { return chatter }, Ledger: ledger, Log: os.Stderr, Skip: skip,
		Prices: prices, AllowHostTests: os.Getenv("TWEVALS_HOST_TESTS") == "1"}
	if cfg.Judge {
		opt.Judge = &grade.Judge{Chat: chatter, Model: ws.models.Judge, Ledger: ledger, Price: prices[ws.models.Judge.Key]}
	}
	opt.SandboxImage = sandboxImage(ctx, ws, units)
	header := report.NewHeader(ws.repo, runID, ws.models, bins.Hash, os.Args)
	header.MaxCostUSD = *maxCost
	results := runner.Execute(ctx, opt, runner.Jobs(units, chosen, cfg.Runs))
	if resumeDir != "" {
		if prev, err := report.Load(resumeDir); err == nil {
			// The caps and the spend of every invocation add up: the runs a
			// resume ran again keep what they cost the first time.
			var replaced float64
			results, replaced = mergeResume(prev.Runs, results)
			header.MaxCostUSD += prev.Header.MaxCostUSD
			header.ReplacedSpendUSD = prev.Header.ReplacedSpendUSD + replaced
			if !prev.Header.Started.IsZero() {
				header.Started = prev.Header.Started
			}
		}
	}
	header.Finished = time.Now().UTC()
	res := &report.Results{Header: header, Runs: results}
	if err := report.Save(outDir, res); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "\nresults: %s\nspend: %s\n", outDir, ledger)
	return partialExit(results)
}

func partialExit(results []*runner.Result) error {
	ceiling, infra := 0, 0
	for _, r := range results {
		switch r.Status {
		case agent.StopCeiling:
			ceiling++
		case agent.StopInfra:
			infra++
		}
	}
	if ceiling > 0 {
		return &exitError{2, fmt.Sprintf("partial: %d runs were stopped or not started at the cost ceiling (continue with twevals resume)", ceiling)}
	}
	if len(results) > 0 && float64(infra)/float64(len(results)) > 0.10 {
		return &exitError{2, fmt.Sprintf("partial: %d of %d runs hit infra errors", infra, len(results))}
	}
	return nil
}

// pricesByKey fetches live prices and, while it has the model list, drops a
// configured reasoning effort for a model that does not list "reasoning" in
// its supported_parameters (require_parameters would otherwise fail the run).
func pricesByKey(ctx context.Context, ws *workspace, models []*catalog.Model) map[string]catalog.Price {
	live, _ := report.FetchPrices(ctx, models, "")
	out := map[string]catalog.Price{}
	for _, m := range models {
		p := live[m.ID]
		out[m.Key] = p.Price
		if m.ReasoningEffort != "" && p.Source == "live" && !slices.Contains(p.Supported, "reasoning") {
			fmt.Fprintf(os.Stderr, "note: %s does not list reasoning in supported_parameters; not sending reasoning.effort\n", m.ID)
			m.ReasoningEffort = ""
		}
	}
	return out
}
