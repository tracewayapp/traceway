# Plan: Root vs Non-Root Trace Classification & Cross-Type Linkage

## Context

The backend tracks three "special" top-level trace types — **endpoints**, **tasks**, and **ai_traces** — alongside generic **spans**. Today these special tables are only populated when the underlying OTel span is the trace root (`len(span.ParentSpanId) == 0`); every non-root span lands in `spans` only. That breaks for AI traces: an LLM call inside an HTTP handler is a child of an endpoint root, so it currently shows up as a generic span and is invisible in the AI traces list.

There is also a latent semantic bug we have to fix as part of this work. For all three special tables today, `<table>.id` is set to the **trace ID**, not the row's own primary key (see `trace_converter.go` calling `buildEndpoint(traceId, …)` → `Endpoint.Id = traceId`). The detail controllers exploit this collision: `endpoint_detail.controller.go` reuses the URL `endpointId` as both `EndpointRepository.FindById` arg AND `SpanRepository.FindByTraceId` arg. The moment we dual-write, multiple rows per trace would share the same `id`, the MergeTree doesn't enforce uniqueness, and `FindById` becomes non-deterministic.

We fix this by adopting the convention already used in `exception_stack_traces`: **`id` is the row's own PK (= OTel span ID), `trace_id` is a separate column**. Existing rows are backfilled with `trace_id = id` so historical data still groups correctly. URLs continue to use the row PK; detail controllers gain one extra dereference to find the trace ID before fetching the waterfall.

We need to:

1. Add `trace_id` and `parent_span_id` columns to `endpoints`, `tasks`, and `ai_traces`, backfilled so existing rows still behave as roots of their own trace.
2. Make `Id = spanId` for all special-table rows (root and non-root). `TraceId = otelTraceIDToUUID(...)`.
3. Capture every HTTP/consumer/gen_ai span as a row in its matching special table, regardless of root status (root status is derived from `parent_span_id IS NULL`).
4. For non-root special spans, **dual-write**: keep the regular `spans` row (so the waterfall stays complete) AND also create the special-table row (so the AI traces / endpoints / tasks lists are complete).
5. Surface root-status as a filter on the three list pages, defaulting to "All" on all three.
6. Visually mark non-root rows with a **"Not Root" chip** on the list pages and on the detail-page headers. Most users will never see this chip (their traces are all roots) — it appears only when the entity actually has non-root entries, so the rare case is unambiguous at a glance.
7. Enable cross-type navigation: from a span row jump to its AI trace (or endpoint/task) when one exists, and from a non-root special detail jump back to the nearest special ancestor that started it.
8. Attribute exception events on non-root spans to the **kind of span they fired on** (`endpoint` / `task` / `ai_trace`), not the existing default `"task"` mislabel. This requires `trace_type` to gain a new valid value `"ai_trace"`.

The change is symmetric across the three special tables — AI is just the most-cited example.

---

## ID strategy (the load-bearing piece)

