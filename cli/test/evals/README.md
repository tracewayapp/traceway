# Traceway skill and MCP evals

**Manual only.** Evals run from the CLI Evals workflow, which you start by hand
(`workflow_dispatch`) and approve at the `evals` environment gate. Nothing
triggers it automatically.

This module measures how well AI agents use Traceway through the skills
(`traceway`, `traceway-write`, `traceway-setup`) and the MCP servers (`/mcp`,
`/mcp/write`), for read and write work. Every run gets its own hermetic
Traceway instance, the standalone backend in SQLite mode, seeded through the
real ingest endpoints. The agent works through real `traceway` commands or MCP
tools, and deterministic graders score the result.

| Suite | Measures | Arms |
|---|---|---|
| perf | Diagnosing latency against every bottleneck row in `performance.md` | `cli+skill`, `cli-noskill`, `mcp` |
| debug | Debugging an issue to root cause and fixing the code | `cli+skill`, `cli-noskill`, `mcp` |
| write | The write playbooks (pages, alerts, monitors, archive, post-mortems, slow marks) and the 403 policy | `cli-write+skill`, `mcp-write` |
| setup | Instrumenting an app with the setup skill until its telemetry arrives | `cli+setup-skill` |

The agents are `anthropic/claude-opus-5.5` and `openai/gpt-6-astra`, and the
judge is `anthropic/claude-haiku-5.5`. All three run on OpenRouter, in the same
harness, with the same tools, prompts and budgets (`models.yaml`).

## The manual-only policy

1. **Paid runs happen in GitHub, started by a person.** Actions → CLI Evals →
   Run workflow (or `gh workflow run cli-evals.yml`). The free preflight job
   writes the matrix and a live-price estimate into the job summary. The
   paid job waits for a required reviewer on the `evals` environment, which
   is the only place `OPENROUTER_API_KEY` exists. Store the key there only,
   never as a repository or organization secret: GitHub creates a missing
   environment on first use with no reviewer, so the preflight refuses a paid
   run unless `evals` exists with a required reviewer.
2. **Nothing runs automatically.** `cli-evals.yml` has exactly one trigger,
   `workflow_dispatch`. There is no push, pull request, label, schedule,
   `workflow_run`, `workflow_call` or `repository_dispatch` trigger. No other
   workflow calls it, and no `just` recipe in `check`/`test`/`default` runs it.
