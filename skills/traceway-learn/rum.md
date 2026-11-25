<!-- GENERATED FILE: copied from cli/pkg/mcpserver/knowledge/learn/rum.md by cli/tools/skillgen. Edit there and run just gen-skills in cli/. -->
# Real User Monitoring (RUM)

Use this reference for browser measurement, release investigations, performance dashboards, frontend SLOs and collection policy. RUM connects population metrics to individual sessions, source-mapped errors, replay and backend traces. It does not require full-session recording.

## Check capabilities before promising coverage

- Browser projects are React, Vue, Svelte and jQuery, separate from backend and mobile projects. Browser SDK 1.x and 2.x have different capabilities; npm `latest` is 2.x. Check the installed package/lockfile and session SDK version, the server version, and live npm dist-tags for both frontend and its wrapper before upgrading. Never assume a dist-tag or a local source checkout says what the app runs.
- Stable browser SDK 2.0.0 and later supply independent device-aggregated metrics. Browser SDK 1.x has no performance metrics. An early browser `2.0.0-beta.N` prerelease supplies only performance events, and only uploaded events become metrics. If the installed version lacks the required feature, report that limitation and the upgrade it needs (1.x to 2.x is a migration: https://docs.tracewayapp.com/client/js-sdk/migration); do not silently substitute source code or promise complete RUM from an older SDK.
- The server accepts mobile `app.*` OTLP metrics. The Flutter SDK 2.x (the 2.x prerelease, `flutter pub add 'traceway:{"git":{"url":"https://github.com/tracewayapp/traceway-flutter.git","ref":"v2-session-capture"}}'`; 1.x sends none) aggregates them on the device and applies `metrics.enabled`, `sampleRate` and `flushSeconds`: app start, screen display times, slow and frozen frames, hangs, unhandled errors (`app.crash`), sessions, users, peak memory and requests, and a Flutter project receives the **Flutter Performance** dashboard on creation. React Native, Android and iOS SDKs do not produce them yet; an available Mobile Performance template does not prove instrumentation exists. Check the app's `pubspec.lock` for the installed Flutter SDK version.

## Measurement and diagnostic detail are separate

SDK 2.x (stable 2.0.0 and later) aggregates distributions as delta exponential histograms and counters as delta sums, sent to `/api/otel/v1/metrics` when the page hides, the session ends, or every 300 seconds. The server merges cells and flushes every 10 seconds by default. Healthy visits contribute without uploading diagnostic breadcrumbs or replay. Event-derived metrics from a browser `2.0.0-beta.N` prerelease instead reflect the events actually sent; do not compare those samples as if they covered every visit.

Detailed console lines, navigation, clicks, requests and replay normally buffer locally. The breadcrumb ring holds 10 minutes or 2 MiB, replay up to 8 MiB. Breadcrumb persistence across page loads is best effort; replay stays in memory. An error uploads preceding breadcrumbs and a 30-second clip (5-second cooldown, 20 clips/session). Recording triggers and targeted captures provide additional evidence. Streamed or captured diagnostics are selected samples, not request-volume denominators.

| Measurement | Metric / interpretation |
|---|---|
| Page-load Web Vitals | `browser.web_vital.lcp`, `.inp`, `.fcp`, `.ttfb` in ms; `.cls` is a score. Page-level vitals belong to the landing route |
| SPA experience | `browser.view.inp`, `browser.view.cls` per view (`view_type`: load/soft), and `browser.route_timing.duration` |
| Loading and main thread | `browser.page_load.duration`, `.dom_content_loaded`, `browser.page.transfer_size`, `browser.long_task.duration`, `.blocking_time` |
| Requests | `browser.request.duration` by host; `browser.request.failed` and `.total` are counters forming a ratio |
| Traffic and reliability | `browser.page_view`, `browser.session`, `browser.js_error`, `browser.rage_click` are delta counters |
| Attribution | `browser.web_vital.attribution` by target/vital/rating; `browser.long_task.script_time` by script where browser support exists |
| Affected population | `browser.user` and `browser.user.affected` count browser profiles once per UTC day, affected counts per issue category. They require browser storage. Across days these are user-days, not globally distinct people |
| App-specific measurements | `metric(name, value, dims?)` for distributions; `count(name, n?, dims?)` for delta counts |

Use `setRoute('/orders/:id')` or the framework route helpers (`useTracewayRoute`, `trackRouter`, `trackRoute`). Browser SDKs 2.0.1 and later read the template from the router with no code for React Router framework mode, Remix, TanStack Router, the Next.js Pages Router, Vue Router and Nuxt; a `createBrowserRouter` router is passed once (`trackRouter(router)` or the provider's `router` prop); React Router `<Routes>` and the Next.js App Router use `useTracewayRoute(pathname, params)`; SvelteKit uses `trackRoute(page.route.id)`. Automatic normalization replaces ID-like segments and segments with a percent-encoded ASCII character, but route templates give meaningful route boundaries. A `url.path` value per order, user or encoded name means a route is still unnamed. Never use raw URLs, customer IDs or email addresses as metric dimensions.

Dimensions include release (`service.version`), environment (`deployment.environment.name`), route (`url.path`), browser/OS, device form factor/class, network, country, and two `custom.<key>` dimensions set by `setMetricDimension` or `metricDimensions`. Custom metrics allow 50 names/project with two dimensions each and 50 values/dimension. Cardinality caps fold excess values into `other`; per metric the daily series budget is 5,000. Attribution target/script also consume that budget. A dimension absent at collection cannot be added to historical data.

Country requires operator-configured `GEOIP_COUNTRY_HEADER` behind a proxy that overwrites it, or `GEOIP_DB_PATH` with correct trusted proxies. No source means no country filter. Metric dimensions carry no session/user/device identity; retained exemplars are a separate diagnostic link to sessions.

## Investigate a regression

1. Select the browser project, environment and a representative range. Open **Web Performance**, assigned when a browser project is created. Empty charts mean no matching observations, not good performance.
2. Compare release cohorts with observation counts. Use p75 for Web Vitals, p95 for the tail; inspect route, device, OS and country breakdowns. Check whether a changed population explains a changed percentile.
3. Use **Rating Bar** for good/needs-improvement/poor shares, **Breakdown Table** for whole-window groups, and a line chart for trends. Merge histogram buckets before calculating a window percentile; never average percentile buckets. Use distribution `count` for observations and delta-counter `sum` for events. Counter `count` counts stored points.
4. Open **Slow sessions** on a distribution widget when exemplars exist; inspect the responsible element, session timeline, errors and network, then replay if captured. Missing exemplars or replay do not invalidate the aggregate. The Sessions page can filter recording reasons; historical trigger names/sequences remain available after edits.
5. Follow request trace IDs into backend work. Cross-origin correlation needs allowed `traceparent` propagation and backend CORS plus OTel extraction. Verify source maps from the same production build resolve the issue. A session id alone is not proof that an exception belongs to a particular request.

## Objectives and alerts

**Metric Threshold** supports distributions and **Metric Release Regression** compares the newest release with the busiest earlier one after enough observations. Release comparisons adjust the baseline to the new device/OS mix by default; **Compare Within** compares matching segments. Percentile regression checks use non-overlapping approximate 95% intervals and effective sample size, not scaled event counts. One observation weighted to 1,000 is not 1,000 independent observations.

Threshold rules evaluate data older than two minutes. Percentiles and good ratios require at least 10 effective observations by default (**Minimum Observations**); choose more for paging/tail percentiles. Volume aggregations do not use that gate. No data is not a threshold breach.

In **SLOs** (`/slos`), choose **Metric → Every observation** for a distribution: for example INP ≤ 200 ms with a product-specific target and rolling window. **One SLO per** creates separate budgets for up to 20 dimension values. Threshold counts are approximate; missing observations are neither good nor bad, and a page without interaction has no INP sample. Distribution SLOs cannot treat empty slices as failures. Keep a separate crash-free-session objective and verify session reporting. Use multi-window burn-rate alerts for sustained budget consumption; **Meeting**, **Breached**, **No data** and **Paused** are distinct states.

## Collection policy

Keep metrics enabled at sample rate 1 unless the requested policy needs sampling; keep input masking and buffered diagnostics by default. `recordAllSessions` is not needed for population metrics. Remote `metrics.enabled`, `metrics.sampleRate` (0 or 0.001..1) and `metrics.flushSeconds` (30..3600, default 300) control aggregated metrics independently of `signals`, `sessions.baselineSampleRate` and replay sampling. Sampled counts are inverse-weighted estimates, not billing-grade totals. `captureMetrics: false` in init is a developer ceiling; remote rules cannot override it. The global kill switch stops collection, including captures.

For collection changes, inspect rules in order, preserve unrelated fields, target the requested population and preview saved settings. Preview does not establish that a browser polled or applied them. Confirm `traceway.config.applied` and actual uploads. Triggers see records even when their signal mode is off; they can flush, stream, start replay, emit or capture. Captures target a user/device/session/attribute for 1 minute to 24 hours (20 active/project) and take effect on polling. Review masking and the app's consent policy before collecting detail. Captures and replay viewing remain dashboard-only for agents.

## Agent capabilities and query semantics

Discover before querying: `traceway metrics discover --since 24h`, then `--name browser.web_vital.inp --key service.version` for release values. MCP has `discover_metrics`. Read sessions with `sessions show <id> --started-at <timestamp>` / `get_session`; no session-list CLI exists.

`traceway metrics query` and MCP `query_metrics` take p50/p75/p90/p95/p99 and `good_ratio` on distributions, with `--threshold`/`threshold` for `good_ratio` and `--whole-range`/`whole_range` for one value over the whole window. A window's percentile cannot be put together from its buckets' percentiles, so read what an alert saw ("INP p75 over 24 hours") with `--whole-range`, not by averaging buckets. A query the server cannot answer (a percentile of a point metric, `last` of a distribution, `good_ratio` without a threshold on an unrated metric) fails the whole request with 422 and its reason; an empty series means no data. For example:

```bash
traceway metrics query --name browser.web_vital.inp --aggregation p75 --whole-range --since 24h --group-by service.version
traceway metrics query --name browser.web_vital.inp --aggregation good_ratio --threshold 200 --whole-range --since 24h
```

The current server accepts p50/p75/p90/p95/p99 and `good_ratio` on distributions. `POST /api/metrics/query?projectId=PROJECT_UUID` accepts UTC `from`/`to`, `queries`, and `wholeRange: true` to merge the full range:

```json
{
  "queries": [
    {"name":"browser.web_vital.inp","aggregation":"p75","groupBy":"service.version"},
    {"name":"browser.web_vital.inp","aggregation":"count","groupBy":"service.version"},
    {"name":"browser.web_vital.inp","aggregation":"good_ratio","threshold":200,"groupBy":"service.version"}
  ],
  "from":"2026-11-16T00:00:00Z",
  "to":"2026-11-17T00:00:00Z",
  "wholeRange":true
}
```

Replace the example dates with the requested range. Use `tagFilters` for environment/population. For trends omit `wholeRange` and optionally set `intervalMinutes`. Inspect errors and truncation. `good_ratio` query results are fractions (0..1); alert rule thresholds are percentages (0..100). Percentiles require distributions; point metrics cannot yield them.

Dashboard import in the write CLI/MCP accepts percentile and `good_ratio` sources and the `rating_bar`/`breakdown_table` widgets, so the Web Performance layout can be exported, edited and imported; preserve existing widget IDs. Exemplars use the dashboard. Read SLOs with `traceway slos list` / `slos show <id>` (`list_slos`, `get_slo`) and try a definition with `slos preview`; creating, changing and deleting them needs `traceway-write slos` (`create_slo`, `update_slo`, `delete_slo`) with its normal permission checks, like alert and OpAMP writes. A distribution SLO is `sliKind: metric` with `aggregation: distribution`.

## Verify delivery and operate it

Check package/server compatibility, release/environment metadata and route templates first. Generate real browser interactions and a soft navigation, hide the page to flush, allow the server flush interval, then check Web Performance and discovery in the same project/range. No interaction means no INP. Verify production source maps, one error clip and trace linkage separately. State which checks could not run; never enable full recording merely to populate a chart.

For self-hosted delivery problems inspect `/api/health/deep`'s `clientMetrics` block and `traceway.client_metrics.*`: requests, observations, pending cells, last flush and drops. Client metrics have their own admission gate, a 64 KiB payload cap and 2,000 points/request. `CLIENT_METRICS_FLUSH_SECONDS`, `CLIENT_METRICS_MAX_CELLS`, `CLIENT_METRICS_MAX_CONCURRENT` and `CLIENT_METRICS_DEDUP_MB` govern flush cadence, memory, admission and retry deduplication. A process crash can lose unflushed cells; deduplication is per instance, so a retry routed elsewhere can double-count. Late data can repair charts after alert evaluation. ClickHouse rolls minutes into hours/days; SQLite/DuckDB fold minutes older than six hours into hours under telemetry retention.
