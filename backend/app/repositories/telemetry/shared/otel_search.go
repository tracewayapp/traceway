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
	Bucket      func(column string, seconds int) string
	Count       string
	CountIf     func(condition string) string
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
	scope, scopeArgs := otelSearchScope(search, dialect)
	return OtelWinningRows("*", scope, dialect.WinnerOrder) + " WHERE " + predicate, append(scopeArgs, args...)
}

func otelSearchScope(search OtelSpanSearch, dialect OtelSearchDialect) (string, []any) {
	return otelSearchPredicate(OtelSpanSearch{ProjectId: search.ProjectId, From: search.From, To: search.To, TraceId: search.TraceId}, dialect)
}

// OtelChartSource is what a span chart reads, as a FROM clause: the spans Search would match. An attribute chart
// reads only the spans that carry the attribute.
func OtelChartSource(search OtelSpanSearch, attribute string, dialect OtelSearchDialect) (string, []any) {
	if attribute != "" {
		search.Attributes = append(append([]OtelAttributeFilter{}, search.Attributes...), OtelAttributeFilter{Key: attribute, Exists: true})
	}
	return otelSearchSource(search, dialect)
}

// otelClickHouseMatchingSpans is a FROM clause over the winning version of every span the search matches, on
// ClickHouse, with the columns project_id, trace_id, span_id, duration, the extra plain columns, and chart_raw when
// value (an expression whose placeholders valueArgs fill) is given.
//
// The generic source picks the winner of every span in the range with a sort, which costs memory in proportion to
// every span in the range (about 633 MB for 4M spans with nothing but the keys in it), although a span is stored
// more than once only when an export was retried, which is rare. So the spans stored more than once are found
// first, by grouping a 64-bit hash of their identity (about 135 MB for 4M spans). Every other span has one version,
// which is its winner, and is filtered as it is read. Only the spans with several versions go through the winner
// sort. A hash collision only sends a span through that sort, so the result stays exact.
//
// In the sort, the verdict of the filters and the value are computed per stored version and close the order, which
// is what makes ClickHouse compute them before it (it evaluates an expression the order does not read after the
// sort, which would make the sort carry the attribute map). The order was total already (span_version is a digest of
// the span), so the winner is the same, and the filters are still applied to the winner.
//
// The filters are an explicit PREWHERE on the single versions: left to itself, ClickHouse moves the hash test into
// PREWHERE ahead of them and hashes every span in the range before a selective filter has removed any.
func otelClickHouseMatchingSpans(search OtelSpanSearch, extra []string, value string, valueArgs []any, dialect OtelSearchDialect) (string, []any) {
	scope, scopeArgs := otelSearchScope(search, dialect)
	predicate, args := otelSearchPredicate(search, dialect)
	columns := append([]string{"project_id", "trace_id", "span_id", "duration"}, extra...)
	selected := strings.Join(columns, ", ")
	order := dialect.WinnerOrder + ", otel_match"
	if value != "" {
		selected += ", " + value + " AS chart_raw"
		columns = append(columns, "chart_raw")
		order += ", chart_raw"
	}
	duplicated := "(SELECT cityHash64(trace_id, span_id) FROM " + SpansTable + " WHERE " + scope +
		" GROUP BY cityHash64(trace_id, span_id) HAVING count() > 1)"
	single := "SELECT " + selected + " FROM " + SpansTable + " PREWHERE " + predicate +
		" WHERE cityHash64(trace_id, span_id) NOT IN " + duplicated
	several := "SELECT " + strings.Join(columns, ", ") + " FROM (SELECT *, ROW_NUMBER() OVER (PARTITION BY project_id, trace_id, span_id ORDER BY " +
		order + ") AS otel_version FROM (SELECT " + selected + ", span_version, toUInt8(" + predicate + ") AS otel_match FROM " + SpansTable +
		" PREWHERE " + scope + " WHERE cityHash64(trace_id, span_id) IN " + duplicated + ") versions) ranked WHERE otel_version = 1 AND otel_match"
	var all []any
	for _, part := range [][]any{valueArgs, args, scopeArgs, valueArgs, args, scopeArgs, scopeArgs} {
		all = append(all, part...)
	}
	return "(" + single + " UNION ALL " + several + ") spans", all
}

