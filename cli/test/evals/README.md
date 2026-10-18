# Traceway skill and MCP evals

**Manual only.** Nothing here runs automatically, and nothing here calls a
model yet: this directory holds eval cases and their fixtures. The harness
that seeds a backend, drives an agent and grades the result is added next,
in its own Go module under this directory.

The cases started as the performance scenarios of PR #405
(`skills/traceway/evals/*.md`). They moved out of `skills/` because anything
in a skill directory ships to users and to the agent under test, which would
hand it the expected answers.

## What changed in the port

Each Markdown scenario became a structured case with one correct answer, one
named distractor, and telemetry that the real SDKs and the real ingest path
can produce:

| PR #405 file | Case | What was fixed |
|---|---|---|
| `backend-go-select-n-plus-1` | `perf-db-n-plus-1` | Consistent counts: N item queries for N orders |
| `backend-missing-index` | `perf-db-missing-index` | No `EXPLAIN` handed to the agent; a 30-day window that matches the claim |
| `backend-node-sequential-calls` | `perf-net-sequential` | No unexplained 700ms per hop |
| `backend-python-pool-exhaustion` | `perf-db-pool-exhaustion` | Pool-wait gaps that add up to the request time |
| `backend-sync-background-task` | `perf-app-sync-background` | Fix advice that does not depend on the retired `CaptureTask` |
| `frontend-js-large-payloads` | `perf-net-large-payload` | A real `http.response.body.size` attribute instead of a fictional `transfer` span |
| `frontend-js-no-cache` | `perf-cache-no-cache` | A browser project is present, and only one checklist row is correct |
| `infra-cpu-saturation` | `perf-infra-cpu-saturation` | Saturation driven by traffic, not by a deploy |
| `infra-cache-stampede` | `perf-cache-stampede` | Realistic request rates |
| `mobile-ios-cold-start` | `perf-xs-ios-watchdog` | Built only from telemetry the iOS SDK sends: a launch watchdog crash whose cause is a slow backend endpoint |
| `mobile-android-gc-pauses` | `perf-xs-android-timeouts`, `debug-android-npe` | The Android SDK sends crashes, actions and logs, not frame or GC metrics |
| `mobile-flutter-shader-jank` | `debug-flutter-null-check` | The Flutter SDK sends errors, not frame timings |

## Layout

```
cases/perf/<id>.yaml     11 performance cases
cases/debug/<id>.yaml    2 mobile debugging cases
testdata/repos/<repo>/   the fixture repository each case's agent works in
testdata/oracle/<case>/  files the reference trajectory writes (debug fixes)
```

A case names its seed (the Go function that will generate its telemetry),
its arms (`cli+skill`, `cli-noskill`), the prompt, budgets, the expected
answer, the tool calls that must, may and must not happen, a judge rubric,
and an `oracle`: the reference trajectory, one tool call per step, ending in
the structured answer.

A fixture repo can carry git history: `history.yaml` lists commits, each an
overlay under `.history/` (only the files that differ from the fixture, plus
an `absent:` list of files the commit lacks), with ages relative to the run so
`git log` lines up with the seeded deploys.

## Credits

The first performance scenarios were contributed by Liyu in PR #405
("feat(skills): add performance skill evals").
