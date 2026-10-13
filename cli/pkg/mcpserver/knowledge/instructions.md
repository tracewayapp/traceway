# Traceway MCP server

Query and debug a Traceway observability instance: exceptions, logs, endpoints, background tasks, AI traces, sessions, distributed traces, metrics, monitors, alert rules, on-call pages and post-mortems.

Ground rules:

- Reads are safe: the read tools never mutate server state. A plain server has no write tools at all. Changes (archiving issues, acknowledging pages, snoozing alerts, editing post-mortems) need the write server, which a user connects separately with a write credential.
- Always bound queries in time. Default to since "1h" for "now" questions and "24h" otherwise. since accepts s, m, h and lowercase Nd (no "1w", no "7d2h"); absolute windows use from/to as RFC3339.
- The by-id detail tools (get_exception_occurrence, get_endpoint_request, get_task, get_ai_trace, get_session, get_trace) require the record's timestamp. It can be approximate (within a day); recover it from a dashboard URL's ?t= param, an occurrence list, or a notification before guessing. Never pass "now" for an old record.
- Log severity is an OTel number, not a name: 1 TRACE, 5 DEBUG, 9 INFO, 13 WARN, 17 ERROR, 21 FATAL. Use min_severity 17 for errors and worse.
- Latency percentiles (p50/p95/p99) come only from list_endpoints and endpoints_chart. query_metrics has no quantile aggregation.
- Keep page_size at 10 to 20 for triage; raise it only when you need the full set.
- Empty results are not errors: widen the window, check the project (list_projects), and consider that the app may not be connected to Traceway yet.

For deep work, read the knowledge resources first: traceway://knowledge/debug-flow before debugging an issue to root cause, traceway://knowledge/performance before any latency investigation, traceway://knowledge/url-resolution when the user pastes a dashboard URL, and traceway://knowledge/notifications when resolving an alert notification. traceway://knowledge/learn is the product guide: how every part of Traceway works (issues, endpoints, traces, logs, metrics, monitors, status pages, alerts, on-call schedules and paging, settings), with one traceway://knowledge/learn/<topic> resource per area. Read it when the user asks how something works or where to find it.