3. **Enforced in code:**
   - `internal/guard/triggers_test.go` fails if the workflow gains a trigger, a
     paid job loses `environment: evals`, the approval flag moves out of that
     job, another workflow or composite action references it (by file, by
     name, through `just eval-dispatch` or the key's name), uses the `evals`
     environment or starts `twevals run`, or a recipe reachable from `just
     check`/`test`/`default` runs or dispatches the evals. Because this module
     is its own Go module, the CLI workflow never runs it; `eval-guard.yml`
     does, on every change to a workflow, an action, the justfile or this
     module (label-gated like the other workflows).
   - `twevals run` and `twevals resume` start in CI only inside a dispatched
     `cli-evals.yml` job that set `TWEVALS_APPROVED=1` (the gated job does).
     They refuse any other GitHub event, any other workflow, and any other CI
     system.
   - `--max-cost-usd` is required, with no default, and must be a finite
     number above 0. The workflow splits it into per-shard caps. Each run
     reserves its share when it starts, so the runs in flight are never
     promised more than is left, and a run stops before a model call that
     would likely cross its share. The cap can still be passed by the error in
     that estimate (about one turn's growth per run in flight) and the judge's
     few cents.
   - `go test ./...` can never call OpenRouter: the real client requires
     `guard.Live()`, which is false under `go test`.
4. **Local paid runs are for harness development only.** You must type `yes`
   on a terminal after reading the preflight. Without a terminal (a script, or
   an agent's shell) a local paid run is refused, and `--yes` does not stand in
   for the typed confirmation; it only serves the dispatched workflow. Recorded
   results (checkpoints, baselines) come from the workflow.

## Running it

### In GitHub (the normal path)

```
gh workflow run cli-evals.yml --ref <branch> -f preset=smoke -f models=opus,astra -f max_cost_usd=30
```

Then open the run, read the preflight estimate in the summary, and approve
the `evals` gate. `just eval-dispatch -f preset=smoke -f max_cost_usd=30` from
`cli/` does the same.

| Input | Meaning |
|---|---|
| `preset` | `smoke`: the cases tagged `smoke`, primary arm. `core`: every case, primary arm. `full`: every arm the case lists |
| `cases` | Comma-separated ids or globs (`perf-db-*`). Overrides the preset's case filter |
| `suites`, `arms` | Narrow the selection |
| `models` | `opus,astra`, `opus` or `astra` |
| `runs` | Repeats per case, 1-3 |
| `max_cost_usd` | Cap for the whole dispatch, split per shard (see the policy above). Required |
| `dry_run` | Preflight, harness tests and every oracle only. Costs $0. Use it to validate workflow edits and Dependabot bumps |
| `judge` | Score reports with the (non-gating) LLM judge |
| `compare_baseline` | Compare against `baseline.json`; a regression turns the report job red |

The workflow never commits. It uploads `results.json`, `summary.md` and every
transcript, with tokens redacted, for 30 days.

### Locally (free)

From `cli/` (each recipe runs only when you type it):

| Command | What it does |
|---|---|
| `just eval-harness` | `go test ./...` plus every oracle trajectory, with the fake model. Free |
| `just eval-plan --preset core --max-cost-usd 200` | The preflight table and cost estimate. Free |
| `just eval-env perf-db-n-plus-1` | Boots and seeds one case, then prints the dashboard URL, a login and a read PAT so you can look at the data by eye |
| `just eval-local --cases perf-db-n-plus-1 --models opus --max-cost-usd 5` | A paid local run, for harness development only. It asks you to type `yes` on a terminal |
| `just eval-dispatch ...` | `gh workflow run cli-evals.yml ...` |

Directly from `cli/test/evals`, `go run ./cmd/twevals help` lists every
command. `twevals oracle <id>...` (or `--all`, or `--suites S`) plays a case's
oracle on every arm it lists and writes `results.json` and `summary.md` to
`--out` (default `.cache/oracle/<timestamp>/`).
`twevals report <dir>... --out DIR` merges shard results (`--out` is
required for more than one directory, and a run that appears twice is an
error). `twevals compare <dir>`
applies the regression rule. `twevals baseline update <dir>` rewrites
`baseline.json` for you to commit.

Requirements: Go (the `go`/`toolchain` lines of `go.mod`), git, and Docker for
the setup suite. Docker also runs the hidden tests of a model's patch with no
network and an empty module cache, so a fixture with hidden tests must not
need module dependencies. Without Docker, setup runs of a real model fail, and hidden tests of
a real model's patch are skipped unless `TWEVALS_HOST_TESTS=1` (the diff
checks, such as touching the culprit, still gate). Oracle runs (our own code)
use the host when Docker is missing. The setup oracles install pinned
packages (Go modules, npm and PyPI), so `oracle --all` needs those registries
or warm caches, and `node`/`npm` and `python3` with `venv` on the host.

### Exit codes

`0` ok, `1` regression (compare), a failed oracle or an error, `2` bad usage
or partial: the cost ceiling stopped the run (continue with `twevals resume
<dir> --max-cost-usd X`) or more than 10% of runs hit infra errors.

## Cost

Estimates per run, from `models.yaml` until `baseline.json` has measured
averages: 100-400k cumulative input tokens (about 70% cached), 5-15k output.

| Model | Per run (typical) | Per run (no cache) |
|---|---|---|
| Opus 5.5 | ~$0.26-0.96 | up to ~$1.90 |
| GPT-6 Astra | ~$0.62-2.23 | up to ~$4.75 |
| Judge (Haiku 5.5) | under $0.01 | |

| Preset, 1 repeat | Runs per model | Both models (live prices, 2026-10-17) |
|---|---|---|
| `smoke` | 12 | ~$11-38 |
| `core` | 68 | ~$60-217 |
| `full` | 178 | ~$158-568 |

`twevals plan` prints the real numbers from live OpenRouter prices, including
the cache-write price and long-prompt tiers. Controls: the required cap, the
preflight, the gate, per-shard caps, a reservation per run and a check before
every model call, and `dry_run`.

## Reading the report

`summary.md` has these sections:

- pass rates by suite × model × arm;
- skill uplift (`cli+skill` minus `cli-noskill`, on the same cases);
- CLI vs MCP;
- model vs model;
- cost and cache hit rate per model;
- the top failing graders and the worst cases;
- one line per run, with its first failure.

A run is `pass`, `fail`, `budget_exhausted` (the case's own turn, call,
token or time budget ran out; counted as a fail), `infra_error` or
`cost_ceiling` (not started, or stopped at its share of `--max-cost-usd`);
the last two are excluded from pass rates, and `twevals resume` runs them
again.

Each run directory, `runs/<case>/<arm>/<model>/r<N>/` (with `+` in the arm
written as `_`, so `cli_skill`), holds:

- `transcript.jsonl`: every model response with usage and provider, every
  tool call with its id, op, argv or args, exit code and output;
- `run.json` and `result.json`;
- `seed.json`: the ids the seed exposed. The agent never sees it;
- the workspace, when the run failed or `--keep` was set.

Graders:

| Grader | Gates | Checks |
|---|---|---|
| `answer` | yes | The structured answer: checklist row (and not the distractor), locus (text after the endpoint's route, such as "(and every route behind the guard)", is ignored), onset; or the debug category, culprit file, symbol and line (±3) |
| `trajectory` | yes | Required, alternative, forbidden and ordered operations. CLI and MCP calls map to the same operations (`internal/ops`) |
| `efficiency` | no | The thrash limits in `expect.calls`: `max_calls`, `max_failed_calls` and `max_identical_repeats`. An operation counts once per tool call, so one shell loop is one call, and a `git` op is keyed by its argv. Re-running a call to check a JSON shape is not a wrong answer, so these are counted in the summary's Efficiency table instead of failing the run. The case `budget` still stops a run that spins |
| `grounding` | yes | Cited calls exist, succeeded and were not truncated. A shell call succeeded when one of its operations (a `traceway` or `git` command) did, even if a later command in it (a `jq`) failed; then only what the model saw counts. Their output contains the deciding signal. No id in the answer was invented |
| `state` | yes | After write and setup runs: the backend and the workspace are in the expected state |
| `hidden_tests` | per case | Hidden tests copied in after the run, and whether the diff (against the workspace before the run) touches the culprit |
| `setup_verify` | yes | The instrumented app builds, starts against the backend, and serves traffic |
| `judge` | no | The rubric, scored by Haiku. It does not gate until it agrees with human labels on at least 85% of runs |

## Baseline

`baseline.json` holds pass rates per suite and case and the measured per-run
usage per model, which the preflight uses instead of the `models.yaml`
envelope. To update it, download the `eval-report` artifact, run
`go run ./cmd/twevals baseline update merged`, review the diff, and commit it
in a normal PR.

The regression rule: a suite's pass rate drops more than 10 percentage points
for a model × arm, or a case that passed at least 2 runs, at 2 of 3 or better,
in the baseline now passes 0 of N. With one run per case the suite rule is the
only one that can fire, and a suite of few cases moves in large steps (one
failure of 4 runs is 25 points).

## Adding a case

A case is three things:

- a seed function in `internal/scenarios/<suite>/`, registered by name;
- a YAML file `cases/<suite>/<id>.yaml`;
- fixtures under `testdata/`.

`internal/suite` tests the catalog as a whole: references exist, checklist ids
are valid, and the leakage lint is clean. Done means:

1. The seed builds data with exactly one correct answer and one named
   distractor (`distractor:` in the YAML).
2. `twevals oracle <id>` passes on every arm the case lists.
3. `go test ./internal/suite/` passes.

The exemplars show every part: `perf-db-n-plus-1`, `debug-go-nil-regression`,
`write-alert-snooze`, `setup-go-nethttp`.

### The seed DSL (`internal/seed`)

```go
func init() { scenarios.Register("perf/db-n-plus-1", seedNPlusOne) }

func seedNPlusOne(s *seed.Scenario) error {
	p := s.Project("storefront", seed.OTel)                 // a real project, created through the API
	api := p.Service("orders-api", seed.Go("nethttp", "sql")).Versions("2.14.3")
	orders := api.Endpoint("GET", "/api/orders")
	orders.Traffic(seed.Daily(160), seed.Diurnal(), seed.Over(29*24*time.Hour))
	orders.Handler(func(r *seed.Req) {
		n := int(r.Lerp(6, 72) * (0.35 + 0.95*r.Float())) // grows over the window
		r.DB("SELECT * FROM orders WHERE user_id = $1 ...", r.Ms(3, 6), seed.Rows(n))
		for range n {
			r.DB("SELECT id, sku, qty, unit_price FROM order_items WHERE order_id = $1", r.Ms(6, 14))
		}
		r.Compute("encode response", r.Ms(2, 6))
	})
	s.Background(api, seed.TypicalTraffic(10)) // healthy endpoints, a benign 400, host metrics
	api.Host().CPU(seed.Flat(0.22, 0.31))
	s.Expose("slowRequest", orders.Slowest(24*time.Hour)) // for the oracle and graders, never the agent
	return nil
}
```

- **Timing is consistent by construction.** Work is laid out on a cursor.
  `r.DB`, `r.HTTP`, `r.Cache`, `r.Compute` and `r.Call(otherEndpoint, rtt)`
  each add a child span. `r.Gap(d)` is waiting time with no span. `r.Span`
  nests, and `r.Parallel` runs branches side by side. A parent's duration is
  where its cursor stops.
- **Releases** are `Versions(initial, seed.Deploy{At, Version})`, which sets
  the `service.version` resource attribute. `r.Version()` lets the handler
  change behaviour after a deploy.
- **Multiple instances** are `Instances("node-a", ...)`, with `r.Host()` and
  `HostNamed(h).CPU(...)`.
- **Traffic shapes** are `Daily`, `Diurnal`, `GrowRate`, `Burst`, `RateFunc`
  and `Over`. Use `Statuses` or `r.Status` to fail requests.
- **Logs and errors:**
  - `r.Log(sev, body, attrs...)` writes a trace-correlated OTLP log.
  - `svc.Log(at, ...)` writes a log outside any request.
  - `r.Error(seed.Exc{Type, Message, Stack})` records an exception event in
    the stack format of the service's language.
  - Every detailed request also gets an access-log line,
    `GET /path 200 1243ms`, with its trace id. That is how an agent finds a
    slow request's id, because `endpoints list` has none.
- **Metrics** come from `Host().CPU/Memory/Disk/Filesystem` (OTel
  hostmetrics names, `system.cpu.utilization` split by `state`) and from
  `svc.Gauge/Counter/Histogram(name, unit, attrs, series)`. Histograms arrive
  as `<name>.avg` and `<name>.count`. Series helpers: `Flat`, `Step`, `Ramp`,
  `Periodic`, `Sawtooth`, `Plus`, `Scale`, `Clamp`. A gauge is a point
  sample every step (1 minute in the last day, 10 before), so a burst
  shorter than a step must be averaged into the series by the seed. `Flat`
  noise depends only on time: two hosts with the same bounds draw the same
  curve unless the seed shifts or offsets one.
- **Background traffic** (`s.Background`) names its routes in the service
  router's syntax (`:id` for Express and gin, `{id}` for net/http, chi,
  FastAPI, Spring and ASP.NET Core) and writes its queries with the driver's
  placeholders. Fixtures do not implement these routes.
- **Browser and mobile SDK data** goes through `p.Crash(seed.Crash{...})`, with
  network and navigation `Actions` and console `Logs`, and `p.Session(...)`.
  It travels the native `/api/report` protocol, and covers only what those
  SDKs really send: no frame timings, no mobile metrics.
- **Dashboard objects:**
  - `p.PagingChannel`, `p.EmailChannel`, `p.Rule(seed.Rule{...})` and
    `p.WaitPage(ruleID, timeout)`. A page should come from a rule that really
    fires.
  - `p.Check(...)` and `p.RunCheckUntil(id, "down", ...)` against
    `s.Sink.Stub()`, the switchable HTTP target.
  - `p.PostMortem`, `p.MarkSlow`, `p.Archive` and `p.GroupMatching` (read an
    exception group's hash back).
  - Put work that needs the ingested data in `s.AfterFlush`.
- **Volume.** Requests outside the detail sample are sent as their root span
  (and any downstream server spans), so percentiles and counts stay complete
  while a scenario stays under about 20k spans. Requests with logs, errors or
  an `Expose` always keep their full tree.

Backend facts that bite (each one cost a case author an afternoon):

- Successful `GET`/`HEAD` requests to health-check routes (`/health`,
  `/healthz`, `/ping`, `/ready`, `/status`, `/up`, ...; the list is in
  `backend/app/services/healthcheck.service.go`) are dropped at ingest, which
  is the projects' default. An endpoint seeded on one never appears and
  `Verify` fails with "endpoints never became visible"; a failing one (status
  400 or more) is kept.
- A log body search over more than 24 hours answers 422 unless another filter
  (service, severity, trace) narrows it.
- MCP results are JSON and escape `&`, `<` and `>`; grounding compares them
  unescaped, but a jq-free MCP agent sees the escaped form.
- A new monitor runs once on its own within a scheduler tick (5s here), so
  switch the stub only after that first run (see `wr_monitor.go`).
- Pages, monitors and incidents are stamped with real wall-clock time; seeds
  that need telemetry around a page lay it out near ingest time
  (`wr_timing.go`).
- Rare events confined to a short window can come out empty at some hours of
  the day (the arrival process follows the wall-clock hour). Use `Burst`, or
  seed the case at every hour of the day into `seed.NewDrySink()` and check
  the event is there before relying on it. The same goes for a `DetailPolicy`
  sample: overnight it can leave the last few hours with no detailed request,
  so a trace (or access log) an oracle step reaches through a log or list
  query must be forced by exposing it, `s.Expose(key, e.Pick(...))` or
  `e.Slowest(...)`, which keeps that request's detail.

- Numeric OTel attributes must be integers, which the DSL does for you.
- Projects with a browser framework never get endpoints from OTLP.
- An exception's message does not split groups: vary the type or the frames
  to get distinct issues.
- Session recordings are stamped with the ingest time, so keep crashes that
  carry actions within the last ~12 hours.
- The unscoped `metrics query --name system.cpu.utilization --aggregation max`
  returns the idle state. That is a real trap the skill should steer around.

### The case YAML

See `perf-db-n-plus-1.yaml` for the full shape. The keys:

- `arms` and `invoke`: `slash` preloads SKILL.md the way a `/traceway`
  command does, and `natural` makes the agent load the skill itself. A slash
  case's prompt starts with `/<skill>`, and only a slash case's does.
- `workspace.repo`: a fixture under `testdata/repos` or
  `testdata/setup-templates`. Fixtures can carry history: a `history.yaml`
  whose commits are overlays under `.history/` (only the files that differ
  from the fixture, plus an `absent:` list of files the commit lacks),
  committed with ages relative to the run so `git log` lines up with the
  seeded deploys.
- `prompt`, `budget`, `user`: scripted `ask_user` answers, first match wins.
- `credential` (`scope: read`, `member: readonly`) and `agent_access: false`
  for the policy cases.
- `expect.answer`, `expect.grounding`, `expect.calls`, `expect.state` (kinds
  in `internal/grade/state.go`), `expect.code`, `expect.report`.
- `judge.rubric` and `judge.expected`. Both are shown only to the judge.
- `hooks`: harness actions after a matching call, such as editing a
  post-mortem behind the agent's back (`edit_post_mortem`, `stub_healthy`,
  `stub_unhealthy`, `sleep`, `approve_setup_draft`). A hook that fails makes
  the run an `infra_error`, since the case did not happen as written.
- `setup`: the env given to the agent's container shell (`@token:<project>`,
  `@url`, `@port`), and the verify step.

Templates are resolved after seeding:

| Template | Value |
|---|---|
| `{{seed.<key>}}` | What the seed exposed |
| `{{env.url}}` | The instance URL |
| `{{env.project}}` | The primary project's id |
| `{{env.projects.<name>.id}}` | Another project's id |
| `{{env.stub}}` | The switchable stub's URL |
| `{{env.now}}` | The scenario's "now" |
| `{{env.setupToken}}` | A setup token (`tws_`), when `setup.token: setup` |

A scalar that is exactly one template keeps its type.

The `oracle` is the reference trajectory the fake model plays:

- **One step is one tool call**, so steps are cited as `c01`, `c02`, ... in
  the submitted evidence.
- **`shell:` steps** run CLI commands. On MCP arms a step's first `traceway`
  call becomes the matching MCP tool call. Use `jq` on CLI arms to keep cited
  outputs under the 30k-character cap: a truncated citation fails grounding.
  MCP results are capped at 100k.
- **`op:` + `args:` steps** are surface-neutral, with MCP parameter names. On
  CLI arms they render as a CLI command, adding `--yes` for writes.
- **`write_file: {from: oracle/<case>/<file>}`** writes whole files for code
  fixes, because YAML block scalars cannot start a line with a tab.
- **`{{last.<json.path>}}` and `{{calls.c03.<path>}}`** read earlier tool
  results at play time.
- **`load_skill:` steps** play as a harmless `list_files .` on arms without
  skills (`cli-noskill`, `mcp`), so call ids stay the same on every arm.
- **The oracle ends** with `submit:` (perf, debug) or `finish:` (write,
  setup).

While developing a case, `TWEVALS_ONLY=<ids>` limits loading to those cases,
so an unfinished YAML file elsewhere cannot break your run.

### Leakage

The seeded strings (span names, attribute values, log bodies, stack traces,
object names) and the prompt must not name the answer. `internal/grade/leakage.go`
holds the denylist, which covers phrases like `n+1`, `stampede`,
`missing index`, `pool exhaust`, `gc pause` and `noisy neighbor`. Use real
library messages, such as SQLAlchemy's `QueuePool limit of size 5 overflow 10
reached` or Postgres's `deadlock detected`, and neutral names.

## The catalog

68 cases, 178 case × arm combinations:

| Suite | Cases | What they cover |
|---|---|---|
| perf | 35 | All 26 rows of the bottleneck checklist; controls (inside a marked-slow allowance, past it, onset bisection outside the chart's top 5, an alert hand-off, a healthy endpoint, the metric "p95" trap); cross-surface cases where browser, Android and iOS crashes have a backend root cause |
| debug | 14 | Go, Python, Node, Android, iOS and Flutter issues reached by hash, pasted notification, dashboard URL or bug report; a mixed group, a log-only failure, a case where the right answer is more instrumentation, and a write-credential case that must stay read-only |
| write | 13 | Pages, alerts, monitors, archiving, post-mortems (incl. a concurrent-edit conflict), slow marks, and the 403 policy (opt-in off, read token, readonly member) |
| setup | 6 | Go net/http, Express, FastAPI, Next.js, a React SPA with the browser SDK, and a Go + React monorepo through the setup-token plan flow |

`twevals list --preset full` prints every combination.

## Layout

```
models.yaml presets.yaml baseline.json
cmd/twevals/           list | plan | run | resume | oracle | env up | report | compare | baseline update
internal/catalog/      the case schema, presets, models.yaml
internal/checklist/    the bottleneck rows of performance.md, the perf answer enum
internal/env/          backend subprocess, registration, members, PATs, CLI logins, the monitor stub
internal/seed/         the DSL, OTLP and /api/report writers, dashboard objects, read-back verification
internal/scenarios/    one seed function per case
internal/agent/        OpenRouter client, the loop, the cost ledger, prompt caching, transcripts, the fake model
internal/tools/        the sandboxed shell (mvdan.cc/sh, in-process coreutils, gojq, goawk), files, skills, MCP bridge, ask_user, submit
internal/ops/          CLI/MCP → operation mapping, cross-checked against tool-map.md
internal/grade/        answer, trajectory, grounding, state, hidden tests, judge, leakage
internal/record/       the run record: calls, operations, usage
internal/runner/       one run end to end; the setup container shell
internal/report/       results.json, summary.md, compare, cost estimates
internal/guard/        the manual-only policy and the workflow trigger test
internal/suite/        catalog-wide tests
cases/ testdata/       cases; fixture repos, hidden tests, oracle files, setup templates, the sandbox image
```

The agent's shell is an interpreter, never the host shell. It has the arm's
`traceway` binary (logged in with a PAT whose config lives outside the
workspace), `jq`, `awk`, read-only text utilities confined to the workspace,
and read-only `git`. No tool writes under a `.git` directory, git always
uses the workspace's repository, and git refuses arguments that name a file
outside the workspace or options that write files or run programs. Tool
subprocesses get a scrubbed environment (`HOME`, `XDG_*`, `PATH`, `TZ`,
`LANG`), so `OPENROUTER_API_KEY`, `GITHUB_TOKEN` and `JWT_SECRET` never reach
them. MCP arms connect to the backend's `/mcp` or `/mcp/write` with a PAT.
`project_id` defaults to the case's project, the way `traceway mcp` defaults
to the CLI's current project.

## Credits

The first performance scenarios were contributed by Liyu in PR #405
("feat(skills): add performance skill evals"). They were ported to this
module as `perf-db-n-plus-1`, `perf-db-missing-index`, `perf-net-sequential`,
`perf-db-pool-exhaustion`, `perf-app-sync-background`, `perf-net-large-payload`,
`perf-cache-no-cache`, `perf-infra-cpu-saturation`, `perf-cache-stampede`,
`perf-xs-ios-watchdog`, `perf-xs-android-timeouts`, `debug-android-npe` and
`debug-flutter-null-check`.
