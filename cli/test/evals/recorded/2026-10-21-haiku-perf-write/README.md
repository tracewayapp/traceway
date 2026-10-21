# Haiku 5.5, perf + write suites, 2026-10-21

A reference point for comparing `anthropic/claude-haiku-5.5` against
`anthropic/claude-opus-5.5`. Only Haiku was measured here.

- **Selection:** `--preset full --suites perf,write --models haiku`, 1 repeat:
  131 runs (perf 35 cases × `cli+skill`/`cli-noskill`/`mcp`, write 13 cases ×
  `cli-write+skill`/`mcp-write`). The debug and setup suites were left out:
  they need Docker, which the machine did not have.
- **Model settings:** the `haiku` entry in `models.yaml` as of `49c92139`, with
  the same reasoning effort (medium) and per-turn token budget (16000) as
  `opus`. Judge: Haiku 5.5, as always.
- **Where it ran:** a local `twevals run` (`-j 3`), not the CLI Evals
  workflow, so it is not a baseline. The results header records commit
  `f2432ec8` with uncommitted changes: that `models.yaml` edit, committed
  afterwards as `49c92139`.
- **Spend:** $1.46 of a $10 cap ($1.32 agent, $0.14 judge), 126 run-minutes.

## Results

| Suite | Arm | Pass | Right answer and evidence (ignoring trajectory) |
|---|---|---|---|
| perf | cli+skill | 13/35 (37%) | 26/35 (74%) |
| perf | cli-noskill | 18/35 (51%) | 25/35 (71%) |
| perf | mcp | 19/35 (54%) | 25/35 (71%) |
| write | cli-write+skill | 10/13 (77%) | 11/13 |
| write | mcp-write | 10/13 (77%) | 11/13 |

On perf `cli+skill`, 13 of the 22 failures are trajectory-only, all of them
`max_identical_repeats`. Haiku re-ran the same `endpoints list`/`chart` query
inside chained shell commands, with a different `jq` filter each time,
instead of reusing the output. The negative skill uplift (-14pp) comes from
that, not from worse diagnoses. The other arms fail mostly on the answer
itself.

Failed on every arm:

- wrong checklist row: `perf-infra-cpu-saturation`, `perf-infra-disk-io`,
  `perf-infra-memory-gc`, `perf-xs-ios-watchdog`;
- never opened a single trace or endpoint occurrence, or thrashed:
  `perf-db-n-plus-1`, `perf-net-sequential`, `perf-ctl-alert-handoff`,
  `perf-ctl-onset-bisect`;
- cited evidence lacks the deciding signal (`G1 Old Gen`): `perf-app-large-alloc`;
- report omits the 95%+ CPU figure: `perf-ctl-metric-p95-trap`;
- never ran the monitor to confirm the fix: `write-monitor-confirm-fix`.

`summary.md` and `results.json` are the harness's own output. Transcripts were
not committed.
