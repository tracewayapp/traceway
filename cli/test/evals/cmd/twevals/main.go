// Command twevals runs the Traceway skill and MCP evals.
//
// Manual only. Paid runs happen in the CLI Evals workflow, which a person
// starts by hand (workflow_dispatch) and approves at the evals environment
// gate. `twevals run` refuses every other CI context, and locally it needs a
// confirmation typed on a terminal. `twevals oracle` and `twevals plan` are free.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

const usage = `twevals: Traceway skill and MCP evals (manual only)

Free:
  twevals list    [--preset P] [--suites S] [--tags T] [--cases GLOB] [--arms A]
  twevals plan    [selection] --models opus,astra [--runs N] --max-cost-usd X [--shards] [--out plan.json] [--summary FILE]
  twevals oracle  [--all | <case-id>... | --suites S] [--arms A] [-j N] [--keep] [--out DIR]
  twevals env up  <case-id>
  twevals report  <run-dir>... [--merge] [--replace] [--out DIR]
  twevals regrade <run-dir> [--out DIR]
  twevals compare <run-dir> [baseline.json]
  twevals baseline update <run-dir> [--out baseline.json]

Paid (OpenRouter; guarded, see README):
  twevals run     [selection] [--shard SUITE] --models opus,astra --max-cost-usd X [--runs N] [-j N] [--keep] [--judge=true]
  twevals resume  <run-dir> --max-cost-usd X

Selection flags: --preset smoke|core|full, --cases (comma-separated ids or globs; overrides the preset), --suites, --tags, --arms.
list defaults to --preset full, plan and run to --preset smoke. oracle writes results.json and summary.md to --out
(default .cache/oracle/<timestamp>). regrade runs the offline graders (answer, trajectory, grounding, report) again
over a run directory's stored records and keeps the state, hidden-test and judge verdicts; it rewrites the directory's
results.json and summary.md unless --out names another place.
Exit codes: 0 ok, 1 a regression, a failed oracle or an error, 2 bad usage or partial (cost ceiling, or infra_error above 10%).
`

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	var err error
	args := os.Args[2:]
	switch os.Args[1] {
	case "list":
		err = cmdList(args)
	case "plan":
		err = cmdPlan(ctx, args)
	case "run":
		err = cmdRun(ctx, args, "")
	case "resume":
		if len(args) == 0 {
			err = errors.New("resume needs a run directory")
			break
		}
		err = cmdRun(ctx, args[1:], args[0])
	case "oracle":
		err = cmdOracle(ctx, args)
	case "env":
		if len(args) == 0 || args[0] != "up" {
			err = errors.New("usage: twevals env up <case-id>")
			break
		}
		err = cmdEnvUp(ctx, args[1:])
	case "report":
		err = cmdReport(args)
	case "regrade":
		err = cmdRegrade(args)
	case "compare":
		err = cmdCompare(args)
	case "baseline":
		if len(args) == 0 || args[0] != "update" {
			err = errors.New("usage: twevals baseline update <run-dir>")
			break
		}
		err = cmdBaselineUpdate(args[1:])
	case "-h", "--help", "help":
		fmt.Print(usage)
		return
	default:
		err = &exitError{2, fmt.Sprintf("unknown command %q (twevals help lists them)", os.Args[1])}
	}
	if err != nil {
		var ex *exitError
		if errors.As(err, &ex) {
			if ex.msg != "" {
				fmt.Fprintln(os.Stderr, "twevals:", ex.msg)
			}
			os.Exit(ex.code)
		}
		fmt.Fprintln(os.Stderr, "twevals:", err)
		os.Exit(1)
	}
}

type exitError struct {
	code int
	msg  string
}

func (e *exitError) Error() string { return e.msg }
