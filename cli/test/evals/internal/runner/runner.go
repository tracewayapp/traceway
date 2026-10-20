// Package runner executes eval runs: for each (case, arm, model, run) it boots
// a hermetic backend, seeds it, prepares the workspace and credentials for
// the arm, drives the agent, grades the result and tears everything down.
package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/agent"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/catalog"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/env"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/grade"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/record"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/scenarios"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/seed"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/tools"
)

// Options configures a batch of runs.
type Options struct {
	EvalsRoot string
	RepoRoot  string
	OutDir    string
	Bins      *env.Binaries
	Models    []*catalog.Model
	Runs      int
	Jobs      int
	Keep      bool
	// Oracle plays each case's oracle with the fake model (free).
	Oracle bool
	// Chat returns the model client for real runs.
	Chat   func(m *catalog.Model) agent.Chatter
	Ledger *agent.Ledger
	Judge  *grade.Judge
	// Prices give a per-model cost estimate when the provider reports none.
	Prices         map[string]catalog.Price
	SandboxImage   string
	AllowHostTests bool
	Log            io.Writer
	// Skip lists run keys (case/arm/model/rN) already finished (resume).
	Skip map[string]bool
}

// Result is one run's outcome (results.json "runs" entries).
type Result struct {
	Case       string           `json:"case"`
	Suite      string           `json:"suite"`
	Arm        string           `json:"arm"`
	Model      string           `json:"model"`
	ModelID    string           `json:"modelId"`
	Providers  []string         `json:"provider,omitempty"`
	RunIdx     int              `json:"runIdx"`
	Status     string           `json:"status"` // pass | fail | budget_exhausted | infra_error | cost_ceiling
	Graders    []*grade.Verdict `json:"graders"`
	Metrics    Metrics          `json:"metrics"`
	Transcript string           `json:"transcript,omitempty"`
	Error      string           `json:"error,omitempty"`
	Seed       *seed.Stats      `json:"seed,omitempty"`
}

// Key identifies the run for resume.
func (r *Result) Key() string { return RunKey(r.Case, r.Arm, r.Model, r.RunIdx) }

// RunKey builds a run key.
func RunKey(c, arm, model string, idx int) string {
	return fmt.Sprintf("%s/%s/%s/r%d", c, arm, model, idx)
}

// Metrics are a run's measurements.
type Metrics struct {
	Turns            int      `json:"turns"`
	ToolCalls        int      `json:"toolCalls"`
	FailedCalls      int      `json:"failedCalls"`
	InputTokens      int      `json:"inputTokens"`
	CachedTokens     int      `json:"cachedTokens"`
	CacheWriteTokens int      `json:"cacheWriteTokens"`
	OutputTokens     int      `json:"outputTokens"`
	CostUSD          float64  `json:"costUsd"`
	JudgeCostUSD     float64  `json:"judgeCostUsd"`
	WallMs           int64    `json:"wallMs"`
	SkillLoaded      bool     `json:"skillLoaded"`
	Skills           []string `json:"skills,omitempty"`
}

// Job is one scheduled run.
type Job struct {
	Unit  catalog.Unit
	Model *catalog.Model
	Idx   int
}

// Jobs expands units × models × runs.
func Jobs(units []catalog.Unit, models []*catalog.Model, runs int) []Job {
	var out []Job
	for _, u := range units {
		for _, m := range models {
			for i := 1; i <= runs; i++ {
				out = append(out, Job{Unit: u, Model: m, Idx: i})
			}
		}
	}
	return out
}

