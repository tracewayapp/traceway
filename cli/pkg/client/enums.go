package client

// Enum values accepted by the list/query endpoints. These mirror the backend
// contract and are shared by the CLI flag validation and the MCP server's
// input validation, so the two surfaces cannot drift.
var (
	ExceptionsOrderByValues  = []string{"lastSeen", "firstSeen", "count"}
	ExceptionsSearchTypes    = []string{"text", "regex"}
	LogsSearchTypes          = []string{"body", "attribute"}
	SortDirections           = []string{"asc", "desc"}
	EndpointsOrderByValues   = []string{"impact", "count", "p95", "lastSeen"}
	EndpointChartMetricTypes = []string{"total_time", "p50", "p95", "p99"}

	// MetricAggregations are the values the server accepts on metrics/query.
	// The server has no quantile aggregation for metric points: p50/p95/p99
	// are silently computed as avg. The CLI accepts them for parity with the
	// dashboard; the MCP server rejects them (see MetricAggregationsExact).
	MetricAggregations = []string{"avg", "sum", "count", "min", "max", "p50", "p95", "p99"}

	// MetricAggregationsExact are the aggregations the server actually
	// computes as named. Latency percentiles come from the endpoints
	// endpoints, never from metrics/query.
	MetricAggregationsExact = []string{"avg", "sum", "count", "min", "max"}
)
