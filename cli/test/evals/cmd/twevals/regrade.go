package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/catalog"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/grade"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/record"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/report"
)

// cmdRegrade runs the offline graders again over a run directory's stored
// records (run.json and seed.json next to each transcript) with the cases
// as they are now, keeps the verdicts that needed the live instance (state,
// hidden tests, judge, setup verify), and recomputes each run's status. It
// is how a grader fix is applied to runs already paid for.
func cmdRegrade(args []string) error {
	fs := flag.NewFlagSet("regrade", flag.ExitOnError)
	out := fs.String("out", "", "write results.json and summary.md here (default: the run directory itself)")
	pos := parseInterleaved(fs, args)
	if len(pos) != 1 {
		return errors.New("regrade needs exactly one run directory")
	}
	ws, err := load()
	if err != nil {
		return err
	}
	dir := pos[0]
	res, err := report.Load(dir)
	if err != nil {
		return err
	}
	byID := map[string]*catalog.Case{}
	for _, c := range ws.cases {
		byID[c.ID] = c
	}
	type change struct {
		key, from, to string
		was, now      []string
	}
	var changes []change
	skipped := 0
	for _, r := range res.Runs {
		switch r.Status {
		case "infra_error", "cost_ceiling":
			continue
		}
		c := byID[r.Case]
		if c == nil || r.Transcript == "" {
			skipped++
			continue
		}
		runDir := filepath.Dir(filepath.Join(dir, filepath.FromSlash(r.Transcript)))
		run, seed, err := loadRunRecord(runDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "note: %s: %v (kept as recorded)\n", r.Key(), err)
			skipped++
			continue
		}
		resolved, err := c.Resolve(map[string]any{"seed": seed["exposed"], "env": seed["env"]})
		if err != nil {
			return fmt.Errorf("%s: %w", r.Key(), err)
		}
		in := &grade.Input{Case: resolved, Arm: r.Arm, Run: run, EvalsRoot: ws.root}
		// A failed run keeps its workspace; the culprit-line mapping reads
		// the edited file from it rather than replaying the edits.
		if fi, err := os.Stat(filepath.Join(runDir, "work")); err == nil && fi.IsDir() {
			in.Workspace = filepath.Join(runDir, "work")
		}
		if m, ok := seed["exposed"].(map[string]any); ok {
			in.Seed = m
		}
		before := r.Status
		was := gatingFailures(r.Graders)
		r.Graders = mergeVerdicts(r.Graders, grade.Offline(in))
		if before != "budget_exhausted" {
			r.Status = "fail"
			if grade.Passed(r.Graders) {
				r.Status = "pass"
			}
		}
		if now := gatingFailures(r.Graders); before != r.Status || strings.Join(was, "|") != strings.Join(now, "|") {
			changes = append(changes, change{r.Key(), before, r.Status, was, now})
		}
	}
	res.Header.Regraded = time.Now().UTC().Format(time.RFC3339)
	target := *out
	if target == "" {
		target = dir
	}
	if err := os.MkdirAll(target, 0o755); err != nil {
		return err
	}
	if err := report.Save(target, res); err != nil {
		return err
	}
	if target == dir {
		for _, r := range res.Runs {
			if r.Transcript == "" {
				continue
			}
			raw, _ := json.MarshalIndent(r, "", "  ")
			_ = os.WriteFile(filepath.Join(filepath.Dir(filepath.Join(dir, filepath.FromSlash(r.Transcript))), "result.json"), raw, 0o644)
		}
	}
	flipped := 0
	for _, ch := range changes {
		if ch.from != ch.to {
			flipped++
		}
	}
	fmt.Printf("regraded %d runs (%d without a record, kept as recorded): %d changed a verdict, %d changed status\n", len(res.Runs), skipped, len(changes), flipped)
	for _, ch := range changes {
		fmt.Printf("  %s: %s -> %s\n", ch.key, ch.from, ch.to)
		if len(ch.was) > 0 {
			fmt.Printf("    was: %s\n", strings.Join(ch.was, "; "))
		}
		if len(ch.now) > 0 {
			fmt.Printf("    now: %s\n", strings.Join(ch.now, "; "))
		}
	}
	return nil
}