// Execute runs every job with up to Jobs in parallel. With a ceiling, each
// run reserves its cap from the ledger as it starts: up to three times its
// even share of --max-cost-usd, and no more than an even split of what is
// neither spent nor reserved among the runs that can still be in flight. A run
// whose split falls below its even share waits for a run in flight to return
// what it did not use; with nothing in flight it is not started and is
// recorded as cost_ceiling. After an interrupt nothing new starts.
func Execute(ctx context.Context, opt Options, jobs []Job) []*Result {
	par := opt.Jobs
	if par <= 0 {
		par = 1
	}
	var todo []int
	for i, j := range jobs {
		if !opt.Skip[RunKey(j.Unit.Case.ID, j.Unit.Arm, j.Model.Key, j.Idx)] {
			todo = append(todo, i)
		}
	}
	results := make([]*Result, len(jobs))
	sem := make(chan struct{}, par)
	finished := make(chan struct{}, len(todo))
	var inFlight atomic.Int32
	var wg sync.WaitGroup
	capped := opt.Ledger != nil && opt.Ledger.Cap > 0 && len(todo) > 0
	even, perRun := 0.0, 0.0
	if capped {
		even = opt.Ledger.Cap / float64(len(todo))
		perRun = even * 3
	}
	for n, i := range todo {
		j := jobs[i]
		notStarted := func(status, why string) {
			results[i] = &Result{Case: j.Unit.Case.ID, Suite: j.Unit.Case.Suite, Arm: j.Unit.Arm, Model: j.Model.Key, ModelID: j.Model.ID, RunIdx: j.Idx,
				Status: status, Error: "not started: " + why}
		}
		sem <- struct{}{}
		if ctx.Err() != nil {
			<-sem
			notStarted(agent.StopInfra, "interrupted")
			continue
		}
		var hold *agent.Hold
		if capped {
			slots := float64(min(par, len(todo)-n))
			share := min(perRun, opt.Ledger.Available()/slots)
			for share < even && inFlight.Load() > 0 && ctx.Err() == nil {
				<-finished
				share = min(perRun, opt.Ledger.Available()/slots)
			}
			if share < even || ctx.Err() != nil {
				<-sem
				notStarted(agent.StopCeiling, "the --max-cost-usd ceiling was reached")
				continue
			}
			hold = opt.Ledger.Reserve(share)
		}
		inFlight.Add(1)
		wg.Add(1)
		go func(i int, j Job, hold *agent.Hold) {
			defer wg.Done()
			defer func() {
				hold.Release()
				inFlight.Add(-1)
				finished <- struct{}{}
				<-sem
			}()
			results[i] = startRun(ctx, opt, j, hold)
		}(i, j, hold)
	}
	wg.Wait()
	var out []*Result
	for _, r := range results {
		if r != nil {
			out = append(out, r)
		}
	}
	return out
}

// startRun is runOne; the scheduling tests replace it.
var startRun = runOne

func logf(opt Options, format string, a ...any) {
	if opt.Log != nil {
		fmt.Fprintf(opt.Log, format+"\n", a...)
	}
}

// surface is what the arm hands the agent.
type surface struct {
	cliBin     string // executable path for CLI arms
	cliName    string // traceway | traceway-write
	cliHome    *env.CLIHome
	mcp        *tools.MCPBridge
	skills     []string
	submit     string
	setupShell bool
	// containerEnv is the setup shell's environment (tokens, URL, PORT).
	containerEnv []string
}

func submitFor(suite string) string {
	switch suite {
	case "perf":
		return tools.SubmitPerf
	case "debug":
		return tools.SubmitDebug
	default:
		return tools.Finish
	}
}

