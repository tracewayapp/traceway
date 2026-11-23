package shared

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

const MaxOtelSearchAttributeFilters = 10

// OtelAttributeFilter selects spans by one attribute, with the operators of
// LogAttributeFilter: Contains switches to a case-insensitive substring match,
// Exclude negates (and then keeps spans without the attribute), and Exists
// matches on the key alone.
type OtelAttributeFilter struct {
	Key, Value                string
	Exclude, Contains, Exists bool
}

// OtelSpanSearch is one page of the span explorer. From and To are required: a search is always bounded by time.
type OtelSpanSearch struct {
	ProjectId                uuid.UUID
	From, To                 time.Time
	Service, Name, TraceId   string
	Kind, Status             *int32
	MinDuration, MaxDuration time.Duration
	Attributes               []OtelAttributeFilter
	OrderBy                  string
	Page, PageSize           int
}

// OtelSearchDialect holds what differs between the three backends. Every fragment uses ? placeholders.
type OtelSearchDialect struct {
	Project  func(uuid.UUID) any
	Time     func(time.Time) any
	Trace    func(project uuid.UUID, traceHex string) (string, any)
	NameLike string
	// AttributeValue is an attribute's value with the key as its one placeholder: '' or NULL when the span lacks it.
	AttributeValue string
	// AttributeHas is true when the span carries the key, its one placeholder.
	AttributeHas string
	// Contains wraps a text expression into a case-insensitive substring test, with the needle as its one placeholder.
	Contains    func(text string) string
	StartOrder  string
	ReadSetting string
	WinnerOrder string
	// Prefilter narrows the winner sort to the spans that match the filters.
	// Only ClickHouse sets it: that is where the sort was measured to run out
	// of memory, and DuckDB returns no rows for an attribute filter in this form.
	Prefilter bool
	Bucket    func(column string, seconds int) string
	Count     string
	CountIf   func(condition string) string
}

var otelSearchOrders = map[string]string{
	"start_time desc": "%s DESC, span_id DESC",
	"start_time asc":  "%s ASC, span_id ASC",
	"duration desc":   "duration DESC, %s DESC",
	"duration asc":    "duration ASC, %s DESC",
}

func otelSearchPredicate(search OtelSpanSearch, dialect OtelSearchDialect) (string, []any) {
	conditions := []string{"project_id = ?", "recorded_at >= ?", "recorded_at <= ?"}
	args := []any{dialect.Project(search.ProjectId), dialect.Time(search.From), dialect.Time(search.To)}
	add := func(condition string, values ...any) {
		conditions, args = append(conditions, condition), append(args, values...)
	}
	if search.TraceId != "" {
		condition, value := dialect.Trace(search.ProjectId, search.TraceId)
		add(condition, value)
	}
	if search.Service != "" {
		add("service_name = ?", search.Service)
	}
	if search.Name != "" {
		add(dialect.NameLike, search.Name)
	}
	if search.Kind != nil {
		add("span_kind = ?", *search.Kind)
	}
	if search.Status != nil {
		add("status_code = ?", *search.Status)
	}
	if search.MinDuration > 0 {
		add("duration >= ?", int64(search.MinDuration))
	}
	if search.MaxDuration > 0 {
		add("duration <= ?", int64(search.MaxDuration))
	}
	for _, filter := range search.Attributes {
		// COALESCE keeps the negated forms true for spans without the attribute,
		// where SQLite and DuckDB answer NULL (ClickHouse answers '').
		text := "COALESCE(" + dialect.AttributeValue + ", '')"
		switch {
		case filter.Exists && filter.Exclude:
			add("NOT ("+dialect.AttributeHas+")", filter.Key)
		case filter.Exists:
			add(dialect.AttributeHas, filter.Key)
		case filter.Contains && filter.Exclude:
			add("NOT ("+dialect.Contains(text)+")", filter.Key, filter.Value)
		case filter.Contains:
			add(dialect.Contains(text), filter.Key, filter.Value)
		case filter.Exclude:
			add(text+" != ?", filter.Key, filter.Value)
		default:
			add(dialect.AttributeValue+" = ?", filter.Key, filter.Value)
		}
	}
	return strings.Join(conditions, " AND "), args
}

// otelSearchSource is what a search reads, as a FROM clause: the matching spans, one retry version of each.
func otelSearchSource(search OtelSpanSearch, dialect OtelSearchDialect) (string, []any) {
	predicate, args := otelSearchPredicate(search, dialect)
	scope, scopeArgs := otelSearchPredicate(OtelSpanSearch{ProjectId: search.ProjectId, From: search.From, To: search.To, TraceId: search.TraceId}, dialect)
	// The winner of every span in the range is picked by a sort, and a filter on
	// the result makes that sort carry the filtered columns (the attribute map
	// among them) for all of those spans: on ClickHouse a day of 4M spans ran out
	// of the search's memory. So the sort only sees the spans that have a version
	// matching the filters; the filters are still applied to the winner.
	inner, innerArgs := scope, scopeArgs
	if dialect.Prefilter && len(args) > len(scopeArgs) {
		inner = scope + " AND (trace_id, span_id) IN (SELECT trace_id, span_id FROM " + SpansTable + " WHERE " + predicate + ")"
		innerArgs = append(append([]any{}, scopeArgs...), args...)
	}
	winners := OtelWinningRows("*", inner, dialect.WinnerOrder)
	return winners + " WHERE " + predicate, append(innerArgs, args...)
}

