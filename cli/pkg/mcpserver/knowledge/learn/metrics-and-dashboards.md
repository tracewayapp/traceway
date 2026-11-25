# Metrics and dashboards

## Metrics

A metric point is a name (`system.cpu.utilization`, `orders.created`), a numeric value, a timestamp and tags (`server_name`, `state`, `direction`, ...).

- **Where they come from**: OpenTelemetry meters in the application (counters, histograms, gauges), runtime instrumentation (Go, Node.js and others), and host metrics from the Traceway OTel Agent or an OpenTelemetry Collector with the hostmetrics receiver (`system.*` names: CPU, memory, disk, filesystem, network, processes).
- **Histograms** are stored as mergeable distributions, including OTLP exponential histograms. Explicit-bound backend histograms also retain `<name>.avg` and `<name>.count` series. Browser SDK 2.x metrics aggregate on-device; see [rum.md](rum.md).
- **Tags** come from the data point's attributes, plus an allowlist of resource attributes Traceway lifts: `host.name`, `host.id`, `os.type`, `cloud.region`, `container.name`, `k8s.cluster.name`, `k8s.pod.name`, `k8s.node.name`, `postgresql.database.name` and a few more. The `server_name` tag comes from `service.name`. The allowlist also covers `k8s.namespace.name`, `k8s.deployment.name`, `cloud.provider`, `cloud.availability_zone` and `process.*`.
- **Resolution**: on ClickHouse it follows the range so long ranges stay fast: raw points up to 6 hours, 1-minute rollups up to 72 hours, 1-hour rollups up to 30 days, daily beyond. SQLite and DuckDB always read raw points.
- **Stored types**: backend gauges, sums and histograms; summaries are dropped. Client projects accept catalog/custom delta sums and delta exponential histograms, not arbitrary gauges or cumulative series. Distribution rollups differ from point metrics: ClickHouse uses minute/hour/day storage; SQLite and DuckDB fold minutes older than six hours into hours.

Aggregations:

| Aggregation | Use |
|---|---|
| `avg`, `min`, `max`, `sum`, `count`, `last` | Gauges and per-bucket values. `max` is the honest choice when one bad series could hide behind healthy ones in an `avg`. |
| `rate` | Cumulative counters such as `system.network.io` and `system.disk.io`. Each series is differenced against its previous sample, resets are dropped, and the result is per second (`By` becomes `By/s`). Charting a counter as `avg` only climbs forever. |

A widget source can also be **inverted** (1 minus the value). The agent reports `system.cpu.utilization` per state, and `idle` inverted is the busy share.

Distribution queries support p50/p75/p90/p95/p99 and `good_ratio`; point metrics cannot supply percentiles. Merge buckets over the whole window before calculating a headline percentile, never average percentiles. Distribution `count` counts observations; delta-counter `sum` counts events. Query good ratios use 0..1; alert thresholds use 0..100. See [rum.md](rum.md) for example queries.

## Dashboards (`/dashboards`)

A dashboard is a set of metric widgets that belongs to the **organization**, not to one project. It is applied to any number of projects and shows as a tab on each of them. An edit made in one project is live in all of them.

The active tab's menu: **Rename Dashboard** (names are unique in the organization), **Edit as JSON**, **Export JSON**, **Share Dashboard**, **Apply to Projects**, **Remove from this Project** (it stays elsewhere), **Delete Dashboard** (everywhere, permanently).

**Adding**: ⌘K (Ctrl K) or **Add or find dashboards...** opens one search over the dashboards of every organization the user belongs to (selecting one applies it here, or copies it in from another organization), the template marketplace (selecting one installs a copy) and every metric name seen in the organization (selecting one opens the widget editor prefilled). An empty project shows **Browse Dashboards & Templates**.

**Templates**:

| Template | Metrics it expects |
|---|---|
| OTelemetry Server Agent | Host CPU, memory, disk and network from the Traceway OTel Agent |
| Go Application | Go runtime and process metrics |
| Traceway ClickHouse | `traceway.ch.*` from a monitored Traceway instance |
| Traceway DuckDB | `traceway.duckdb.*` from a monitored Traceway instance |

**Widgets**: Line Chart, Area Chart, Stacked Area, Bar Chart, Single Value, Gauge, Table, Rating Bar and Breakdown Table. Rating bars show rated distribution shares; breakdown tables compare groups over the whole range. Distribution headline values merge the full range first. Each has one or more sources: a metric name, an aggregation, optional tag filters (equals), an optional group-by that draws one series per tag value, and optionally inverted. Several sources overlay on one chart. Hover for values, drag across a chart to zoom in.

**Instance filter**: the picker in the header narrows every widget to one `server_name` seen in the range. It lives in the URL (`?server=<name>`), is not saved to the dashboard, and is where a row on the organization's Servers page leads.

**Starring**: the star on a widget pins it to the project's home page, where starred widgets can be resized and reordered. Stars are per project.

**As code**: every dashboard is one JSON document (`schemaVersion`, `name`, `widgets` in display order, each with `id`, `title`, `widgetType`, `config.sources`). Edit it in place, export it, or import a single dashboard or a whole organization bundle. The import API has an upsert mode that matches by name, so CI can keep dashboards in sync from files. Keep widget ids when editing so home page stars survive.

**Grafana import**: paste a Grafana export. Time series and graph panels become line charts, stat becomes single value, gauge stays a gauge, and Prometheus queries become metric sources. Anything that does not convert faithfully is listed as a warning, never silently dropped.