// OtelClickHouseChartSource is OtelChartSource on ClickHouse, with the columns recorded_at and duration, plus
// chart_raw, the attribute's value, for an attribute chart.
func OtelClickHouseChartSource(search OtelSpanSearch, attribute string, dialect OtelSearchDialect) (string, []any) {
	if attribute == "" {
		return otelClickHouseMatchingSpans(search, []string{"recorded_at"}, "", nil, dialect)
	}
	search.Attributes = append(append([]OtelAttributeFilter{}, search.Attributes...), OtelAttributeFilter{Key: attribute, Exists: true})
	return otelClickHouseMatchingSpans(search, []string{"recorded_at"}, dialect.AttributeValue, []any{attribute}, dialect)
}

func otelSearchOrder(search OtelSpanSearch, dialect OtelSearchDialect) string {
	order, known := otelSearchOrders[search.OrderBy]
	if !known {
		order = otelSearchOrders["start_time desc"]
	}
	return strings.ReplaceAll(order, "%s", dialect.StartOrder) + ", trace_id, span_id"
}

// OtelSearchQueries returns the page query, which selects OtelTopologyColumns, and the matching count query.
func OtelSearchQueries(search OtelSpanSearch, dialect OtelSearchDialect) (page string, pageArgs []any, count string, countArgs []any) {
	order := otelSearchOrder(search, dialect)
	source, args := otelSearchSource(search, dialect)
	page = "SELECT " + OtelTopologyColumns + " FROM " + source + " ORDER BY " + order + " LIMIT ? OFFSET ?" + dialect.ReadSetting
	pageArgs = append(append([]any{}, args...), search.PageSize, (search.Page-1)*search.PageSize)
	return page, pageArgs, "SELECT count(*) FROM " + source + dialect.ReadSetting, args
}

// OtelClickHouseSearchQueries is OtelSearchQueries on ClickHouse, over otelClickHouseMatchingSpans. The page is found
// in two steps: the spans of the page are chosen by what the page order reads, and their topology is read afterwards,
// for those spans only.
func OtelClickHouseSearchQueries(search OtelSpanSearch, dialect OtelSearchDialect) (page string, pageArgs []any, count string, countArgs []any) {
	order := otelSearchOrder(search, dialect)
	keys, keyArgs := otelClickHouseMatchingSpans(search, []string{"start_time_unix_nano"}, "", nil, dialect)
	scope, scopeArgs := otelSearchScope(search, dialect)
	selected := scope + " AND (trace_id, span_id) IN (SELECT trace_id, span_id FROM " + keys + " ORDER BY " + order + " LIMIT ? OFFSET ?)"
	page = "SELECT " + OtelTopologyColumns + " FROM " + OtelWinningRows(OtelTopologyColumns, selected, dialect.WinnerOrder) + " ORDER BY " + order + dialect.ReadSetting
	pageArgs = append(append(append([]any{}, scopeArgs...), keyArgs...), search.PageSize, (search.Page-1)*search.PageSize)
	return page, pageArgs, "SELECT count(*) FROM " + keys + dialect.ReadSetting, keyArgs
}

// OtelServicesQuery lists the services that reported spans in the range, busiest first, for the explorer's filter.
func OtelServicesQuery(project uuid.UUID, from, to time.Time, dialect OtelSearchDialect) (string, []any) {
	return "SELECT service_name FROM " + SpansTable + " WHERE project_id = ? AND recorded_at >= ? AND recorded_at <= ? AND service_name != '' GROUP BY service_name ORDER BY count(*) DESC LIMIT 100" + dialect.ReadSetting,
		[]any{dialect.Project(project), dialect.Time(from), dialect.Time(to)}
}
