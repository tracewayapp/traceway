# Opus 5.5, perf + write suites, 2026-10-21 (partial)

The Opus side of a Haiku vs Opus comparison, on the same selection as
`2026-10-21-haiku-perf-write` (branch `evals/haiku-perf-write-2026-10-21`).

- **Selection:** `--preset full --suites perf,write --models opus`, 1 repeat:
  131 runs (perf 35 cases × `cli+skill`/`cli-noskill`/`mcp`, write 13 cases ×
  `cli-write+skill`/`mcp-write`). The debug and setup suites were left out:
  they need Docker, which the machine did not have.
- **Partial: 48 of 131 runs graded.** 83 runs ended in `infra_error` with
  `openrouter HTTP 402` (`in_flight_budget_exhausted`): the OpenRouter account
  ran out of credit. The harness cap ($75) was not reached. Graded perf runs
  cover the first 14 cases alphabetically (`perf-app-*`, `perf-cache-*`,
  `perf-ctl-*`); no `perf-db-*`, `perf-infra-*`, `perf-net-*` or `perf-xs-*`
  case was graded. Write: 8 runs over 6 cases.
- **Model settings:** the `opus` entry in `models.yaml` at `49c92139`
  (reasoning effort medium, 16000 tokens per turn, Anthropic caching), the
  same as Haiku's run. Judge: Haiku 5.5, as always.
- **Where it ran:** a local `twevals run` (`-j 3`) at clean commit `49c92139`,
  not the CLI Evals workflow, so it is not a baseline.
- **Spend:** $15.75 ($15.70 agent, $0.05 judge); $13.87 on graded runs and
  $1.83 on runs cut off by the 402.

## Results (graded runs only)

| Suite | Arm | Pass | Right answer and evidence (ignoring trajectory) |
|---|---|---|---|
| perf | cli+skill | 10/14 (71%) | 11/14 (79%) |
| perf | cli-noskill | 7/13 (54%) | 11/13 (85%) |
| perf | mcp | 12/13 (92%) | 13/13 (100%) |
| write | cli-write+skill | 5/5 | 5/5 |
| write | mcp-write | 3/3 | 3/3 |

On the same 48 case × arm pairs, Haiku passed 21 (44%) and gave the right
answer on 36 (75%), against Opus's 37 (77%) and 43 (90%), at about 1/28 of
the cost ($0.49 vs $13.87).

- **Repeat thrash on `cli+skill` is mostly Haiku's:** 8 of Haiku's 14 runs on
  those pairs failed only `max_identical_repeats`, against 1 for Opus. Without
  the skill, Opus re-fetches `endpoints chart` to apply a different `jq` filter
  on 3 of 13 runs.
- **Trajectory grader artifacts:** a shell loop whose `date -d` failed counts
  every iteration as an identical call (7× and 15× on `cli-noskill`; Opus
  noticed the failure and redid the loop), and different `git show` commands
  share one op key (`git {"subcommand":"show"}`).
- **Grounding:** `perf-app-large-alloc` fails on both CLI arms because no
  cited output contains `G1 Old Gen`, as it did for Haiku on every arm. One
  run cites a call marked failed only because a trailing `jq` exited 5.
- **One malformed tool call:** on `perf-app-excessive-logging` (`cli+skill`),
  `submit_perf_diagnosis` arrived with tool-call XML inside `locus`
  (`"\n<parameter name=\"service\">listings-api"`), failing the answer grader
  on an otherwise right diagnosis.

`summary.md` and `results.json` are the harness's own output, including the 83
`infra_error` runs. Transcripts were not committed.