func runOne(ctx context.Context, opt Options, j Job, hold *agent.Hold) (res *Result) {
	c := j.Unit.Case
	arm := j.Unit.Arm
	res = &Result{Case: c.ID, Suite: c.Suite, Arm: arm, Model: j.Model.Key, ModelID: j.Model.ID, RunIdx: j.Idx}
	if opt.Oracle {
		res.Model, res.ModelID = "oracle", "fake"
	}
	started := time.Now()
	dir := filepath.Join(opt.OutDir, "runs", c.ID, sanitize(arm), res.Model, fmt.Sprintf("r%d", j.Idx))
	defer func() {
		res.Metrics.WallMs = time.Since(started).Milliseconds()
		raw, _ := json.MarshalIndent(res, "", "  ")
		_ = os.WriteFile(filepath.Join(dir, "result.json"), raw, 0o644)
		if !opt.Keep && res.Status == "pass" {
			// A passing run keeps its record (result, run, seed, transcript,
			// backend log); the database, CLI state and workspace go.
			for _, d := range []string{"backend", "cli", "work", ".twevals-bin", ".twevals-home"} {
				_ = os.RemoveAll(filepath.Join(dir, d))
			}
		}
	}()
	infra := func(err error) *Result {
		res.Status, res.Error = agent.StopInfra, err.Error()
		logf(opt, "  %s infra_error: %v", RunKey(c.ID, arm, res.Model, j.Idx), err)
		return res
	}
	// A rerun into the same --out starts clean: a database left by an
	// earlier failed run already has its owner registered.
	if err := os.RemoveAll(dir); err != nil {
		return infra(err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return infra(err)
	}
	logf(opt, "▶ %s", RunKey(c.ID, arm, res.Model, j.Idx))

	inst, err := env.NewInstance(ctx, opt.Bins, dir)
	if err != nil {
		return infra(fmt.Errorf("boot: %w", err))
	}
	defer func() {
		_ = inst.Backend.WriteLog(filepath.Join(dir, "backend.log"))
		inst.Close()
	}()

	sink := seed.NewLiveSink(ctx, inst.Backend, inst.Admin)
	sink.StubSrv = inst.Stub
	sc := seed.NewScenario(c.ID, time.Now(), sink)
	if err := scenarios.Run(c.Seed, sc); err != nil {
		return infra(err)
	}
	if err := sc.Verify(60 * time.Second); err != nil {
		return infra(err)
	}
	stats := sc.Stats()
	res.Seed = &stats
	projects := sink.Projects()
	if len(projects) == 0 && c.Suite != "setup" {
		return infra(fmt.Errorf("seed %s created no project", c.Seed))
	}
	exposed := sc.Exposed()
	primary := primaryProject(exposed, projects)

	envv := envVars(inst, projects, primary, sc.Now)
	if c.Setup != nil && c.Setup.Token == "setup" {
		tok, err := env.SetupToken(ctx, inst.Backend, inst.Admin)
		if err != nil {
			return infra(err)
		}
		envv["setupToken"] = tok
	}
	vars := map[string]any{"seed": jsonify(exposed), "env": envv}
	resolved, err := c.Resolve(vars)
	if err != nil {
		return infra(err)
	}
	writeJSON(filepath.Join(dir, "seed.json"), map[string]any{"exposed": jsonify(exposed), "env": vars["env"], "stats": stats})

	work := filepath.Join(dir, "work")
	if err := buildWorkspace(opt.EvalsRoot, resolved.Workspace.Repo, work, sc.Now); err != nil {
		return infra(err)
	}
	surf, cleanup, err := prepareSurface(ctx, opt, resolved, arm, inst, projects, primary, dir, work)
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		return infra(err)
	}
	if err := installSkills(opt.RepoRoot, work, surf.skills); err != nil {
		return infra(err)
	}
	var pristine map[string]string
	if resolved.Expect.Code != nil {
		if pristine, err = grade.Snapshot(work); err != nil {
			return infra(err)
		}
	}
	if surf.setupShell {
		port, err := env.FreePort()
		if err != nil {
			return infra(err)
		}
		if surf.containerEnv, err = setupEnv(resolved, inst, projects, port); err != nil {
			return infra(err)
		}
	}

	sb := &tools.Sandbox{Root: work, ReadOnly: []string{".skills"}}
	ask := &tools.AskUser{Rules: resolved.User}
	var loaded []string
	var submit *tools.Submit
	var toolset []tools.Tool
	var calls []*record.Call
	var callsMu sync.Mutex
	callExists := func(id string) (bool, bool) {
		callsMu.Lock()
		defer callsMu.Unlock()
		for _, cl := range calls {
			if cl.ID == id {
				return cl.OK, true
			}
		}
		return false, false
	}
	var agentShell *ContainerShell
	if surf.setupShell {
		cs, err := startContainerShell(ctx, opt, work, surf)
		if err != nil {
			return infra(err)
		}
		defer cs.Close()
		agentShell = cs
		toolset = append(toolset, cs)
	} else {
		sh := &tools.Shell{Sandbox: sb, Git: true, Denied: deniedFor(arm)}
		if surf.cliHome != nil {
			sh.Bins = map[string]string{surf.cliName: surf.cliBin}
			sh.BinEnv = surf.cliHome.Env(filepath.Dir(surf.cliBin))
		}
		toolset = append(toolset, sh)
	}
	toolset = append(toolset, &tools.ReadFile{Sandbox: sb}, &tools.ListFiles{Sandbox: sb}, &tools.Search{Sandbox: sb},
		&tools.EditFile{Sandbox: sb}, &tools.WriteFile{Sandbox: sb}, ask)
	skillDefs, _ := tools.LoadSkills(filepath.Join(work, ".skills"))
	if len(skillDefs) > 0 {
		toolset = append(toolset, &tools.LoadSkill{Skills: skillDefs, Rel: sb.Rel, Loaded: &loaded})
	}
	if surf.mcp != nil {
		toolset = append(toolset, surf.mcp.Tools()...)
	}
	submit = &tools.Submit{Kind: surf.submit, CallExists: callExists, MaxRepairs: 2}
	toolset = append(toolset, submit)

	prompt := strings.TrimSpace(resolved.Prompt)
	var preloaded *tools.Skill
	if resolved.Invoke == "slash" {
		name := agent.SlashSkill(prompt)
		for i := range skillDefs {
			if skillDefs[i].Name == name {
				preloaded = &skillDefs[i]
				loaded = append(loaded, name)
			}
		}
		if preloaded == nil {
			prompt = agent.StripSlash(prompt)
		}
	}
	system := agent.SystemPrompt(agent.PromptInput{
		SubmitTool: surf.submit, Skills: skillDefs, Preloaded: preloaded,
		MCPInstructions: mcpInstructions(surf), Environment: environmentNote(surf, inst, primary),
	})

	var chat agent.Chatter
	var fake *agent.Fake
	if opt.Oracle {
		steps := resolved.OracleFor(arm)
		fake = agent.NewFake(steps, surf.cliName, filepath.Join(opt.EvalsRoot, "testdata"))
		defer fake.Close()
		chat = fake.Client()
	} else {
		chat = opt.Chat(j.Model)
	}
	tr, err := agent.OpenTranscript(filepath.Join(dir, "transcript.jsonl"))
	if err != nil {
		return infra(err)
	}
	defer tr.Close()
	res.Transcript, _ = filepath.Rel(opt.OutDir, filepath.Join(dir, "transcript.jsonl"))

	hooks := newHookRunner(resolved, inst, primary, sink)
	stopApprover := startDraftApprover(ctx, resolved, inst)
	defer stopApprover()

	model := j.Model
	if opt.Oracle {
		model = &catalog.Model{Key: "oracle", ID: "fake/oracle", Caching: "none"}
	}
	run := agent.Run(ctx, agent.Config{
		Model: model, Chat: chat, Tools: toolset, System: system, Prompt: prompt, SubmitTool: surf.submit,
		Budget: resolved.Budget, Hold: hold, Ledger: opt.Ledger, Price: opt.Prices[j.Model.Key], Transcript: tr,
		AfterCall: func(ctx context.Context, cl *record.Call) {
			callsMu.Lock()
			calls = append(calls, cl)
			callsMu.Unlock()
			hooks.after(ctx, cl)
		},
		Questions:   ask.Questions,
		SkillLoaded: func() []string { return loaded },
	})
	writeJSON(filepath.Join(dir, "run.json"), run)
	if agentShell != nil {
		// Whatever the agent left running (its own copy of the app on PORT)
		// must be gone before the verify step starts the app afresh.
		agentShell.Close()
	}

	res.Providers = run.Providers
	res.Metrics = metricsOf(run)
	if fake != nil {
		if errs := fake.Errors(); len(errs) > 0 {
			res.Error = "oracle: " + strings.Join(errs, "; ")
		}
	}
	if errs := hooks.errors(); len(errs) > 0 {
		return infra(fmt.Errorf("hook failed: %s", strings.Join(errs, "; ")))
	}
	switch run.StopReason {
	case agent.StopInfra:
		res.Status, res.Error = agent.StopInfra, run.StopDetail
		return res
	case agent.StopCeiling:
		res.Status, res.Error = agent.StopCeiling, run.StopDetail
		return res
	}

	in := &grade.Input{
		Case: resolved, Arm: arm, Run: run, Seed: jsonify(exposed), Workspace: work,
		State: &stateReader{inst: inst, projects: projects, primary: primary}, FakeModel: opt.Oracle,
		AllowHostTests: opt.AllowHostTests, SandboxImage: opt.SandboxImage, EvalsRoot: opt.EvalsRoot, Judge: opt.Judge, Hold: hold,
		Pristine: pristine,
	}
	if resolved.Setup != nil && resolved.Setup.Verify != nil {
		in.VerifyStart = time.Now()
		if v := verifySetup(ctx, opt, resolved, work, inst, projects, surf); v != nil {
			res.Graders = append(res.Graders, v)
		}
	}
	res.Graders = append(res.Graders, grade.All(ctx, in)...)
	for _, v := range res.Graders {
		res.Metrics.JudgeCostUSD += v.CostUSD
	}
	switch {
	case run.StopReason == agent.StopBudget:
		res.Status = "budget_exhausted"
	case res.Error != "" && opt.Oracle:
		res.Status = "fail"
	case grade.Passed(res.Graders):
		res.Status = "pass"
	default:
		res.Status = "fail"
	}
	if opt.Oracle {
		logf(opt, "  %s %s (%d calls)", RunKey(c.ID, arm, res.Model, j.Idx), res.Status, res.Metrics.ToolCalls)
	} else {
		logf(opt, "  %s %s (%d calls, $%.4f)", RunKey(c.ID, arm, res.Model, j.Idx), res.Status, res.Metrics.ToolCalls, res.Metrics.CostUSD)
	}
	return res
}

