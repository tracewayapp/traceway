# Spans, traces and logs

## Spans are the primitive

Traceway is OpenTelemetry native. A **trace** is one logical operation (a request, a queued job, an LLM call) as it moves through the system. It is a tree of **spans** that share one **trace id** (32 hex characters). Each span has a span id, a parent span id (empty for a root), a name, a kind (`SERVER`, `CLIENT`, `INTERNAL`, `PRODUCER`, `CONSUMER`), a start time, a duration, a status, attributes and events.

At ingest, some spans are also copied into the tables the list pages read. The copy keeps the span's ids:

| Span | Becomes |
|---|---|
| An HTTP `SERVER` span (or a root `INTERNAL` span with HTTP attributes) that is an entry point: no HTTP server span above it in the same export batch, so nested HTTP spans exported separately can each become an endpoint | An **endpoint** request (`endpoints-and-tasks.md`) |
| A `CONSUMER` span, or a root span with `console.command` | A **task** run |
| A span with `gen_ai.*` attributes, unless it already became an endpoint or task | An **AI trace** (`sessions-and-ai.md`) |
| An `exception` event (or `exception.*` attributes) on any span | An **issue** occurrence (`issues.md`) |
| Everything else | A span in the waterfall |

So an endpoint's waterfall is just the spans of its trace below its own span, and everything connected to it is whatever shares its trace id. An exception belongs to the nearest endpoint, task or AI trace above the span it happened on.

Two rules worth knowing when something seems missing:

- **Span reads are bound to 24 hours** either side of the entity being viewed. A span recorded further away is not shown on that page.
- **Orphans**: a span whose parent never arrived is shown under the trace's root entity, marked "Parent span is unavailable". It nests normally once the parent arrives.

## Spans page (`/spans`)

A search over every span of the project. Hidden in the sidebar for browser and mobile projects.

- **Filters**: span name (contains, case insensitive), service, kind, status (unset, ok, error), minimum and maximum duration, trace id, and up to 10 exact attribute matches (`key = value`).
- **Sort**: start time or duration, either direction.
- **Time range**: defaults to the last 60 minutes, and can be up to 31 days. A search that takes too long answers with a warning to narrow the range or add filters.
- The service filter lists the busiest 100 services in the range.

## The trace page (`/spans/<traceId>`)

The whole trace as one waterfall, across every service and **every project of the organization**, because the services of one trace usually report to different projects. Other organizations stay out.

- Each row names its service. Bars are coloured per service once a trace crosses services, and error spans are marked.
- Clicking a span opens its attributes, service, kind, status and scope. Attributes load when the popover opens.
- Large traces: over 500 spans, any node with more than 50 children starts folded, with a pill counting the hidden spans and an error marker when one of them failed. **Expand all** and **Collapse all** sit in the header. Over the row cap, the page keeps errors first, then entry points, then the slowest spans, and says the trace is partial.
- The trace's logs are listed alongside, from every project of the organization, with a service column.
- The URL needs `?at=` (any time near the trace) and can carry `?span=` to highlight one span.

## Distributed trace card

On endpoint request pages, task run pages and issue events, a **Distributed Trace** card lists every endpoint, task, AI trace and exception that shares the trace id, across the projects the user can open, nested by span parentage. It only appears when there is more than one such node. Each node has a project badge and a **View** button. A browser request and the backend request it called share the trace id through W3C `traceparent` propagation, so they appear together. For the whole span tree, use **Open the whole trace** on the waterfall.

Reading it from an agent: `traceway traces show <trace-id> --recorded-at <t>`, MCP `get_trace`. `--recorded-at` is the timestamp of the occurrence or node the trace id came from.

## Logs (`/logs`)

OpenTelemetry log records with a timestamp, severity, service (`service.name`), body, trace id and span id, and three attribute layers:

- **Resource** attributes describe the process (`service.version`, `host.name`, `deployment.environment`).
- **Scope** attributes describe the logger or instrumentation library.
- **Log** attributes belong to one record (`user.id`, `order.id`).

Severity is a number, grouped into levels: TRACE 1 to 4, DEBUG 5 to 8, INFO 9 to 12, WARN 13 to 16, ERROR 17 to 20, FATAL 21 and up. The severity dropdown is a threshold: `WARN+` keeps warnings and worse.

Filters, which all combine:

- **Search**, by **Message** (the body), **Service** or **Trace ID**. On ClickHouse a single-word body search is a fast whole-word match that is case sensitive (`timeout` does not find `Timeout` or `timeouts`). Anything longer, and every search on SQLite and DuckDB, is a case-insensitive substring match.
- **Severity**: All levels, or a threshold from TRACE+ to FATAL.
- **Attribute filters** (**Add filter**): any `resource.*`, `scope.*` or log attribute key, with equals, not equals, contains or not contains. Not equals is how teams mute a noisy logger. Each filter is a pill, red for exclusions. Expanding a log row lets you click its body, trace id, span id or scope to filter on them.
- **Time range**: presets or custom, 24 hours by default. Results are newest first by default (timestamp, level and service are sortable), and **Load 50 more** fetches more.

Filters are kept in the URL, so a filtered view can be shared.

**Live** turns the page into a live tail. It polls every 5 seconds, prepends new records and keeps up to 2000 rows. It works with preset ranges only, honours every filter, holds the scroll position while reading, and changing the sort stops it.

Structured message templates (Serilog, `Microsoft.Extensions.Logging` and others) are rendered inline: `{Provider}` in the body is replaced by the matching attribute and highlighted. A placeholder with no attribute stays as written.

**Logs and traces**: a log written inside an active span carries that span's trace and span ids. Every endpoint request, task run and AI trace page has a **Logs** panel with that trace's logs from the same project, and the trace page lists them from the whole organization.

Retention: 90 days on ClickHouse. Self-hosted SQLite and DuckDB keep 30 days by default, and an operator can also cap the row count.

## Reading it from an agent

| Task | CLI | MCP tool |
|---|---|---|
| Search logs | `traceway logs query --since 1h [--min-severity 17] [--service S] [--search text] [--trace-id T]` | `query_logs` |
| Logs of one request | `traceway logs query --trace-id <trace-id> --since 24h` | `query_logs` with `trace_id` |
| A whole distributed trace | `traceway traces show <trace-id> --recorded-at <t>` | `get_trace` |

The span explorer and the waterfall attributes popover are dashboard only. For one request's spans, open the endpoint or task request (`get_endpoint_request`, `get_task`), which returns its span waterfall.

A trace id is usually the fastest route to a root cause: one id gives the request, its downstream calls, its logs and its exceptions.