Editing dashboards and widgets needs an organization role above `readonly`. Starring, tab order and home page layout follow the effective project role. Browser projects receive Web Performance on creation; mobile projects offer Mobile Performance on the empty dashboard page. Client dashboards filter by release, OS and country (when configured), rather than server instance. Slow sessions opens retained distribution exemplars, which may have element attribution and session links.

## Sharing and embedding a dashboard

A shared dashboard is a read-only page at `/embed/dashboard` that needs no Traceway account and may be framed. The link signs the project, timeframe, filters and expiry.

For a one-off link, select the project and filters, then open **Share Dashboard** in the active tab’s menu. Set **Share for** to 1 to 90 days (default one) and **Timeframe** to a Last preset (default 42 hours). Choose **Generate Link**, check **Expires at**, then **Copy Link**. Each open resolves the timeframe against the server time and keeps all widgets on that window. Issuance requires effective project write access.

To embed KPIs in a customer admin panel, use the following flow.

1. After authorizing the viewer, the host backend calls `POST /api/dashboards/{numericDashboardId}/embed-token?projectId={projectUuid}`. The share dialog shows the endpoint. The dashboard must be applied to that project. Use a server-side personal access token whose owner can write to the project. A **Read only** token (`twp_`) is enough, since a link changes nothing: it needs no **Allow agent writes** and can be created without an expiry.
2. Choose the dashboard, project and filters on the server. For account-specific KPIs tagged with `account_id`, send a body such as `{"tagFilters":{"account_id":"acme"}}` using the signed-in viewer's authorized account. These filters apply to every widget source and override a saved filter on the same key; at most 20, and an empty tag or value is refused. With no range specified, each link shows the last 42 hours. Set `timeframe=30m`, `timeframe=42h` or `timeframe=7d` on the token request to choose the reporting window (positive whole minutes, hours or days, up to 731 days). For a fixed range, omit `timeframe` and send both `from` and `to` as RFC 3339 timestamps.
3. The **201** response contains `embedPath`, `expiresInSeconds`, `expiresAt`, `token` (the signed JWT) and `dashboardUuid` (stable per dashboard and project); the last two are already part of `embedPath`. Resolve `embedPath` against the browser-facing Traceway origin and send that URL and `expiresInSeconds` to the client with `Cache-Control: no-store`. Set the iframe's `src` to the complete URL, including its `#token=` fragment. Keep the personal access token on the backend.
4. Backend-issued links default to a five-minute lifetime and a window fixed at issuance. For a custom expiry supply either `expiresInDays` (whole days, 1 to 90) or `expiresAt` (future RFC 3339 timestamp, at most 90 days away) in the POST body. Relative timeframes on these links follow each viewer’s open time. Explicit timestamps stay fixed. To keep the panel visible, authorize the viewer again and assign a fresh URL to the same iframe shortly before the link expires; the dashboard is replaced in place. Traceway verifies the JWT on each read without storing an embed session. A single link cannot be revoked; removing the dashboard from the project stops every link for that project. The page receives widget titles and display settings only, never the filters saved on a widget, and reads are limited per client address (60 opens and 600 widget reads a minute, then 429 with `Retry-After`).

The page is dark by default. Add `&theme=light` or `&theme=dark` to the token request to receive a themed iframe URL. You can also set the theme directly in the iframe URL before its `#token=` fragment. Issuance answers 403 for a read-only token, a switched-off **Allow agent writes** or a token owner without write access to the project, 404 when the dashboard is not applied to the project, and 422 with a message for half a range, a reversed range, an expiry over 90 days or an empty filter.

Use the dashboard menu for interactive sharing and the HTTP endpoint for application integration. The write CLI and write MCP server issue the same link with `traceway-write dashboards embed <id>` and `create_dashboard_embed`; the read-only CLI and server cannot, and their dashboard exports return the widget definition. [Dashboard integration example](https://docs.tracewayapp.com/learn/dashboards#share-and-embed-a-dashboard)

## Reading it from an agent

| Task | CLI | MCP tool |
|---|---|---|
| A metric series | `traceway metrics query --name <metric> --aggregation avg --since 1h [--tag key=value] [--group-by tag] [--interval-minutes N]` | `query_metrics` |

- An unknown metric name returns an empty series, so probing names is safe.
- A grouped query keeps the first 200 groups and says `truncatedGroups: true` when it cut some.
- The server widens buckets so no series has more than 2000 points, and returns the bucket size it used as `intervalMinutes`.
- Both the CLI and MCP support `avg`, `sum`, `count`, `min`, `max` and `rate`. Point metrics also support `last`. Distributions also support `p50`, `p75`, `p90`, `p95`, `p99` and `good_ratio`. Inversion is a widget display setting.
- Use `--whole-range` in the CLI or `whole_range: true` in MCP for one value per series over the full window. Set the cutoff for `good_ratio` with `--threshold` or the query’s `threshold` field. Rated web vitals have a default cutoff.
- Use `traceway metrics discover --since 24h` for recorded names, types, units and tag keys; add `--name <metric> --key <tag>` for tag values. The MCP equivalent is `discover_metrics` with optional `name` and `key`.

Use `traceway-write dashboards import --file dashboard.json` to create and apply dashboards, or `--mode upsert` to replace by organization-wide name. Read `dashboard-building.md` in the `traceway-write` skill for chart selection from instrumented code, JSON examples and verification. Read-only `dashboards list` and `dashboards export <id>` are available in both CLIs.

MCP supports `list_dashboards`, `dashboard_library`, `export_dashboard` and, on the write server, `import_dashboards`. CLI `dashboards validate --file dashboard.json` validates offline with the same checks used by CLI and MCP imports.
