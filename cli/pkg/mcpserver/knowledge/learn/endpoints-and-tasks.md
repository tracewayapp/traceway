# Endpoints and tasks

Both pages are hidden for browser and mobile projects. Both are built from spans promoted at ingest (`traces-and-logs.md`), so a request or a task run always opens its own span waterfall.

## Endpoints (`/endpoints`)

Every HTTP request a backend serves, grouped by method and route template (`GET /api/users/:id`).

**The list**

| Column | Meaning |
|---|---|
| Endpoint | Method and route. Badges: **Stream** (SSE or WebSocket: left out of latency and Apdex, scored on errors only), **Healthcheck** (the project drops healthy healthcheck requests, so only those with status 400 or more are kept), **Non Root** or **Mixed** (all or some requests have a parent span, usually an upstream service in the same trace) |
| Calls | Request count |
| Typical | P50 |
| Slow | P95 |
| Awful | P99 |
| Impact | The impact level (below). The default sort. |

- Filters: search, HTTP method, and Root (all, root, non-root).
- The **Performance** chart above the table shows total time as a stacked area of the top 5 endpoints plus Other (drag across it to zoom the range), or P50, P95 or P99 as a bar chart of the top 8 endpoints on the current table page.
- **Healthchecks**: the project settings (**Edit Project > Healthchecks**) can drop healthy healthcheck requests, with extra healthcheck paths.
- Requests that return 404 without matching a route are grouped as `UNMATCHED`, so scanners do not create one endpoint per probed path. A 404 from a real route keeps its route.

**Impact score**: one number from 0 to 1 per endpoint that ranks the endpoint hurting users the most first. It is the **worst** of five components:

1. Apdex degradation: 1 minus Apdex.
2. Error-rate floor, where bad means slow or 5xx: over 10% gives 0.25, over 20% gives 0.50, over 33% gives 0.75.
3. P99 floor: over 3 s gives 0.25, over 6 s gives 0.50, over 8 s gives 0.75.
4. Client error (4xx) floor, with more than 10 requests and ignoring `UNMATCHED`: over 25% gives 0.50, over 50% gives 0.75.
5. A volume-aware floor: a smaller error share with a large number of bad requests still ranks high.

| Level | Score |
|---|---|
| Critical | 0.75 and up |
| High | 0.50 and up |
| Medium | 0.25 and up |
| Good | below 0.25 |

Every endpoint has an **impact reason** naming the dominant factor: "P99 latency is 9.2s", "N errors with X% error rate", "X% of requests returning 4xx errors", "X% of requests are slow or failing", or "Endpoint is healthy". Stream endpoints get error-only versions such as "Streaming endpoint is healthy".

**Apdex** uses fixed bands: satisfied up to 750 ms and not a 5xx, tolerating up to 1500 ms, anything slower or a 5xx is frustrated. Apdex = (satisfied + tolerating / 2) / total. 1.0 is perfect. Below about 0.85, users notice.

**Expected performance (slow endpoint marks)**: some endpoints are allowed to be slow, such as a report export or a batch upload. On the endpoint page, **Expected Performance** sets an offset in milliseconds and a reason. The offset is added to the Apdex bands and the P99 floor, so the endpoint stops being penalized up to that latency. Raw percentiles are never adjusted. An offset of 0 removes the mark. The header then shows "+N ms offset". Setting it needs write access on the project.

**The endpoint page** (`/endpoints/<endpoint>`)

- Tiles: average, median, P95 and P99 response time, Apdex, average error rate, average throughput (requests per minute). Stream endpoints show connections, error rate and throughput instead.
- The request table: recorded at, duration, status, body size, client IP, server, version and attributes, sortable. Search attribute values and add attribute filters (equals, not equals, contains, not contains) to find the requests of one tenant or user.

**A request** (`/endpoints/<endpoint>/<id>`)

- Details: endpoint, status, duration, recorded at, server, version, client IP, body size, attributes.
- **Exception Occurred**, linking to the issue, and captured **Messages**.
- The **Spans** waterfall: database queries, outgoing HTTP calls and other work inside the request, nested by parent. Large traces start folded, with **Expand all** and **Collapse all**. Each span opens its attributes. **Open the whole trace** jumps to the trace page.
- **Logs** for the trace, and the **Distributed Trace** card.

## Tasks (`/tasks`)

Background work outside a request: queue consumers (`CONSUMER` spans from any queue driver), scheduled commands (root `console.command` spans), cron jobs and scripts.

- **The list**: task name, runs, typical (P50), slow (P95), sorted by runs. Search and the Root filter. There is no impact score. Rank by count times (P95 minus P50) for a rough idea of where time goes.
- **The task page**: average, median, P95 and P99 duration and throughput, then the runs: recorded at, duration, client IP, server, version, attributes.
- **A run**: details, the exception if one happened, messages, the span waterfall and logs.
- A run **failed** when an exception was recorded on its own span. That is what the Task Failure Rate alert counts.

A queue worker whose span is parented to the request that enqueued it shares that request's trace id, so the distributed trace card shows the endpoint and the task together.

## Reading and changing it from an agent

| Task | CLI | MCP tool |
|---|---|---|
| Endpoints with percentiles and impact | `traceway endpoints list --since 24h [--order-by impact\|count\|p95\|lastSeen] [--search S]` | `list_endpoints` |
| Latency over time (find when it changed) | `traceway endpoints chart [--metric-type total_time\|p50\|p95\|p99] [--interval-minutes N]` | `endpoints_chart` |
| One request with its spans and errors | `traceway endpoints show <id> --recorded-at <t>` | `get_endpoint_request` |
| An endpoint's expected-performance mark | `traceway endpoints slow <endpoint>` | `get_slow_endpoint_config` |
| One task run | `traceway tasks show <id> --recorded-at <t>` | `get_task` |
| Mark an endpoint slow (write) | `traceway-write endpoints mark-slow <endpoint> --offset-ms N --reason R` | `set_slow_endpoint` |

There is no task list command. Find a task's runs through its exceptions, its trace id or the dashboard. Latency percentiles only come from endpoints, never from `metrics query`. The `traceway` skill's performance flow covers a full latency investigation.