// OtelChartSource is what a span chart reads, as a FROM clause: the spans Search would match. An attribute chart
// reads only the spans that carry the attribute, which on ClickHouse also keeps the version sort to those spans.
func OtelChartSource(search OtelSpanSearch, attribute string, dialect OtelSearchDialect) (string, []any) {
	if attribute != "" {
		search.Attributes = append(append([]OtelAttributeFilter{}, search.Attributes...), OtelAttributeFilter{Key: attribute, Exists: true})
	}
	return otelSearchSource(search, dialect)
}

// OtelClickHouseAttributeChartSource is OtelChartSource for an attribute chart on ClickHouse, with the columns
// recorded_at and chart_raw. ClickHouse evaluates span_attributes[key] after the winner sort, so the generic source
// makes that sort carry the whole attribute map of every span with the attribute: a day of 4M spans that all carry it
// ran out of the search's memory, where the duration chart of the same spans did not. Here the value and the verdict
// of the filters are computed per stored version and close the sort order, which is what makes ClickHouse compute
// them before the sort. The order was total already (span_version is a digest of the span), so the winner is the same,
// and it is still the winner the filters are applied to.
func OtelClickHouseAttributeChartSource(search OtelSpanSearch, attribute string, dialect OtelSearchDialect) (string, []any) {
	scope, scopeArgs := otelSearchPredicate(OtelSpanSearch{ProjectId: search.ProjectId, From: search.From, To: search.To, TraceId: search.TraceId}, dialect)
	_, filterArgs := otelSearchPredicate(search, dialect)
	search.Attributes = append(append([]OtelAttributeFilter{}, search.Attributes...), OtelAttributeFilter{Key: attribute, Exists: true})
	predicate, args := otelSearchPredicate(search, dialect)
	inner, innerArgs := scope, scopeArgs
	// The chart's own attribute is no reason to narrow the sort: it no longer carries the map.
	if len(filterArgs) > len(scopeArgs) {
		inner = scope + " AND (trace_id, span_id) IN (SELECT trace_id, span_id FROM " + SpansTable + " WHERE " + predicate + ")"
		innerArgs = append(append([]any{}, scopeArgs...), args...)
	}
	source := "(SELECT recorded_at, chart_raw FROM (SELECT recorded_at, chart_raw, chart_match, ROW_NUMBER() OVER (PARTITION BY project_id, trace_id, span_id ORDER BY " +
		dialect.WinnerOrder + ", chart_match, chart_raw) AS otel_version FROM (SELECT project_id, trace_id, span_id, recorded_at, duration, span_version, " +
		dialect.AttributeValue + " AS chart_raw, toUInt8(" + predicate + ") AS chart_match FROM " + SpansTable + " WHERE " + inner +
		") versions) ranked WHERE otel_version = 1 AND chart_match) winners"
	return source, append(append([]any{attribute}, args...), innerArgs...)
}

// OtelSearchQueries returns the page query, which selects OtelTopologyColumns, and the matching count query.
func OtelSearchQueries(search OtelSpanSearch, dialect OtelSearchDialect) (page string, pageArgs []any, count string, countArgs []any) {
	order, known := otelSearchOrders[search.OrderBy]
	if !known {
		order = otelSearchOrders["start_time desc"]
	}
	order = strings.ReplaceAll(order, "%s", dialect.StartOrder)
	order += ", trace_id, span_id"
	source, args := otelSearchSource(search, dialect)
	page = "SELECT " + OtelTopologyColumns + " FROM " + source + " ORDER BY " + order + " LIMIT ? OFFSET ?" + dialect.ReadSetting
	pageArgs = append(append([]any{}, args...), search.PageSize, (search.Page-1)*search.PageSize)
	return page, pageArgs, "SELECT count(*) FROM " + source + dialect.ReadSetting, args
}

// OtelServicesQuery lists the services that reported spans in the range, busiest first, for the explorer's filter.
func OtelServicesQuery(project uuid.UUID, from, to time.Time, dialect OtelSearchDialect) (string, []any) {
	return "SELECT service_name FROM " + SpansTable + " WHERE project_id = ? AND recorded_at >= ? AND recorded_at <= ? AND service_name != '' GROUP BY service_name ORDER BY count(*) DESC LIMIT 100" + dialect.ReadSetting,
		[]any{dialect.Project(project), dialect.Time(from), dialect.Time(to)}
}
