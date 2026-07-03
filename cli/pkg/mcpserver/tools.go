package mcpserver

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// addTools registers the full tool surface. Every tool wraps exactly one
// pkg/client method; the descriptions carry the operating discipline that
// the knowledge resources explain in depth.
func (s *server) addTools(srv *mcp.Server) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_projects",
		Description: "List the projects the authenticated user can access. Use it to find a project_id and as a cheap auth check.",
		Annotations: readOnly(),
	}, s.listProjects)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_exceptions",
		Description: "List grouped exceptions (error groups) in a time window. Groups are keyed by a 16-hex-char hash of the normalized stack trace and carry count, firstSeen/lastSeen, and an hourly trend. firstSeen right after a deploy points at that release. Empty data is not an error: widen the window or check the project.",
		Annotations: readOnly(),
	}, s.listExceptions)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_exception",
		Description: "Get one exception group by its 16-hex hash: the representative stack trace plus paginated occurrences, each with recordedAt, attributes, and optional distributedTraceId/sessionId. A hash can bundle several distinct errors that share top frames: when debugging, anchor on the most recent occurrence, not the group; distinct first stack lines mean distinct bugs. Capture each occurrence's id together with its recordedAt for the by-id drill-in tools.",
		Annotations: readOnly(),
	}, s.getException)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_exception_occurrence",
		Description: "Get a single exception occurrence by UUID, including its sessionId and session recording reference. Prefer this over get_exception when you already hold the occurrence id and its timestamp (dashboard URL path + ?t= param, or a notification's Exception ID + Occurred at).",
		Annotations: readOnly(),
	}, s.getExceptionOccurrence)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "query_logs",
		Description: "Search log records by window, service, severity, text, or trace. min_severity is an OTel severity number, never a name: 1 TRACE, 5 DEBUG, 9 INFO, 13 WARN, 17 ERROR, 21 FATAL; use 17 for errors and worse. trace_id returns one request's logs and is often the fastest route to a root cause.",
		Annotations: readOnly(),
	}, s.queryLogs)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_endpoints",
		Description: "Per-endpoint latency stats in a window: request count, p50/p95/p99/avg durations (nanoseconds), impact score with reason. This is the ONLY source of true latency percentiles (computed from raw request durations; query_metrics cannot do quantiles). Read the shape first: high p50 means every request pays the cost (systemic), p99 much greater than p50 means a tail (contention, pool exhaustion, GC, retries). Raw percentiles are not adjusted for operator-accepted slowness; check get_slow_endpoint_config before calling an endpoint slow.",
		Annotations: readOnly(),
	}, s.listEndpoints)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_endpoint_request",
		Description: "Get one request (transaction) by id: the request, its span waterfall, and any linked exception/messages. In the waterfall, a gap before work starts is itself the finding: time spent queueing, waiting on a lock, or acquiring a pool connection.",
		Annotations: readOnly(),
	}, s.getEndpointRequest)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "endpoints_chart",
		Description: "Endpoint latency bucketed over time: the top 5 endpoints ranked by metric_type (plus an Other bucket), each as a {timestamp, endpoint, value} series in milliseconds. Use it to pinpoint WHEN latency changed: read down one endpoint's buckets for the step. A non-top-5 endpoint folds into Other; bisect list_endpoints over narrowing from/to windows for it instead.",
		Annotations: readOnly(),
	}, s.endpointsChart)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_slow_endpoint_config",
		Description: "Whether an operator marked an endpoint as expected-slow: offsetMs is the accepted latency allowance and reason the operator's note; offsetMs 0 means not marked. The offset shifts apdex/impact thresholds server-side but raw p50/p95/p99 are never offset-adjusted, so compare p95 against offsetMs, not against zero. Check this before reporting an endpoint as slow.",
		Annotations: readOnly(),
	}, s.getSlowEndpointConfig)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "query_metrics",
		Description: "Query time-bucketed metric series (aggregations: avg, sum, count, min, max). There is NO quantile aggregation: latency percentiles come from list_endpoints only. Host metrics from the Traceway OTel Agent live under system.* names; runtime names include go.gc_pause, mem.used_pcnt, go.go_routines. OTLP histogram metrics are stored as two series, <name>.avg and <name>.count. An unknown name returns an empty series cleanly, so probing names is safe.",
		Annotations: readOnly(),
	}, s.queryMetrics)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_task",
		Description: "Get one background task run by id: the task, its span waterfall, and any linked exception/messages.",
		Annotations: readOnly(),
	}, s.getTask)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_ai_trace",
		Description: "Get one AI trace by id: token and cost stats plus the stored conversation.",
		Annotations: readOnly(),
	}, s.getAiTrace)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_session",
		Description: "Get one user session by id plus the exceptions that fired during it. Session replay stays in the dashboard. Note the timestamp param is started_at (sessions are partitioned by start time); a linked occurrence's recordedAt falls inside the window and works.",
		Annotations: readOnly(),
	}, s.getSession)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_trace",
		Description: "Get a distributed trace by id: every endpoint/task/AI-trace/exception node sharing the trace id, across services and across all projects the user can see (no project_id param). Usually the single highest-value root-cause call: it stitches one logical request together end to end. The lookup window is 48h around recorded_at.",
		Annotations: readOnly(),
	}, s.getTrace)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "archive_exceptions",
		Description: "Archive exception groups by hash so they leave the default issue list. MUTATING (reversible via unarchive_exceptions). Only call when the user explicitly asked to archive; investigating or debugging an error never implies archiving it.",
		Annotations: mutating(),
	}, s.archiveExceptions)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "unarchive_exceptions",
		Description: "Unarchive previously archived exception groups by hash. MUTATING. Only call when the user explicitly asked for it.",
		Annotations: mutating(),
	}, s.unarchiveExceptions)
}
