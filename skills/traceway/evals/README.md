# Performance Skill Evals

Scenario-based evals for the `traceway` skill's **Performance** flow
(`/traceway perf <endpoint or symptom>`). Each eval hands an agent a
synthetic-but-realistic Traceway telemetry snapshot and asks it to diagnose
the bottleneck using the skill; the expected diagnosis is pinned to one row
of the bottleneck checklist in `../performance.md`.

## How to run an eval

1. Give the agent the `traceway` skill (or the `traceway` CLI against a
   staging instance seeded with the snapshot data).
2. Paste the **Task** prompt from the scenario file verbatim.
3. Grade the answer against **Expected diagnosis** and **Pass criteria**.
   A pass requires naming the right checklist suspect *and* citing the
   telemetry that confirms it — a correct guess with no evidence is a fail.

The snapshots are static text, so evals also work offline: the agent should
reason from the snapshot exactly as it would from live `traceway` output.

## Coverage

| ID | Area | Framework | Checklist suspect |
|---|---|---|---|
| `backend-go-select-n-plus-1` | backend | Go (Gin) | Select N+1 |
| `backend-python-pool-exhaustion` | backend | Python (FastAPI) | Connection-pool exhaustion |
| `backend-node-sequential-calls` | backend | Node.js (NestJS) | Sequential calls that could run in parallel |
| `backend-missing-index` | backend | any (PostgreSQL) | Missing or unused index |
| `backend-sync-background-task` | backend | Go (Gin) | Synchronous work that belongs in a background task |
| `frontend-js-large-payloads` | frontend | browser JS | Large request/response payloads |
| `frontend-js-no-cache` | frontend | browser JS | No cache / low hit ratio |
| `mobile-ios-cold-start` | mobile | iOS | Cold start (synchronous init on launch) |
| `mobile-android-gc-pauses` | mobile | Android | Memory pressure / GC pauses |
| `mobile-flutter-shader-jank` | mobile | Flutter | Cold start / JIT / shader warmup |
| `infra-cpu-saturation` | infrastructure | n/a | CPU saturation |
| `infra-cache-stampede` | infrastructure | n/a | Cache stampede / thundering herd |

## Adding a new eval

1. Copy the file format below: header table, `## Telemetry snapshot`,
   `## Task`, `## Expected diagnosis`, `## Pass criteria`.
2. Build the snapshot from a real incident when possible; keep numbers
   internally consistent (percentiles, span durations, and metric values
   must agree with each other).
3. Pin the expected diagnosis to exactly one `performance.md` checklist
   row, and make the snapshot contain the confirming signal that row
   asks for — plus one plausible distractor the agent must rule out.
4. Add a row to the coverage table above.