func primaryProject(exposed map[string]any, projects []env.Project) env.Project {
	if name, ok := exposed["primaryProject"].(string); ok {
		for _, p := range projects {
			if p.Name == name {
				return p
			}
		}
	}
	if len(projects) == 0 {
		return env.Project{}
	}
	return projects[0]
}

func envVars(inst *env.Instance, projects []env.Project, primary env.Project, now time.Time) map[string]any {
	ps := map[string]any{}
	for _, p := range projects {
		ps[p.Name] = map[string]any{"id": p.ID, "name": p.Name, "framework": p.Framework}
	}
	return map[string]any{
		"url": inst.Backend.URL, "projects": ps, "project": primary.ID, "projectName": primary.Name,
		"org": float64(inst.Admin.OrgID), "stub": inst.Stub.URL, "now": now.UTC().Format(time.RFC3339),
	}
}

// jsonify round-trips a value through JSON so templates and graders see
// plain maps, strings and float64s.
func jsonify(v map[string]any) map[string]any {
	raw, _ := json.Marshal(v)
	var out map[string]any
	_ = json.Unmarshal(raw, &out)
	if out == nil {
		out = map[string]any{}
	}
	return out
}

func writeJSON(path string, v any) {
	raw, err := json.MarshalIndent(v, "", "  ")
	if err == nil {
		_ = os.WriteFile(path, raw, 0o644)
	}
}

