package shared

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"
)

const OtelTextWinnerOrder = "duration DESC, otlp DESC"

// A stored digest keeps ClickHouse topology reads independent of payload size.
const OtelClickHouseWinnerOrder = "duration DESC, span_version DESC"

// Apply the same winner before filtering or limiting every logical-span read.
// Otherwise topology, attributes and export can describe different retry versions.
func OtelWinningRows(columns, predicate, order string) string {
	return "(SELECT " + columns + " FROM (SELECT " + columns +
		", ROW_NUMBER() OVER (PARTITION BY project_id, trace_id, span_id ORDER BY " + order +
		") AS otel_version FROM " + SpansTable + " WHERE " + predicate + ") ranked WHERE otel_version = 1) winners"
}

func OtelTopologyQuery(lookups []SpanLookup, combinedKey bool, fromUnixNano uint64, timeValue func(time.Time) any) (string, []any) {
	predicate, args := OtelTracePredicate(lookups, combinedKey)
	predicate, args = OtelWindowPredicate(predicate, args, lookups, timeValue)
	startOrder := "start_time_unix_nano"
	if !combinedKey {
		startOrder = "CAST(start_time_unix_nano AS INTEGER)"
	}
	query := "SELECT " + OtelTopologyColumns + " FROM " + OtelWinningRows(OtelTopologyColumns, predicate, OtelTextWinnerOrder)
	if fromUnixNano > 0 {
		query += " WHERE " + startOrder + " >= ?"
		args = append(args, OtelNanosArgument(fromUnixNano))
	}
	return query + fmt.Sprintf(" ORDER BY %s, span_id LIMIT %d", startOrder, MaxOtelGraphRows+1), args
}

// OtelImportanceOrder decides which spans survive the row cap on a whole trace read: errors, then entry points, then
// the slowest. A parent usually outlasts its children, so ordering by duration also tends to keep the path above a span.
const OtelImportanceOrder = " ORDER BY (status_code = 2) DESC, (parent_span_id = '' OR span_kind IN (2, 5)) DESC, duration DESC, span_id"

const OtelStatementPreviewLength = 240

// OtelOutlineQuery reads one whole trace without its attributes, from every project named by the lookups: services of
// one trace often report to different projects. statement is the backend's expression for the preview
// of the database statement, the one attribute a row label needs.
func OtelOutlineQuery(lookups []SpanLookup, combinedKey bool, statement string, timeValue func(time.Time) any) (string, []any) {
	predicate, args := OtelTracePredicate(lookups, combinedKey)
	predicate, args = OtelWindowPredicate(predicate, args, lookups, timeValue)
	return "SELECT " + OtelTopologyColumns + ", " + statement + " FROM " + OtelWinningRows(OtelTopologyColumns+", span_attributes", predicate, OtelTextWinnerOrder) + OtelImportanceOrder + fmt.Sprintf(" LIMIT %d", MaxOtelGraphRows+1), args
}

// OtelWindowPredicate adds the lookup window to a predicate on the spans table.
func OtelWindowPredicate(predicate string, args []any, lookups []SpanLookup, timeValue func(time.Time) any) (string, []any) {
	from, to := OtelLookupWindow(lookups)
	return predicate + " AND recorded_at >= ? AND recorded_at <= ?", append(args, timeValue(from), timeValue(to))
}

func OtelPlaceholders(count int) string {
	return strings.TrimSuffix(strings.Repeat("?,", count), ",")
}

// OtelNanosArgument keeps a start bound inside what database/sql can bind.
func OtelNanosArgument(nanos uint64) int64 {
	return int64(min(nanos, uint64(math.MaxInt64)))
}

func ScanOtelAttributeRows(rows *sql.Rows) (map[string]OtelSpanAttributes, error) {
	result := make(map[string]OtelSpanAttributes)
	for rows.Next() {
		var id, encoded string
		var flag int
		if err := rows.Scan(&id, &encoded, &flag); err != nil {
			return nil, err
		}
		if _, seen := result[id]; seen {
			continue
		}
		attributes := OtelSpanAttributes{Omitted: OtelAttributeOmission(flag), Bytes: len(encoded)}
		if attributes.Omitted == "" && encoded != "" {
			if err := json.Unmarshal([]byte(encoded), &attributes.Attributes); err != nil {
				return nil, err
			}
		}
		result[id] = attributes
	}
	return result, rows.Err()
}