// loadRunRecord reads a run's record and seed. A run directory without a
// run.json (the process died before it was written) is rebuilt from its
// transcript, which carries the same calls, prompt and submission.
func loadRunRecord(runDir string) (*record.Run, map[string]any, error) {
	var run record.Run
	raw, err := os.ReadFile(filepath.Join(runDir, "run.json"))
	switch {
	case err == nil:
		if err := json.Unmarshal(raw, &run); err != nil {
			return nil, nil, fmt.Errorf("run.json: %w", err)
		}
	case errors.Is(err, os.ErrNotExist):
		r, terr := runFromTranscript(filepath.Join(runDir, "transcript.jsonl"))
		if terr != nil {
			return nil, nil, fmt.Errorf("%w; transcript: %v", err, terr)
		}
		run = *r
	default:
		return nil, nil, err
	}
	raw, err = os.ReadFile(filepath.Join(runDir, "seed.json"))
	if err != nil {
		return nil, nil, err
	}
	var seed map[string]any
	if err := json.Unmarshal(raw, &seed); err != nil {
		return nil, nil, fmt.Errorf("seed.json: %w", err)
	}
	return &run, seed, nil
}

// runFromTranscript rebuilds a run record from transcript.jsonl: the start
// event's prompt, every tool event's call, the accepted submit call's
// arguments, the last response text and the stop event.
func runFromTranscript(path string) (*record.Run, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	run := &record.Run{}
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var ev struct {
			Type    string          `json:"type"`
			Prompt  string          `json:"prompt"`
			Call    *record.Call    `json:"call"`
			Content json.RawMessage `json:"content"`
			Reason  string          `json:"reason"`
			Detail  string          `json:"detail"`
			Turns   int             `json:"turns"`
			Usage   *record.Usage   `json:"usage"`
		}
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			return nil, err
		}
		switch ev.Type {
		case "start":
			run.Prompt = ev.Prompt
		case "tool":
			if ev.Call == nil {
				continue
			}
			run.Calls = append(run.Calls, ev.Call)
			if strings.HasPrefix(ev.Call.Tool, "submit") && ev.Call.OK && strings.HasPrefix(ev.Call.Output, "Submission received") {
				var sub map[string]any
				if json.Unmarshal(ev.Call.Args, &sub) == nil {
					run.Submission, run.SubmitTool, run.SubmitCall = sub, ev.Call.Tool, ev.Call.ID
				}
			}
		case "response":
			var text string
			if json.Unmarshal(ev.Content, &text) == nil && text != "" {
				run.FinalText = text
			}
		case "stop":
			run.StopReason, run.StopDetail, run.Turns = ev.Reason, ev.Detail, ev.Turns
			if ev.Usage != nil {
				run.Usage = *ev.Usage
			}
		}
	}
	if run.StopReason == "" {
		return nil, errors.New("no stop event")
	}
	return run, nil
}

// mergeVerdicts replaces the recorded verdicts the fresh ones cover, in
// place, and keeps the rest (state, hidden tests, judge, setup verify).
func mergeVerdicts(recorded, fresh []*grade.Verdict) []*grade.Verdict {
	out := make([]*grade.Verdict, 0, len(recorded)+len(fresh))
	used := map[string]bool{}
	for _, v := range recorded {
		replaced := false
		for _, f := range fresh {
			if f.Grader == v.Grader {
				out = append(out, f)
				used[f.Grader] = true
				replaced = true
				break
			}
		}
		if !replaced {
			out = append(out, v)
		}
	}
	for _, f := range fresh {
		if !used[f.Grader] {
			out = append(out, f)
		}
	}
	return out
}

func gatingFailures(vs []*grade.Verdict) []string {
	var out []string
	for _, v := range vs {
		if v.Gating && !v.Skipped && !v.Pass {
			for _, f := range v.Failures {
				out = append(out, v.Grader+": "+f)
			}
		}
	}
	return out
}