func sanitize(s string) string { return strings.NewReplacer("+", "_", "/", "_").Replace(s) }

func metricsOf(run *record.Run) Metrics {
	m := Metrics{Turns: run.Turns, InputTokens: run.Usage.InputTokens, CachedTokens: run.Usage.CachedTokens,
		CacheWriteTokens: run.Usage.CacheWriteTokens, OutputTokens: run.Usage.OutputTokens, CostUSD: run.Usage.CostUSD,
		Skills: run.SkillLoaded, SkillLoaded: len(run.SkillLoaded) > 0}
	for _, c := range run.Calls {
		if c.Rejected {
			continue
		}
		m.ToolCalls++
		if !c.OK {
			m.FailedCalls++
		}
	}
	return m
}

func deniedFor(arm string) map[string]string {
	d := map[string]string{
		"curl": "no network tools in this sandbox", "wget": "no network tools in this sandbox",
		"python": "no interpreters in this sandbox", "python3": "no interpreters in this sandbox", "node": "no interpreters in this sandbox",
		"bash": "this shell already is the interpreter; run the command directly", "sh": "this shell already is the interpreter; run the command directly",
	}
	switch {
	case catalog.IsMCPArm(arm):
		d["traceway"] = "the traceway CLI is not installed on this machine; use the connected Traceway MCP tools"
		d["traceway-write"] = d["traceway"]
	case arm == catalog.ArmCLIWriteSkill:
		d["traceway"] = "not installed here; use traceway-write, which has every read command too"
	default:
		d["traceway-write"] = "traceway-write is not installed on this machine"
	}
	return d
}

func mcpInstructions(s surface) string {
	if s.mcp == nil {
		return ""
	}
	return s.mcp.Instructions
}

func environmentNote(s surface, inst *env.Instance, primary env.Project) string {
	switch {
	case s.setupShell:
		return "This is the developer's machine. The shell has go, node, npm, python3, pip, git and curl, and the traceway CLI (not logged in). Package registries are reachable. The team's Traceway instance runs at " + inst.Backend.URL + "."
	case s.mcp != nil:
		return fmt.Sprintf("A Traceway MCP server for the team's instance (%s) is connected. Its tools default to the %q project when you omit project_id.", inst.Backend.URL, primary.Name)
	case s.cliHome != nil:
		return fmt.Sprintf("The %s CLI is installed and logged in to the team's Traceway instance at %s, with the %q project selected.", s.cliName, inst.Backend.URL, primary.Name)
	}
	return ""
}
