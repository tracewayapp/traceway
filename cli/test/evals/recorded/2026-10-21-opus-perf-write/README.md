# Opus 5.5, perf + write suites, 2026-10-21

The Opus side of a Haiku vs Opus comparison, on the same selection as
`2026-10-21-haiku-perf-write` (branch `evals/haiku-perf-write-2026-10-21`).

- **Selection:** `--preset full --suites perf,write --models opus`, 1 repeat:
  131 runs (perf 35 cases × `cli+skill`/`cli-noskill`/`mcp`, write 13 cases ×
  `cli-write+skill`/`mcp-write`), every one graded. The debug and setup suites
  were left out: they need Docker, which the machine did not have.
- **Two invocations, one result.** The first pass (cap $75, at `49c92139`)
  graded 48 runs before the OpenRouter account ran out of credit; the other
  83 ended in `infra_error` (HTTP 402). After a top-up, `twevals resume` (cap
  $28, at `69ab7bb4`, which only adds the earlier partial record) re-ran
  exactly those 83 and merged them in. The harness, CLI, backend and skills
  were the same throughout.
- **Model settings:** the `opus` entry in `models.yaml` (reasoning effort
  medium, 16000 tokens per turn, Anthropic caching), the same as Haiku's run.
  Judge: Haiku 5.5, as always.
- **Where it ran:** a local `twevals` run (`-j 3`), not the CLI Evals
  workflow, so it is not a baseline.
- **Spend:** $39.90 in all. $37.94 agent and $0.13 judge for the graded runs,
  plus $1.83 on runs the 402 cut off (`replacedSpendUsd` in the header).

## Results

| Suite | Arm | Pass | Right answer and evidence (ignoring trajectory) | Haiku pass, same runs |
|---|---|---|---|---|
| perf | cli+skill | 18/35 (51%) | 29/35 (83%) | 13/35 |
| perf | cli-noskill | 19/35 (54%) | 27/35 (77%) | 18/35 |
| perf | mcp | 31/35 (89%) | 34/35 (97%) | 19/35 |
| write | cli-write+skill | 11/13 (85%) | 13/13 | 10/13 |
| write | mcp-write | 11/13 (85%) | 12/13 | 10/13 |

Overall, 90/131 pass (Haiku 70) and 115 have the right answer (Haiku 98), at
29× Haiku's agent cost ($37.94 vs $1.32) with similar token use.

- **Infrastructure cases:** 9/12 right answers against Haiku's 1/12.
- **The skill's `endpoints list` jq examples use fields that do not exist.**
  `cli/pkg/mcpserver/knowledge/performance.md:12` uses `p50, p95, p99,
  errorCount` and `notifications.md:38` uses `p50, p95, p99`, but
  `EndpointStats` has `p50Duration` and so on, in nanoseconds. The filters
  print nulls. With the skill, Opus copied them in 27/35 perf runs and Haiku
  in 33/35. Skill uplift on perf is -3pp for Opus and -14pp for Haiku.
- **Repeat-limit failures mostly come from guessing the JSON shape.** A blind
  `jq` filter returns nothing, the command is re-run with `head -c` to see the
  shape, then re-run again with the right filter. This explains 25 of Opus's
  26 re-fetches and 30 of Haiku's 33.
- **Grader artifacts:**
  - each iteration of one shell loop counts as an identical call;
  - different `git show`/`git log` commands share one op key;
  - grounding fails a cited call when only a trailing `jq` in it errored (5 of
    6 such failures);
  - `perf-db-missing-query-cache` fails on every arm because the answer names
    `GET /api/me (and every other route ...)` instead of exactly
    `GET /api/me`.
- **Malformed tool arguments:** in 5 of 107 diagnoses, the nested `locus`
  arrived as a string starting with tool-call XML. Three of them failed the
  answer grader on a right diagnosis. Haiku did this 0 times in 138.
- **Write:** `write-archive-ambiguous` archived without `ask_user` on both
  arms.

`summary.md` and `results.json` are the harness's own output. Transcripts were
not committed.