| Field | Before | After |
|-------|--------|-------|
| `endpoints.id` / `tasks.id` / `ai_traces.id` | OTel trace ID (encoded as UUID) | OTel **span** ID (encoded as UUID) — unique per row |
| `endpoints.trace_id` / `tasks.trace_id` / `ai_traces.trace_id` | (didn't exist) | OTel trace ID — many rows per trace can share this |
| `endpoints.span_id` / `tasks.span_id` | OTel span ID (already there from 0047, 0048; today identical to `id` for roots) | Same as `id` (kept for now; redundant after the refactor — flagged in the session log as a follow-up cleanup) |

**Backfill plan.** For ClickHouse, every new `trace_id` column uses `DEFAULT id` so existing rows materialize the right value lazily without a mutation. For SQLite, a follow-up `UPDATE … SET trace_id = id WHERE trace_id IS NULL` runs in the same migration (the SQLite runner — `backend/app/migrations/migrations_sqlite.go:80` — splits on `;`, so multi-statement files are supported).

**URL impact.** `/endpoints/<endpoint>/<endpointId>`, `/tasks/<task>/<taskId>`, `/ai-traces/<traceName>/<traceId>` paths keep their `id` semantics: the value is whatever the row's PK is. Existing bookmarks for root rows resolve correctly because the `DEFAULT id` backfill leaves their `id` unchanged and there is exactly one row per trace today. New links generated post-deploy point at the row's actual PK (= span ID).

**Detail controller dereference.** Today `endpoint_detail.controller.go:67-79` does `SpanRepository.FindByTraceId(c, projectId, endpointId)` and `ExceptionStackTraceRepository.FindAllByTraceId(c, projectId, endpointId)`, reusing `endpointId` as a trace ID. After this change, those two calls become `SpanRepository.FindByTraceId(c, projectId, endpoint.TraceId)` and `ExceptionStackTraceRepository.FindAllByTraceId(c, projectId, endpoint.TraceId)`. Same shape for the task and AI-trace detail controllers.

---

## Schema changes

### ClickHouse — `backend/app/migrations/ch/` (one statement per file)

- `0058_add_trace_id_to_endpoints.up.sql` — `ALTER TABLE endpoints ADD COLUMN trace_id UUID DEFAULT id`
- `0059_add_trace_id_to_tasks.up.sql` — `ALTER TABLE tasks ADD COLUMN trace_id UUID DEFAULT id`
- `0060_add_trace_id_to_ai_traces.up.sql` — `ALTER TABLE ai_traces ADD COLUMN trace_id UUID DEFAULT id`
- `0061_add_parent_span_id_to_endpoints.up.sql` — `ALTER TABLE endpoints ADD COLUMN parent_span_id Nullable(UUID)`
- `0062_add_parent_span_id_to_tasks.up.sql` — same for `tasks`
- `0063_add_parent_span_id_to_ai_traces.up.sql` — same for `ai_traces`
- `0064_add_span_id_to_ai_traces.up.sql` — `ALTER TABLE ai_traces ADD COLUMN span_id Nullable(UUID)` (endpoints/tasks already got this in 0047/0048)
- `0065_add_distributed_trace_id_to_ai_traces.up.sql` — `ALTER TABLE ai_traces ADD COLUMN distributed_trace_id Nullable(UUID)`
- `0066_add_span_id_index_ai_traces.up.sql` — `ALTER TABLE ai_traces ADD INDEX idx_span_id span_id TYPE bloom_filter GRANULARITY 4` (needed for the spans→ai_trace lookup; endpoints/tasks already have `idx_id` per 0002/0003)

No new column for `is_root` — derived from `parent_span_id IS NULL`. Matches the existing convention (`distributed_trace_id` is also nullable, not boolean-flagged).

### SQLite — `backend/app/migrations/sqlite_telemetry/`

- `0009_endpoints_root_columns.up.sql`:
  ```sql
  ALTER TABLE endpoints ADD COLUMN trace_id TEXT DEFAULT NULL;
  ALTER TABLE endpoints ADD COLUMN parent_span_id TEXT DEFAULT NULL;
  UPDATE endpoints SET trace_id = id WHERE trace_id IS NULL;
  ```
- `0010_tasks_root_columns.up.sql` — same for `tasks`
- `0011_ai_traces_root_columns.up.sql`:
  ```sql
  ALTER TABLE ai_traces ADD COLUMN trace_id TEXT DEFAULT NULL;
  ALTER TABLE ai_traces ADD COLUMN span_id TEXT DEFAULT NULL;
  ALTER TABLE ai_traces ADD COLUMN parent_span_id TEXT DEFAULT NULL;
  ALTER TABLE ai_traces ADD COLUMN distributed_trace_id TEXT DEFAULT NULL;
  UPDATE ai_traces SET trace_id = id WHERE trace_id IS NULL;
  ```

---

## Model changes

- `backend/app/models/endpoint.model.go` — add `TraceId uuid.UUID \`json:"traceId" ch:"trace_id"\`` and `ParentSpanId *uuid.UUID \`json:"parentSpanId,omitempty" ch:"parent_span_id"\``.
- `backend/app/models/task.model.go` — same.
- `backend/app/models/ai_trace.model.go` — add `TraceId uuid.UUID`, `SpanId *uuid.UUID`, `ParentSpanId *uuid.UUID`, `DistributedTraceId *uuid.UUID` (all with appropriate `omitempty` where nullable).
- `backend/app/models/exception_stack_trace.model.go` — update the doc comment on `TraceType` (line 13): the valid values are now `"endpoint"`, `"task"`, or `"ai_trace"`.
- Stats types: add `NonRootCount uint64 \`json:"nonRootCount"\`` to `EndpointStats`, `TaskStats`, and `AiTraceStats` (same file as their parent model). Drives the "Not Root" chip on the list pages.

For SQLite, the row wrapper types (`endpointRow`, `taskRow`, AI-trace equivalent) in `backend/app/repositories/sqlite_types.go` and the `_sqlite.go` repositories need matching fields.

---

## Ingestion changes — `backend/app/controllers/otelcontrollers/trace_converter.go`

Today: lines 113–161 branch on `isRoot`. Root → one of endpoint/task/ai_trace (with `Id = traceId`). Non-root → `spans` only.

Change to **classify every span by kind first**, then write with `Id = spanId` always:

```go
parentSpanId := ptrSpanUUID(span.ParentSpanId)  // nil for root
kind := classifyKind(span, spanAttrs)           // "endpoint" | "task" | "ai_trace" | "other"

switch kind {
case "endpoint":
    ep := buildEndpoint(spanId, traceId, projectId, span, spanAttrs, allAttrs, startTime, duration, serverName, appVersion)
    ep.DistributedTraceId = distributedTraceId
    ep.SpanId = &spanId
    ep.ParentSpanId = parentSpanId
    endpoints = append(endpoints, ep)
case "task":
    t := buildTask(spanId, traceId, projectId, span, allAttrs, startTime, endTime, duration, serverName, appVersion)
    t.DistributedTraceId = distributedTraceId
    t.SpanId = &spanId
    t.ParentSpanId = parentSpanId
    tasks = append(tasks, t)
case "ai_trace":
    ai := buildAiTrace(spanId, traceId, projectId, span, spanAttrs, allAttrs, startTime, duration, serverName, appVersion)
    ai.DistributedTraceId = distributedTraceId
    ai.SpanId = &spanId
    ai.ParentSpanId = parentSpanId
    aiTraces = append(aiTraces, ai)
    if conv := extractConversation(spanAttrs, projectId, spanId); conv != nil {
        aiConversations = append(aiConversations, *conv)
    }
}

// Dual-write the generic span row for non-roots so the waterfall is complete.
// (Root rows aren't part of any waterfall and don't need a span row.)
if !isRoot {
    spans = append(spans, models.Span{
        Id: spanId, TraceId: traceId, ProjectId: projectId,
        Name: spanName, StartTime: startTime, Duration: duration,
        RecordedAt: startTime, ParentSpanId: parentSpanId,
    })
}
```

Notes:
- The `build*` helpers' signature gains a new second arg: `buildEndpoint(id, traceId, projectId, …)`, `buildTask(id, traceId, projectId, …)`, `buildAiTrace(id, traceId, projectId, …)`. Inside each helper, the returned struct sets `Id: id` and `TraceId: traceId`. The caller passes `spanId` as `id` and the OTel trace UUID as `traceId`. Fields not set by the helper (`SpanId`, `ParentSpanId`, `DistributedTraceId`) are still assigned by the caller after the call, as shown in the snippet above.
- `buildAiTrace` also takes `DistributedTraceId` from the `traceway.distributed_trace_id` attribute (already extracted in lines 107–111 for the other kinds).
- Storage keys for AI conversations need updating. Today, `buildAiTrace` (line 402) and `extractConversation` (line 507) each independently build `ai-traces/<projectId>/<traceId>.json`, and they agree because both use the trace ID. After this change, both must use the row PK (= spanId) so that (a) each AI row has its own file and (b) the two formulas still agree. Concretely:
  - Inside `buildAiTrace`, the existing `storageKey := fmt.Sprintf("ai-traces/%s/%s.json", projectId, id)` is already correct *by construction* — once the wider refactor passes spanId as `id`, the formula resolves to the new path. No code change needed inside the function.
  - `extractConversation` builds the same key independently. Pass `spanId` as its third arg (currently named `traceId`) and rename the parameter to `id uuid.UUID` (or `aiTraceId`) so the variable name doesn't lie.
  - **No backfill of existing `storage_key` values.** The read path (`ai_trace.controller.go:153`) reads the column verbatim and calls `storage.Store.Read(aiTrace.StorageKey)` — it never recomputes the key. Historical rows keep their `ai-traces/<projectId>/<oldTraceId>.json` value in the column, the file still exists at that path, reads continue to work. Pre-deploy state has at most one AI row per OTel trace (roots only), so no historical collisions exist either.
- `classifyKind`: HTTP server/internal with HTTP attrs → `"endpoint"`; `SPAN_KIND_CONSUMER` → `"task"`; `hasGenAiAttributes` → `"ai_trace"`; otherwise `"other"`. Preserve the existing precedence (endpoint > task > ai_trace) for the unlikely case of an attribute set that satisfies more than one — comment this explicitly.

### Exception attribution

Replace the current `traceType := "task"` default (line 163) and the `isRoot && SERVER/INTERNAL && hasHTTPAttributes` override with:

```go
var traceType string
switch kind {
case "endpoint": traceType = "endpoint"
case "task":     traceType = "task"
case "ai_trace": traceType = "ai_trace"
default:         traceType = "task"  // preserved fallback for generic spans
}
```

The `traceId` passed to `buildException` stays the OTel trace ID — exceptions still hang off the trace, not the row.

### Downstream `trace_type = "ai_trace"` audit

The new value needs handling in:
- `backend/app/controllers/distributed_trace.controller.go:115-142` — currently branches on `endpoint` / `task` / `exception`. Add an `ai_trace` branch that fetches from `AiTraceRepository` and emits the appropriate `TraceType: "ai_trace"` entry.
- Any Issues page filter/grouping that enumerates trace types — grep for `"endpoint"` / `"task"` literals together as the canonical pair and add `"ai_trace"` alongside.
- `models/exception_stack_trace.model.go:13` doc comment.

---

## Repository changes

### Inserts — add the new columns
- `backend/app/repositories/endpoint.repository.go` + `_sqlite.go`: add `trace_id` and `parent_span_id` to the INSERT column list and `batch.Append` arg list. Pre-existing gap: `FindAll`/`FindByEndpoint` SELECT lists don't currently include `span_id` (column exists since 0047). Widen those SELECTs to include `trace_id`, `span_id`, and `parent_span_id` so detail pages can rely on them.
- `backend/app/repositories/task.repository.go` + `_sqlite.go`: same.
- `backend/app/repositories/ai_trace.repository.go` + `_sqlite.go`: add `trace_id`, `span_id`, `parent_span_id`, `distributed_trace_id`.

### `is_root` filter on grouped queries
- `endpointRepository.FindGroupedByEndpoint` / `taskRepository.FindGroupedByTaskName` / `aiTraceRepository.FindGroupedByTraceName`: add an `isRoot *bool` argument. When non-nil, append `AND parent_span_id IS NULL` (true) or `AND parent_span_id IS NOT NULL` (false). For `FindGroupedByEndpoint` specifically, this clause must be added to **both** `whereClause` (used by the unique-endpoint count) **and** `joinWhereClause` with the `e.` prefix (used by the LEFT JOIN — see `endpoint.repository.go:97-103`). Three-state at the API surface: omitted = no filter.

### Non-root count for the chip
Independently of the filter, the **grouped query SELECT** for all three tables gains an aggregate:
```sql
countIf(parent_span_id IS NOT NULL) AS non_root_count
```
This count is per-group (per endpoint name / task name / trace name) and is unaffected by the `isRoot` filter — the frontend uses it to decide whether to show the "Not Root" chip. Surface it on the response types:
- `EndpointStats.NonRootCount uint64 \`json:"nonRootCount"\``
- `TaskStats.NonRootCount uint64 \`json:"nonRootCount"\``
- `AiTraceStats.NonRootCount uint64 \`json:"nonRootCount"\``

For the **detail** endpoints, no aggregate is needed: the row already exposes `parentSpanId`, and the frontend treats `parentSpanId != null` as "this entity is non-root" → show chip in the header.

### Span → AI-trace link lookup
Add `aiTraceRepository.FindBySpanIds(ctx, projectId, spanIds []uuid.UUID) (map[uuid.UUID]AiTraceRef, error)` where:
```go
type AiTraceRef struct {
    Id        uuid.UUID
    TraceName string  // needed because the detail route is /ai-traces/<traceName>/<traceId>
}
```
The endpoint/task detail controllers fetch spans via `SpanRepository.FindByTraceId(row.TraceId)` and then call this to enrich the response.

### Parent lookup helper (`FindParentRef`) — walk-up logic
Add `repositories.FindParentRef(ctx, projectId, parentSpanId uuid.UUID)` returning `*ParentRef` (or nil if no special ancestor exists):

```go
type ParentRef struct {
    Kind    string    // "endpoint" | "task" | "ai_trace"
    Id      uuid.UUID // the row PK (= the span's UUID)
    Name    string    // endpoint route, task_name, or trace_name — used by the frontend to build the URL
    TraceId uuid.UUID // exposed in case the UI wants to display the parent trace separately
}
```

Algorithm:
1. Probe `endpoints`, `tasks`, `ai_traces` for a row with `id = parentSpanId`. If hit, return it.
2. Otherwise probe `spans` for `id = parentSpanId`. If the span exists and has its own `parent_span_id`, recurse with `parent_span_id` as the new `parentSpanId`.
3. Cap depth at 10 to avoid runaway in a cycle (defensive — shouldn't happen with valid OTel data).
4. If the chain ends without a special ancestor, return nil.

This means a chain like `endpoint(root) → generic span → gen_ai child` produces a `ParentRef{Kind: "endpoint", …}` when called from the AI-trace detail page, not a dead-end "Parent span: …" label.

### Detail controller dereference
- `endpoint_detail.controller.go:67-79`: `SpanRepository.FindByTraceId(c, projectId, endpointId)` becomes `SpanRepository.FindByTraceId(c, projectId, endpoint.TraceId)`. Same for `ExceptionStackTraceRepository.FindAllByTraceId(...)`.
- `task_detail.controller.go`: same pattern (look up `task` first, then use `task.TraceId`).
- `ai_trace.controller.go GetAiTraceDetail` (around routes.go:102): if it currently reuses `traceId` for span/exception lookups, switch to `aiTrace.TraceId`.

---

## Controller changes

- `backend/app/controllers/endpoint.controller.go`, `task.controller.go`, `ai_trace.controller.go` (`/grouped` endpoints): parse `isRoot *bool` from the request body and pass it through to the repository. Body key: `"isRoot"` (matching existing camelCase). Three-state: missing / true / false.
- Detail endpoints (`/endpoints/:endpointId`, `/tasks/:taskId`, `/ai-traces/:traceId`): include `parentSpanId` and `parentRef` (from `FindParentRef`) in the response so the UI can render a "View parent" link.
- Endpoint/task detail spans response (returned alongside the waterfall): include `linkedAiTraceId *string` and `linkedAiTraceName *string` per span, populated by `aiTraceRepository.FindBySpanIds`.

---

## Frontend changes

### List-page `is_root` filter (3-state select)
- `frontend/src/routes/endpoints/+page.svelte`, `tasks/+page.svelte`, `ai-traces/+page.svelte`:
  - Add `let rootFilter = $state<'all'|'root'|'nonroot'>(...)` initialized from URL param `root`. **Default: `'all'` on all three pages** (post-deploy, Endpoints/Tasks may briefly surface previously-hidden non-root rows; that's the intended behavior).
  - Reuse the existing `<Select>` shadcn component (same pattern as the time-range preset on `endpoints/+page.svelte`). Label "Roots:" with options `All` / `Root only` / `Non-root only`.
  - When the value changes: update URL (`updateUrl({ root })`), reset page to 1, reload.
  - In the request body, map `'all'` → omit, `'root'` → `isRoot: true`, `'nonroot'` → `isRoot: false`.

### Type updates
- `frontend/src/lib/types/spans.ts` (or wherever the `Span` type lives): add `linkedAiTraceId?: string`, `linkedAiTraceName?: string`.
- Add types for `parentRef` shape on the three detail-page response types.

### Span → AI trace link
- `frontend/src/lib/components/spans/span-row.svelte`: when the span has `linkedAiTraceId` and `linkedAiTraceName`, render a small icon link to `/ai-traces/<encodeURIComponent(linkedAiTraceName)>/<linkedAiTraceId>` next to the span name. Pattern follows existing row icon links — no new component needed.

### Child → parent link
- AI trace detail page, plus the endpoint and task detail pages where non-root rows can now appear: show a "Parent: <kind> <name>" link near the header when `parentRef` is present. Link target depends on `parentRef.kind`:
  - `endpoint` → `/endpoints/<parentRef.name>/<parentRef.id>`
  - `task` → `/tasks/<parentRef.name>/<parentRef.id>`
  - `ai_trace` → `/ai-traces/<parentRef.name>/<parentRef.id>`
  - (`FindParentRef` never returns `kind: "span"` thanks to walk-up — so the UI has no dead-end case to handle.)

### "Not Root" chip
Use the existing shadcn-svelte `Badge` component (no new component). Two render sites:

**List pages** (`endpoints/+page.svelte`, `tasks/+page.svelte`, `ai-traces/+page.svelte`): in the row's name column, render the badge **only when `row.nonRootCount > 0`**. Examples:
- Group with `count=12, nonRootCount=0` → no chip (the common case — 99% of rows).
- Group with `count=12, nonRootCount=3` → chip shown (mixed group; some non-root entries to investigate).
- Group with `count=3, nonRootCount=3` → chip shown (entire group is non-root).

**Detail pages** (`endpoints/[endpoint]/[endpointId]`, `tasks/[task]/[taskId]`, `ai-traces/[traceName]/[traceId]`): render the badge in the header **only when the entity's `parentSpanId != null`** (i.e., this specific row is non-root).

Badge spec:
- Text: `Not Root`
- Variant: `secondary` (or `outline` — design call; pick whichever has the least visual weight while still being readable, since the chip is informational, not actionable).
- Tooltip on hover: "This entry was captured as a child span of another endpoint/task/AI trace." — so a user encountering it for the first time understands what it means without leaving the page.

---

## Critical files to modify

| File | Purpose |
|------|---------|
| `backend/app/migrations/ch/0058..0066_*.up.sql` | New ClickHouse columns + index (9 files) |
| `backend/app/migrations/sqlite_telemetry/0009..0011_*.up.sql` | SQLite mirror (3 files, multi-stmt) |
| `backend/app/models/{endpoint,task,ai_trace,exception_stack_trace}.model.go` | New fields; doc-comment update for TraceType |
| `backend/app/repositories/sqlite_types.go` | Row wrappers for SQLite |
| `backend/app/controllers/otelcontrollers/trace_converter.go` | Dual-write + Id=spanId refactor + parent_span_id + trace_type for ai_trace |
| `backend/app/repositories/{endpoint,task,ai_trace,span}.repository{,_sqlite}.go` | Inserts, SELECT widening, isRoot filter, `FindBySpanIds`, `FindParentRef` |
| `backend/app/controllers/{endpoint,task,ai_trace,endpoint_detail,task_detail}.controller.go` | `isRoot` param + detail dereference + parent enrichment |
| `backend/app/controllers/distributed_trace.controller.go` | Add `ai_trace` branch (lines 115-142) |
| `frontend/src/routes/{endpoints,tasks,ai-traces}/+page.svelte` | Filter UI + payload |
| `frontend/src/routes/{endpoints/[endpoint]/[endpointId],tasks/[task]/[taskId],ai-traces/[traceName]/[traceId]}/+page.svelte` | Parent link |
| `frontend/src/lib/components/spans/span-row.svelte` | AI-trace link icon |
| `frontend/src/lib/types/spans.ts` | New optional fields |
| `frontend/src/lib/utils/url-params.ts` | Add `root` to the sticky-params list if applicable |

---

## Things to reuse, not reinvent

- `otelSpanIDToUUID` and `ptrSpanUUID` in `trace_converter.go` — already give us the span/parent UUIDs.
- `parent_span_id IS NULL` derivation — matches how the existing converter already treats root status (line 98).
- `exception_stack_traces` schema convention — `id` is the row PK and `trace_id` is a separate column. We're adopting the same shape for endpoints/tasks/ai_traces.
- `Select` shadcn component used for the time-range preset on `endpoints/+page.svelte`.
- `getSortState` / `updateUrl` in `frontend/src/lib/utils/` for URL/localStorage persistence.
- `aiTraceRepository.FindGroupedByTraceName` parameter shape — the new `isRoot` param threads through naturally next to `search`.
- `idx_id` bloom_filter already exists on endpoints (`0002`), tasks (`0003`), and ai_traces (`0044`) — `FindById(spanId)` after the refactor still hits these indexes.

---

## Verification

1. **Backend unit/integration**
   - Add a test in `trace_converter_test.go` (create one if missing) feeding a synthetic OTLP payload with an HTTP root span containing a gen_ai child span. Assert:
     - 1 endpoint row: `id == spanId(root)`, `trace_id == otelTraceId`, `parent_span_id IS NULL`.
     - 1 ai_trace row: `id == spanId(child)`, `trace_id == otelTraceId`, `parent_span_id == spanId(root)`.
     - 1 span row for the child (dual-write).
     - No span row for the root (single-write).
     - Exception event on the gen_ai child gets `trace_type = "ai_trace"`.
   - Second test: a multi-step agent with HTTP root + N gen_ai children. Assert N ai_trace rows with distinct `id` values.
   - Run `cd backend && go test ./...`.

2. **Migrations & backfill**
   - Boot the backend against ClickHouse mode (default) and SQLite mode (`DB_TYPE=sqlite`) on a database populated with pre-change rows.
   - For each special table, verify `SELECT count() FROM <table> WHERE trace_id != id` returns 0 — i.e., existing rows backfilled `trace_id = id`.
   - Hit the detail URL for one historical endpoint and confirm the spans waterfall still resolves (the controller now uses `row.TraceId` which equals the old `id`).

3. **End-to-end with the Go SDK / OTel sample**
   - Run `cd backend && go run .`
   - Point an OTel client at it and emit a trace: HTTP root → gen_ai child. Then a standalone gen_ai root.
   - Hit `/api/ai-traces/grouped` with `isRoot: true`, `isRoot: false`, omitted; verify counts.
   - Hit `/api/endpoints/grouped` with the same three states.
   - Hit `/api/distributed-trace/<traceId>` and confirm both endpoint and ai_trace entries appear (regression check on the `distributed_trace.controller.go` audit).

4. **Frontend**
   - `cd frontend && npm run dev`, open `/ai-traces`, `/endpoints`, `/tasks`.
   - Default filter shows `All` and URL has no `root` param. Toggle the three states; confirm URL param `root` updates, page resets to 1, results match expectations.
   - Open an endpoint detail with a gen_ai child span in its waterfall. Confirm the span row shows the "View AI trace" icon and that clicking it navigates correctly (URL has both `<traceName>` and `<traceId>`).
   - Open the AI trace detail for the child gen_ai trace. Confirm it shows "Parent: endpoint <name>" and the link returns to the endpoint.
   - Construct a span-only intermediate parent (HTTP root → generic span → gen_ai child) and confirm the parent link walks up to the HTTP root, not the generic span.
   - **Not Root chip**: confirm rows with `nonRootCount == 0` show no chip; rows with `nonRootCount > 0` show "Not Root"; non-root detail pages show the chip in the header. Hover the chip and confirm the tooltip explains the meaning.
   - Run `npm run check` and ignore the pre-existing svelte-check noise in `session-replay.svelte` / `turnstile-widget.svelte` (verified by running on `main`).

5. **Regression sanity**
   - Pre-change traces still render as roots everywhere (`parent_span_id IS NULL` derived from backfilled rows).
   - Open a historical AI trace detail page and confirm the conversation body still loads — the row's pre-existing `storage_key` column points at the file written under the old key scheme and the read path reads the column verbatim.
   - Issues page still shows endpoint-attributed and task-attributed exceptions; any new ai_trace-attributed exceptions appear with the right grouping.
   - `/exception-stack-traces/by-id/:exceptionId` continues to work — the exception's `trace_id` is still the OTel trace ID, not the row PK.

---

## Open follow-ups (not blocking this work)

- `endpoints.span_id` / `tasks.span_id` are now redundant with `id`. Remove in a later migration once we're confident nothing reads them.
- `ai_traces` is partitioned daily (`toYYYYMMDD(recorded_at)`) while CLAUDE.md says monthly — pre-existing discrepancy, worth correcting separately.
- Pre-existing exception traceType mislabel: today every non-root generic span's exception is recorded as `trace_type = "task"`. After this change, the only spans landing in `spans` are non-special-kind ones, and they keep that `"task"` default. Consider a fourth value `"other"` later if it matters.

---

# Phase 2: Span ownership via `entity_id` (subtree-correct waterfalls)

## Why this is needed

Phase 1 above made `trace_id` a separate column and dual-wrote non-root special-table spans. That correctly populates the lists but introduced a **subtree leak** on the detail pages:

`endpoint_detail.controller.go:82` and `task_detail.controller.go:69` both call `SpanRepository.FindByTraceId(c, projectId, row.TraceId)`. But `trace_id` is the **global** W3C trace identifier — a single trace can now legitimately contain multiple entity rows (e.g. a Laravel HTTP request that dispatches a queued job: one `endpoints` row + one `tasks` row sharing the same `trace_id`, see `routes/web.php:87` `test/traceway/dispatch-job` in the Zentigo repro).

Worked example (`GET /test/traceway/dispatch-job`):

```
trace_id = T1
├── endpoint row "GET /test/traceway/dispatch-job"  (span_id = E, parent = NULL)   ← endpoints table
│   └── producer span                                (span_id = P, parent = E)      ← spans table
└── task row "TestTracewayJob"                       (span_id = C, parent = P)      ← tasks table + spans (dual-write)
    └── DB query inside handle()                     (span_id = Q, parent = C)      ← spans table
```

Today `FindByTraceId(T1)` returns `{P, C, Q}` for both detail pages — the endpoint shows the consumer's internal DB query in its waterfall, and the task shows the request's producer span. Each page should show only its own subtree:

- Endpoint detail should return `{P}`.
- Task detail should return `{C, Q}` (the dual-written consumer span + its descendants).

## The fix: denormalize the owning entity onto each span

Add an `entity_id` column to the `spans` table. At ingest, for each generic span, resolve its **nearest endpoint/task/ai_trace ancestor (including itself if the span lands in a special table)** and store that span_id. Then the detail pages query `WHERE entity_id = <row.id>` instead of `WHERE trace_id = <row.trace_id>`.

Why this is the right shape:

- **A span belongs to exactly one entity.** "Nearest special-table ancestor walking up `parent_span_id`" is unambiguous.
- **The consumer span owns its own subtree.** A CONSUMER span lands in the `tasks` table AND is dual-written to `spans`. Its `entity_id` is itself (its own `span_id`), so loading `WHERE entity_id = C` returns the consumer span row plus all its descendants — a complete waterfall rooted at the task.
- **`trace_id` keeps its W3C meaning unchanged.** Cross-service log/trace correlation, OTel propagation, exception-grouping by trace — none of this changes. We are not overloading `distributed_trace_id` to mean "top-level entity" either; those concepts stay separate.
- **Reads stay flat.** No recursive CTE, no per-row walk. One indexed equality lookup.

## ID strategy (extending Phase 1)

| Field on `spans` | Before Phase 2 | After Phase 2 |
|------------------|----------------|----------------|
| `trace_id` | W3C trace_id (global) | Unchanged — W3C trace_id |
| `parent_span_id` | Immediate parent OTel span_id | Unchanged |
| `entity_id` | (didn't exist) | **NEW** — nearest endpoint/task/ai_trace ancestor's span_id (= that row's `id`). Nullable; populated at ingest. |

No corresponding column is added to `endpoints` / `tasks` / `ai_traces` in this phase: each entity row's own `id` (= its OTel span_id, post-Phase-1) is its identity, and Phase 1's `parent_span_id` + `FindParentRef` already cover the parent-entity breadcrumb on the detail page. A later phase can denormalize `parent_entity_id` directly onto entity rows if `FindParentRef`'s walk-up becomes a bottleneck — but it's not load-bearing for the subtree-leak fix.

## Schema changes

### ClickHouse — `backend/app/migrations/ch/`

- `0067_add_entity_id_to_spans.up.sql` — `ALTER TABLE spans ADD COLUMN entity_id Nullable(UUID) DEFAULT NULL`
- `0068_add_entity_id_index_spans.up.sql` — `ALTER TABLE spans ADD INDEX idx_entity_id entity_id TYPE bloom_filter GRANULARITY 4` (the table's primary key is ordered by `(project_id, trace_id, start_time)`, so a query keyed on `entity_id` needs its own skip index)

Existing rows materialize `entity_id = NULL`. See **Backfill** below for the optional one-shot mutation to populate historical rows.

### SQLite — `backend/app/migrations/sqlite_telemetry/`

- `0012_spans_entity_id.up.sql`:
  ```sql
  ALTER TABLE spans ADD COLUMN entity_id TEXT DEFAULT NULL;
  UPDATE spans SET entity_id = trace_id WHERE entity_id IS NULL;
  ```

  The `UPDATE` is the backfill: in pre-Phase-2 data, every trace had exactly one root entity, so `entity_id == trace_id` is exact. (For ClickHouse this is too expensive to do inline — see Backfill section.)

## Model changes

- `backend/app/models/span.model.go` — add `EntityId *uuid.UUID \`json:"entityId,omitempty" ch:"entity_id"\``. Pointer to allow NULL for historical/orphan rows.
- `backend/app/repositories/span.repository_sqlite.go` — add matching field to the `span` row struct with `lit:"entity_id"`, and update `spanToRow` / `toModel`.

No frontend type change: the frontend never reads `entity_id` directly — the spans array it receives is already pre-filtered server-side.

## Ingest changes — `backend/app/controllers/otelcontrollers/trace_converter.go`

Today the converter does one pass over `rs.ScopeSpans`, builds `parentMap` and `spanById`, then a second pass to classify and emit rows. Add a small **kind-of-each-span map** during the first pass (so we know which spans land in special tables) and a `resolveEntityId` walker for the second pass.

```go
// First pass additions: track which spans become entities in THIS batch.
spanKindInBatch := map[string]string{} // spanIdStr → "endpoint"|"task"|"ai_trace" (only non-"other")
for _, entry := range allSpans {
    if k := classifyKind(entry.span, entry.span.Attributes); k != "other" {
        spanKindInBatch[string(entry.span.SpanId)] = k
    }
}

// resolveEntityId returns the nearest special-kind ancestor's spanId, INCLUDING the span
// itself if it's a special kind. Walks parentMap; returns (uuid.Nil, false) if the chain
// runs out without hitting a special-table span.
spanToEntityId := map[string]uuid.UUID{}
var resolveEntityId func(spanIdStr string) (uuid.UUID, bool)
resolveEntityId = func(spanIdStr string) (uuid.UUID, bool) {
    if eid, ok := spanToEntityId[spanIdStr]; ok {
        return eid, eid != uuid.Nil
    }
    if _, isSpecial := spanKindInBatch[spanIdStr]; isSpecial {
        eid := otelSpanIDToUUID([]byte(spanIdStr))
        spanToEntityId[spanIdStr] = eid
        return eid, true
    }
    if parent, ok := parentMap[spanIdStr]; ok {
        if eid, found := resolveEntityId(parent); found {
            spanToEntityId[spanIdStr] = eid
            return eid, true
        }
    }
    spanToEntityId[spanIdStr] = uuid.Nil
    return uuid.Nil, false
}
```

Then in the second-pass dual-write block (lines 152-170 today):

```go
if !isRoot {
    var entityId *uuid.UUID
    if eid, ok := resolveEntityId(string(span.SpanId)); ok {
        entityId = &eid
    }
    spans = append(spans, models.Span{
        Id: spanId, TraceId: traceId, ProjectId: projectId,
        Name: spanName, StartTime: startTime, Duration: duration,
        RecordedAt: startTime, ParentSpanId: parentSpanId,
        EntityId: entityId,
    })
}
```

### What happens for each span kind

- **Root endpoint/task/ai_trace span** — single-write to its special table; **no** generic span row (already true). Nothing to resolve.
- **Non-root span that itself lands in a special table** (e.g. the CONSUMER `TestTracewayJob` span when it shares a trace with the dispatch HTTP request): dual-write to `spans` with `entity_id = own spanId`. The task detail page will show this row at the top of its waterfall.
- **Generic span (kind="other")**: walks up `parentMap` to find the nearest special-table ancestor; `entity_id = that ancestor's spanId`. If the chain runs out in this batch (parent not present), `entity_id = NULL`.

### Cross-batch parents (the dispatch-job real case)

For `test/traceway/dispatch-job` specifically, in-batch resolution is sufficient — the consumer batch contains both the consumer span itself (special, kind=task) and any spans it emits inside `handle()`, so every span in that batch resolves to the consumer entity.

The case where in-batch resolution fails is **a non-special span whose parent is in a different batch**. Example: a long-running streaming handler that emits internal spans before the root HTTP server span finishes and gets flushed. The orphan span lands with `entity_id = NULL` and won't appear on any detail page until backfilled.

This is acceptable for now because OTel SDKs typically flush parent + descendants together (BatchSpanProcessor groups by trace context only by accident; spans are usually exported when their batch timer fires). If the orphan rate becomes material, add a second pass at read time: for `entity_id IS NULL` rows, walk `parent_span_id` via one round-trip into the DB (`spans` + the three entity tables). For now, document and move on.

## Repository changes — `backend/app/repositories/span.repository.go` (+ `_sqlite`)

### Insert: add the column

ClickHouse `InsertAsync`:

```go
batch, err := chdb.Conn.PrepareBatch(..., "INSERT INTO spans (id, trace_id, project_id, name, start_time, duration, recorded_at, parent_span_id, entity_id)")
...
batch.Append(s.Id, s.TraceId, s.ProjectId, s.Name, s.StartTime, int64(s.Duration), s.RecordedAt, s.ParentSpanId, s.EntityId)
```

SQLite: add `EntityId *uuid.UUID \`lit:"entity_id"\`` to the `span` row struct; `spanToRow` / `toModel` pass it through.

### Add `FindByEntityId`

```go
// ClickHouse
func (r *spanRepository) FindByEntityId(ctx context.Context, projectId, entityId uuid.UUID) ([]models.Span, error) {
    rows, err := chdb.Conn.Query(ctx, `
        SELECT id, trace_id, project_id, name, start_time, duration, recorded_at, parent_span_id, entity_id
        FROM spans
        WHERE project_id = ? AND entity_id = ?
        ORDER BY start_time ASC`, projectId, entityId)
    // ...
}

// SQLite mirror — same query, lit-style parameters.
```

`FindByTraceId` and `FindById` stay (still used by tests, and useful for debugging / future trace-wide views). They are no longer called from the detail controllers.

### No isRoot / non-root counts here

The Phase 1 `parent_span_id IS NULL` filters live on endpoints/tasks/ai_traces, not on `spans`. Don't touch them.

## Controller changes

Two one-line swaps:

- `backend/app/controllers/endpoint_detail.controller.go:82` — `SpanRepository.FindByTraceId(c, projectId, endpoint.TraceId)` → `SpanRepository.FindByEntityId(c, projectId, endpoint.Id)`.
- `backend/app/controllers/task_detail.controller.go:69` — `SpanRepository.FindByTraceId(c, projectId, task.TraceId)` → `SpanRepository.FindByEntityId(c, projectId, task.Id)`.

`endpoint.Id` and `task.Id` are now the entity's own span_id (Phase 1 already made this true). They match `entity_id` of every span that belongs to that entity.

**Exceptions stay on `FindAllByTraceId`** — exceptions still hang off the W3C trace, not the entity. An exception inside a job's `handle()` and an exception inside the request handler are both interesting on the issues page regardless of which entity they belonged to. No behavior change.

The AI-trace detail (`ai_trace.controller.go:127`) doesn't currently load a spans waterfall, so it's untouched. If a future change adds one, use `FindByEntityId(aiTrace.Id)`.

## Backfill (operational)

**SQLite (inline):** the migration's `UPDATE spans SET entity_id = trace_id WHERE entity_id IS NULL` runs against the file at boot. Pre-Phase-2 data had at most one root entity per trace, so `entity_id = trace_id` is exact for every historical row.

**ClickHouse (deferred):** don't inline the equivalent `ALTER TABLE spans UPDATE entity_id = trace_id WHERE entity_id IS NULL` mutation in the migration runner — `spans` is the highest-volume table and mutations rewrite parts. Document this as an **operator-run one-shot** in the deploy notes:

```sql
ALTER TABLE spans UPDATE entity_id = trace_id WHERE entity_id IS NULL;
```

Until that mutation runs, historical endpoint/task detail pages will show empty waterfalls (because `FindByEntityId` won't match `NULL` entity_id rows even when `entity_id = trace_id` would be the right answer). To avoid this UX cliff, have the detail controllers do a **read-side fallback**: if `FindByEntityId` returns zero rows AND the entity's id != trace_id is false (i.e., we can't distinguish "no spans" from "old data"), fall through to `FindByTraceId`. Concretely:

```go
spans, err := repositories.SpanRepository.FindByEntityId(c, projectId, endpoint.Id)
if err != nil { ... }
if len(spans) == 0 && endpoint.Id == endpoint.TraceId {
    // Pre-Phase-1 row (id == trace_id by backfill) — fall back to trace-wide query
    // to keep historical detail pages working until the CH mutation runs.
    spans, err = repositories.SpanRepository.FindByTraceId(c, projectId, endpoint.TraceId)
}
```

This fallback is **idempotent and self-disabling**: it only fires for rows where `id == trace_id`, which is exactly the pre-Phase-1 backfill marker. New rows (post-Phase-1) have `id != trace_id`, so the fallback never runs and the subtree-correct path is the only path. Once the operator runs the CH mutation, the fallback also disappears for historical rows (because spans get entity_id values and the first query returns them). Safe to keep indefinitely; safe to remove in a later cleanup.

## Critical files to modify

| File | Purpose |
|------|---------|
| `backend/app/migrations/ch/0067_add_entity_id_to_spans.up.sql` | New ClickHouse column |
| `backend/app/migrations/sqlite_telemetry/0012_spans_entity_id.up.sql` | SQLite column + backfill |
| `backend/app/models/span.model.go` | `EntityId *uuid.UUID` field |
| `backend/app/repositories/span.repository.go` | Insert column, add `FindByEntityId` |
| `backend/app/repositories/span.repository_sqlite.go` | Row struct, insert, `FindByEntityId` |
| `backend/app/controllers/otelcontrollers/trace_converter.go` | `spanKindInBatch` + `resolveEntityId` + populate `EntityId` |
| `backend/app/controllers/endpoint_detail.controller.go` | Switch to `FindByEntityId(endpoint.Id)` + historical fallback |
| `backend/app/controllers/task_detail.controller.go` | Switch to `FindByEntityId(task.Id)` + historical fallback |

No frontend changes (it consumes whatever the detail endpoint returns).

## Consistency check after the change

| Scenario | Endpoint detail spans | Task detail spans |
|----------|----------------------|---------------------|
| Plain HTTP request, no job dispatched | All non-root spans in the request — unchanged from Phase 1 (they all resolve `entity_id = endpoint.id`) | n/a |
| HTTP request that dispatches a queued job (the Zentigo `test/traceway/dispatch-job` repro) | Only the producer span and any request-side internal spans (`entity_id = endpoint.id`). **Consumer span and `handle()` internals are excluded.** | Consumer span at the top + its descendants (`entity_id = task.id`). **Producer span and request-side spans are excluded.** |
| HTTP request that calls an LLM (gen_ai child) | Endpoint shows non-AI internal spans. The gen_ai span lands in `ai_traces`; its dual-write to `spans` has `entity_id = ai_trace.id`, so it's excluded from the endpoint's waterfall — clicking the linked-ai-trace icon on the endpoint detail still navigates to the AI trace. | (n/a — no task) |
| Pre-Phase-2 historical endpoint/task | Read-side fallback kicks in (`endpoint.Id == endpoint.TraceId` → use `FindByTraceId`) | Same fallback |
| Orphan span (parent in another batch, no special ancestor) | Not shown anywhere until backfilled — known acceptable loss |

## Verification

1. **Backend tests.** Add to `trace_converter_test.go`:
   - Extend `TestConvertTraces_HttpRootWithGenAiChild`: assert the gen_ai child's dual-written span row has `EntityId == childSpanUUID` (itself), NOT the root's spanId.
   - New `TestConvertTraces_DispatchJob_TwoBatches`: simulate the dispatch-job case by feeding two `ExportTraceServiceRequest`s back-to-back (or one request with both scopes). Assert: endpoint row's entity has `entity_id` on its child producer span pointing to the endpoint; task row's child internal spans have `entity_id` pointing to the task; no cross-contamination.
   - `cd backend && go test ./...`.

2. **End-to-end.**
   - Run Zentigo with `OTEL_SDK_DISABLED=false` (the existing `.env` setting points at `http://localhost:8082/api/otel`).
   - Hit `GET /test/traceway/dispatch-job`, then run `php artisan queue:work` once to drain the job.
   - Open the endpoint detail in Traceway: confirm the waterfall contains the producer span only, NOT the consumer span or its DB query.
   - Open the task detail: confirm the waterfall starts with the consumer span and contains its DB query — no producer span.
   - Both pages still show the cross-link (`parentRef` from Phase 1's `FindParentRef`).

3. **Backfill verification (CH).** On a populated database, before running the mutation: open a historical endpoint detail and confirm the read-side fallback returns spans. Run `ALTER TABLE spans UPDATE entity_id = trace_id WHERE entity_id IS NULL`. After it settles, open the same endpoint and confirm the same spans appear without the fallback (debug-log the path taken).

## Session log

- [ ] CH + SQLite migrations for `entity_id`
- [ ] `EntityId` on `models.Span` + SQLite row wrapper
- [ ] `spanKindInBatch` + `resolveEntityId` in converter; populate `EntityId` on dual-writes
- [ ] Span repo: insert `entity_id` + new `FindByEntityId` (CH + SQLite)
- [ ] Detail controllers: switch to `FindByEntityId(row.Id)` with historical fallback
- [ ] Trace converter tests: extended HTTP+gen_ai assertion + new dispatch-job two-batch test
- [ ] Document CH mutation in deploy notes (`ALTER TABLE spans UPDATE entity_id = trace_id WHERE entity_id IS NULL`)

## Known limitations & Phase 3 follow-ups

These are NOT bugs in the Phase 2 design — they're cases where Traceway's coverage of OTel is intentionally narrower than the full spec, or where the next phase needs to surface child entities. Calling them out so we don't quietly regress them.

### 1. Child entities aren't surfaced on parent waterfalls — **RESOLVED in Phase 3**

`FindChildEntitiesBySpanIds` plus the controller wiring landed. Endpoint/task detail responses include a `childEntities: ChildEntityResponse[]` array (transitive: any entity whose `parent_span_id` is in the parent's owned-span subtree). The waterfall component interleaves them with regular spans by start time and renders each as a click-target linking to its own detail page with a kind-specific icon. See **Phase 3** for the design + verification.

### 2. `parentMap` is scoped per `ResourceSpans` block

`parentMap`, `spanById`, `spanKind`, and the two resolver caches are all declared inside the `for _, rs := range req.ResourceSpans` loop in `trace_converter.go:32`. If a single OTLP request contains a parent in `ResourceSpans[0]` and its child in `ResourceSpans[1]` (different Resources — usually different service.name or service.version), `resolveEntityId` cannot connect them and the child's `entity_id` ends up NULL.

In practice OTel SDKs put a whole trace in one `ResourceSpans` block (Resource is per-service-instance, not per-trace), so this is rare. But it's the same shape as cross-batch resolution (Phase 2 already documents that) and worth flagging together. If the orphan rate becomes material, the fix is to lift the maps out of the loop and key them on `(trace_id_bytes, span_id_bytes)` rather than just `span_id_bytes`.

### 3. OTel Span Links are not processed

The converter never reads `span.Links`. Some instrumentations (notably the Sentry-style "follows from" semantic, and certain messaging consumers that prefer Links over parent_span_id) reference an upstream span via Links instead of `parent_span_id`. In those cases:
- The consumer span appears as a **root** (no parent_span_id) — it lands in the `tasks` table as a non-root-filter-passes row, but it's disconnected from the producer entity.
- The producer→consumer association is lost; the task detail page has no `parentRef` link back to the dispatching endpoint.

**Phase 3 fix:** in the converter, if `parent_span_id` is empty but `span.Links` contains exactly one entry with `attributes["traceway.parent"] = true` (or simply the first Link, matching keepsuit/Sentry conventions), use that Link's `SpanId` as a synthetic parent for entity resolution and the `parent_span_id` column. Needs a decision on conventions.

### 4. `span.Status.Code = ERROR` without an `exception` event creates no row

Pre-existing. The converter only iterates `span.Events` looking for `event.Name == "exception"` (line ~225). A span that completes with Status = ERROR but no embedded exception event will not create an `ExceptionStackTrace` row. The issues page won't show it. Not entity_id related — the issue and the fix predate this work.

### 5. Non-exception span events are dropped

Pre-existing. The converter ignores any `span.Events` whose name isn't `"exception"`. OTel allows arbitrary events (log lines, custom signals); Traceway has no generic event table to land them in.

### 6. Cross-service distributed traces show one waterfall per service

Pre-existing & by design. Service A's endpoint detail shows A's spans only; service B's endpoint detail shows B's spans only. The two are joined via `distributed_trace_id`. There's no "follow this trace across services" waterfall view; users navigate manually. Listed here so the "no cross-service waterfall" gap isn't conflated with the Phase 2 entity_id fix.

### 7. Cycle protection in `resolveEntityId`

Already handled in the implementation: the resolver sets `spanToEntityId[spanIdStr] = uuid.Nil` **before** recursing into its parent, so a pathological parent cycle (`A → B → A`) terminates naturally on the second visit. `resolveTraceId` (line ~70 of the converter) still has the original "cache-after-recurse" shape — same theoretical risk, predates this work, low priority because cycles only happen with malformed OTel data. Worth bringing into line in a separate cleanup.

---

## Session log

Use this section to track progress across sessions. Mark items as work proceeds.

- [ ] Migrations (CH + SQLite) including trace_id backfill
- [ ] Model fields (TraceId, SpanId, ParentSpanId, DistributedTraceId; TraceType doc-comment)
- [ ] OTel converter: Id=spanId refactor + dual-write + trace_type from kind + storage-key per row
- [ ] Repository inserts updated (column lists, batch args)
- [ ] Repository SELECT widening (FindAll, FindByEndpoint, etc. — include trace_id / span_id / parent_span_id)
- [ ] Repository `isRoot` filter on three `/grouped` queries (count AND joinWhere)
- [ ] `countIf(parent_span_id IS NOT NULL) AS non_root_count` aggregate on the three `/grouped` queries + `NonRootCount` stats fields
- [ ] `aiTraceRepository.FindBySpanIds` returning `{Id, TraceName}`
- [ ] `FindParentRef` with walk-up + depth cap
- [ ] Detail controllers: dereference via `row.TraceId` for FindByTraceId / FindAllByTraceId
- [ ] Controllers: `isRoot` parsing + detail parentRef enrichment + linkedAiTrace enrichment
- [ ] `distributed_trace.controller.go`: add `ai_trace` branch
- [ ] Frontend filter UI (3 pages, default 'all')
- [ ] Frontend types update (Span.linkedAiTrace*, ParentRef, EndpointStats/TaskStats/AiTraceStats.nonRootCount)
- [ ] Frontend "Not Root" chip on list rows (nonRootCount > 0) and detail headers (parentSpanId != null) with tooltip
- [ ] Frontend span→ai_trace link
- [ ] Frontend child→parent link
- [ ] Tests (converter + multi-row)
- [ ] End-to-end verification (CH + SQLite, pre-change + new traces)

---

# Phase 3: Child entities on parent waterfalls

## Why this is needed

Phase 2 made detail-page waterfalls subtree-correct by switching from `FindByTraceId(row.TraceId)` to `FindByEntityId(row.Id)`. That was the right call — it stopped a task's spans from leaking into the originating endpoint's waterfall, and vice versa. But it traded one bug for a smaller, real-world one:

**Child entities of the current row no longer appear in the waterfall at all.** Concretely:

- `/endpoints/<E>` where `E` contains a gen_ai child `A`: `A` has `entity_id = A` (it owns its own subtree as an `ai_trace` entity). `FindByEntityId(E)` returns spans owned by `E` but not `A` itself. Phase 1's `linkedAiTraceId` enrichment runs `aiTraceRepository.FindBySpanIds(spanIds)` over the returned spans — but `A` isn't in there, so the "View AI trace" icon never renders. Pre-Phase-2, `FindByTraceId` returned `A` as a dual-written non-root span and the icon worked.
- `/endpoints/<E>` where `E` dispatches a queued job (the Zentigo `dispatch-job` repro): the task `C` has `entity_id = C` (itself). `E`'s waterfall doesn't even hint that a job was dispatched. The user has to know to look at the tasks list, find `C`, and inspect it independently.
- `/tasks/<T>` where the job calls an LLM: same shape — the AI trace under the task is invisible from the task detail.

The Phase 1 design treated dual-written rows as the surfacing mechanism. Phase 2 correctly stopped including them in foreign waterfalls. Phase 3 reintroduces them — but explicitly, as **child entities** in their own response field, so the front-end can render them with the right click target and the right visual cue (it's a navigable child, not just another row in the waterfall).

## Why `distributed_trace_id` and `trace_id` aren't the answer

Three IDs live in the system, easy to confuse:

| Field | Set when | Useful for "all rows in this distributed trace"? |
|-------|----------|--------------------------------------------------|
| `trace_id` | Always (W3C OTel trace_id, propagated via `traceparent`) | Yes, but matches **siblings and ancestors too** in cross-service traces |
| `distributed_trace_id` | Only when the Traceway-SDK explicitly sets the `traceway.distributed_trace_id` span attribute (`trace_converter.go:145`) — i.e. cross-service hops that Traceway instruments | NULL for OTel-only flows like the Zentigo Laravel repro; can't be relied on |
| `entity_id` (Phase 2) | At ingest, every non-root span | Identifies the **owning** entity — but doesn't tell us which entities are children of a given parent |

`distributed_trace_id` is a no-op here because the OTel-only flows we care about don't set it. `trace_id` is too broad: in a cross-service trace `A → B → C`, all three endpoints share `trace_id`, so `WHERE trace_id = B.TraceId AND id != B.Id` returns `{A, C}` — but `A` is `B`'s **ancestor**, not a child. B's detail page would render its own parent as a "child", which is upside-down.

The precise filter is "entities whose `parent_span_id` chain leads back into MY entity's owned-span subtree." We already have that subtree — it's the span_ids returned by `FindByEntityId(self.Id)` plus `self.Id` itself. The right query is:

```sql
SELECT … FROM endpoints WHERE project_id = ? AND parent_span_id IN (<self.Id + my_owned_span_ids>)
UNION ALL
SELECT … FROM tasks      WHERE project_id = ? AND parent_span_id IN (…)
UNION ALL
SELECT … FROM ai_traces  WHERE project_id = ? AND parent_span_id IN (…)
```

This catches **transitive** children at any depth (because intermediate generic spans are in my owned set), and correctly excludes ancestors and siblings (their `parent_span_id` is outside my subtree).

## Repository — `backend/app/repositories/`

Add to `parent_ref.go` (the file already houses related cross-table walkers — `FindParentRef` lives there):

```go
type ChildEntityRef struct {
    Kind         string        // "endpoint" | "task" | "ai_trace"
    Id           uuid.UUID     // row PK = OTel span_id
    Name         string        // endpoint route / task name / trace name
    ParentSpanId uuid.UUID     // not nullable here — by construction every match has a parent in the input set
    TraceId      uuid.UUID
    RecordedAt   time.Time
    Duration     time.Duration // nanoseconds, matches existing models.*.Duration shape
}

func FindChildEntitiesBySpanIds(ctx context.Context, projectId uuid.UUID, spanIds []uuid.UUID) ([]ChildEntityRef, error)
```

Implementation notes:

- **Empty input → empty output, no query.** Avoid `IN ()` syntax errors.
- **ClickHouse**: three separate queries (one per entity table) with dynamic placeholder lists. Combine in Go, sort by `RecordedAt` ascending so the frontend renders children chronologically in the waterfall.
- **SQLite**: same shape via `lit.SelectNamed`. lit doesn't support array params natively — build a comma-separated placeholder string and a flat param map (`{"sid_0": id0, "sid_1": id1, ...}`).
- **Exclude self defensively**: if `self.Id` is in the input list (it should be, so child-entities-of-self gets caught), the result can't include `self` because no row's `parent_span_id == self.Id` AND `self.Id == self.Id` — that'd be a row with `parent_span_id = id`, which is a self-loop and doesn't happen.

## Models / response — new types in the existing detail controllers

`backend/app/controllers/endpoint_detail.controller.go` and `task_detail.controller.go` gain:

```go
type ChildEntityResponse struct {
    Kind         string    `json:"kind"`
    Id           uuid.UUID `json:"id"`
    Name         string    `json:"name"`
    ParentSpanId uuid.UUID `json:"parentSpanId"`
    TraceId      uuid.UUID `json:"traceId"`
    RecordedAt   string    `json:"recordedAt"`           // ISO 8601 string, matches other timestamp fields
    Duration     int64     `json:"duration"`             // nanoseconds (matches Span.duration)
}
```

Added to both `EndpointDetailResponse` and `TaskDetailResponse`:

```go
ChildEntities []ChildEntityResponse `json:"childEntities"`
```

Always non-nil (`[]` when empty, never `null`) so the frontend can iterate without a guard.

## Controller wiring (endpoint + task — identical shape)

After loading spans via `FindByEntityId(row.Id)`:

```go
subtreeIds := make([]uuid.UUID, 0, len(spans)+1)
subtreeIds = append(subtreeIds, row.Id)
for _, s := range spans {
    subtreeIds = append(subtreeIds, s.Id)
}

childSpan := traceway.StartSpan(c, "loading child entities")
children, err := repositories.FindChildEntitiesBySpanIds(c, projectId, subtreeIds)
childSpan.End()
if err != nil { ... }

childEntities := make([]ChildEntityResponse, 0, len(children))
for _, ch := range children {
    childEntities = append(childEntities, ChildEntityResponse{
        Kind:         ch.Kind,
        Id:           ch.Id,
        Name:         ch.Name,
        ParentSpanId: ch.ParentSpanId,
        TraceId:      ch.TraceId,
        RecordedAt:   ch.RecordedAt.Format("2006-01-02T15:04:05Z07:00"),
        Duration:     int64(ch.Duration),
    })
}
```

### What about the legacy `linkedAiTraceId` enrichment?

Keep it for now. It still works when an AI-trace child happens to be in the spans array (which it isn't under Phase 2, but might be again if we change the resolution semantics later). For new code paths, `ChildEntities` is the canonical source of "child entities of this row." The frontend will read both during the migration period.

We'll remove the `linkedAiTraceId` field in a later cleanup once the frontend is fully on the new shape — track it as a follow-up in the session log, not load-bearing for Phase 3.

### AI-trace detail (`ai_trace.controller.go`) — deferred

The AI-trace detail page doesn't render a spans waterfall today (`ai_trace.controller.go:127` doesn't call `SpanRepository`), so adding `ChildEntities` there would be a no-op until the frontend grows that affordance. Out of scope for Phase 3; revisit when/if the AI-trace detail learns to show a waterfall.

## Frontend — `frontend/src/routes/`

### Types

`endpoints/[endpoint]/[endpointId]/+page.svelte` and `tasks/[task]/[taskId]/+page.svelte` are the two pages that need updates. Each has a local response-type definition near the top of the script block:

```ts
type ChildEntity = {
    kind: 'endpoint' | 'task' | 'ai_trace'
    id: string
    name: string
    parentSpanId: string
    traceId: string
    recordedAt: string
    duration: number  // nanoseconds
}
```

Add `childEntities: ChildEntity[]` to the response interface.

### Rendering in the waterfall

The spans waterfall (`frontend/src/lib/components/spans/...` — already used by both detail pages) needs to accept and render `childEntities` alongside spans. The simplest shape: pass `childEntities` as a separate prop to the same waterfall component; the component renders them as visually distinct rows interleaved at their `recordedAt`, with kind-specific icons and a `<a href>` to the appropriate detail page:

| Kind | Icon | Link target |
|------|------|-------------|
| `endpoint` | `Globe` (or whatever the endpoint list uses) | `/endpoints/<encodeURIComponent(name)>/<id>` |
| `task` | `Briefcase` (matches the tasks list) | `/tasks/<encodeURIComponent(name)>/<id>` |
| `ai_trace` | `Sparkles` (matches the ai-traces list) | `/ai-traces/<encodeURIComponent(name)>/<id>` |

Use the existing sticky-params helper (`addStickyParamsToHref` in `frontend/src/lib/utils/navigation.ts`) so the time range carries over.

Visual treatment: same row height as a span, but a different background tint and a chip/badge to the right of the name showing the kind. Tooltip on the row: "Child {kind} — click to open."

## Critical files to modify

| File | Purpose |
|------|---------|
| `backend/app/repositories/parent_ref.go` | Add `ChildEntityRef` + `FindChildEntitiesBySpanIds` (ClickHouse) |
| `backend/app/repositories/parent_ref_sqlite.go` (new, or fold into existing build-tag pair) | SQLite mirror — build tag `!pgch` |
| `backend/app/controllers/endpoint_detail.controller.go` | Build `subtreeIds`, call `FindChildEntitiesBySpanIds`, attach `ChildEntities` to response |
| `backend/app/controllers/task_detail.controller.go` | Same |
| `frontend/src/routes/endpoints/[endpoint]/[endpointId]/+page.svelte` | Type + prop wiring |
| `frontend/src/routes/tasks/[task]/[taskId]/+page.svelte` | Same |
| `frontend/src/lib/components/spans/...` (waterfall component) | Accept + render `childEntities` rows |

## Verification

1. **Backend unit test** for `FindChildEntitiesBySpanIds` (SQLite testhelper):
   - Seed: endpoint `E`, span `P` (entity_id=E, parent=E), task `C` (parent=P), AI trace `A` (parent=E), unrelated entity `X` (parent outside subtree).
   - Call `FindChildEntitiesBySpanIds(projectId, [E.Id, P.Id])`.
   - Assert result has exactly `{C, A}` — transitive child (`C` via `P`) and direct child (`A` via `E`). `X` excluded.
   - Assert each returned ref has the right `Kind` and the `ParentSpanId` matches what we seeded.

2. **End-to-end (dispatch-job repro)**:
   - Hit `GET /test/traceway/dispatch-job`; drain the queue.
   - Open `/endpoints/<route>/<id>`: confirm the waterfall now shows the task `TestTracewayJob` as a child entity row, at the consumer's `recordedAt`. Click → navigates to the task detail.
   - Open the task detail: spans still subtree-correct (no producer-side leak); `parentRef` link still goes back to the endpoint.

3. **End-to-end (HTTP → gen_ai child)**:
   - Run an OTel agent that emits an HTTP root with a gen_ai child.
   - Endpoint detail: the AI trace appears as a child entity row with the AI icon; click → navigates to `/ai-traces/<name>/<id>`. **This is the regression closed.**

4. **Cross-service safety check**:
   - Two services `A → B`, both emit endpoints sharing `trace_id`.
   - From A's endpoint detail: B's endpoint appears in `childEntities` (correct — it's a descendant).
   - From B's endpoint detail: A's endpoint does NOT appear in `childEntities` (correct — A is an ancestor; only `parentRef` should surface it). This validates that the subtree filter beats a naive `WHERE trace_id = same`.

## Session log

- [x] `ChildEntityRef` + `FindChildEntitiesBySpanIds` (CH + SQLite)
- [x] Endpoint detail controller: subtreeIds + child-entities query + response field
- [x] Task detail controller: same
- [x] Detail response types: `ChildEntities` field (always `[]`)
- [x] Repo unit test: seeded subtree returns correct transitive children, excludes ancestors/siblings (`child_entities_repository_test.go`)
- [x] Frontend types updated (`ChildEntity` in `spans.ts`, endpoint + task detail page response types)
- [x] Waterfall renderer accepts + renders childEntities (`child-entity-row.svelte`, interleaved in `span-waterfall.svelte`)
- [x] Side-fix: register `ai_traces` SQLite table name override (`aiTraceRowNaming`) — pre-existing bug, would have broken `AiTraceRepository.InsertAsync` in production SQLite
- [ ] End-to-end dispatch-job verification (manual, requires Zentigo + `php artisan queue:work`)
- [ ] End-to-end gen_ai-under-endpoint verification (manual)
- [x] Mark Known Limitations #1 as resolved
- [ ] Follow-up: remove `linkedAiTraceId` enrichment once frontend is fully on `childEntities`
