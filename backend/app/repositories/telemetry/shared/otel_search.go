package shared

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

const MaxOtelSearchAttributeFilters = 10

type OtelAttributeFilter struct{ Key, Value string }

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
	Project     func(uuid.UUID) any
	Time        func(time.Time) any
	Trace       func(project uuid.UUID, traceHex string) (string, any)
	NameLike    string
	Attribute   string
	StartOrder  string
	ReadSetting string
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
		add(dialect.Attribute, filter.Key, filter.Value)
	}
	return strings.Join(conditions, " AND "), args
}

// OtelSearchQueries returns the page query, which selects OtelTopologyColumns, and the matching count query.
func OtelSearchQueries(search OtelSpanSearch, dialect OtelSearchDialect) (page string, pageArgs []any, count string, countArgs []any) {
	predicate, args := otelSearchPredicate(search, dialect)
	order, known := otelSearchOrders[search.OrderBy]
	if !known {
		order = otelSearchOrders["start_time desc"]
	}
	order = strings.ReplaceAll(order, "%s", dialect.StartOrder)
	page = "SELECT " + OtelTopologyColumns + " FROM " + SpansTable + " WHERE " + predicate + " ORDER BY " + order + " LIMIT ? OFFSET ?" + dialect.ReadSetting
	pageArgs = append(append([]any{}, args...), search.PageSize, (search.Page-1)*search.PageSize)
	return page, pageArgs, "SELECT count(*) FROM " + SpansTable + " WHERE " + predicate + dialect.ReadSetting, args
}

// OtelServicesQuery lists the services that reported spans in the range, busiest first, for the explorer's filter.
func OtelServicesQuery(project uuid.UUID, from, to time.Time, dialect OtelSearchDialect) (string, []any) {
	return "SELECT service_name FROM " + SpansTable + " WHERE project_id = ? AND recorded_at >= ? AND recorded_at <= ? AND service_name != '' GROUP BY service_name ORDER BY count(*) DESC LIMIT 100" + dialect.ReadSetting,
		[]any{dialect.Project(project), dialect.Time(from), dialect.Time(to)}
}
