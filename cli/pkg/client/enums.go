package client

// Enum values accepted by the list/query endpoints. These mirror the backend
// contract and are shared by the CLI flag validation and the MCP server's
// input validation, so the two surfaces cannot drift.
var (
	ExceptionsOrderByValues = []string{"lastSeen", "firstSeen", "count"}
	ExceptionsSearchTypes   = []string{"text", "regex"}
	LogsSearchTypes         = []string{"body", "attribute"}
	SortDirections          = []string{"asc", "desc"}
	EndpointsOrderByValues  = []string{"impact", "count", "p95", "lastSeen"}
	// EndpointRequestsOrderByValues sort one endpoint's requests.
	EndpointRequestsOrderByValues = []string{"duration", "recordedAt", "statusCode"}
	EndpointChartMetricTypes      = []string{"total_time", "p50", "p95", "p99"}
	SessionsOrderByValues         = []string{"startedAt", "duration"}

	// MetricAggregations are the values the server accepts on metrics/query and
	// in dashboard sources. The percentiles and good_ratio read a distribution
	// (histogram) metric, such as browser.web_vital.inp or an OTLP histogram; for
	// a point metric the server answers 422 instead of a number, and a
	// distribution answers 422 to last.
	MetricAggregations = []string{"avg", "sum", "count", "min", "max", "rate", "last", "p50", "p75", "p90", "p95", "p99", "good_ratio"}

	// DashboardWidgetTypes mirror the server's dashboards.AllowedWidgetTypes.
	DashboardWidgetTypes = []string{"line_chart", "area_chart", "stacked_area", "bar_chart", "single_value", "gauge", "table", "rating_bar", "breakdown_table"}
)

// orderByColumns maps the camelCase order-by values above to the column names
// the backend sorts on. The backend falls back to its default order for any
// name it does not know, so sending "p95" or "lastSeen" as is silently did
// not sort.
var orderByColumns = map[string]string{
	"p95":        "p95_duration",
	"lastSeen":   "last_seen",
	"firstSeen":  "first_seen",
	"recordedAt": "recorded_at",
	"statusCode": "status_code",
	"startedAt":  "started_at",
}

func orderByColumn(v string) string {
	if c, ok := orderByColumns[v]; ok {
		return c
	}
	return v
}
