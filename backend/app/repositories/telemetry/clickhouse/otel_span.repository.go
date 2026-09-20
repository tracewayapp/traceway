//go:build telemetry_ch

package clickhouse

import (
	"cmp"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	ch "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/chdb"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry/shared"
)

type otelSpanRepository struct{}

const otelInsertColumns = `INSERT INTO ` + shared.SpansTable + ` (` + shared.OtelScalarColumns + `, span_attributes, resource, scope, events, links, resource_pb, scope_pb, span_pb)`

var clickhouseOtelCodec = shared.OtelValueCodec{
	UUID:   func(id uuid.UUID) any { return id },
	Time:   func(t time.Time) any { return t.UTC() },
	Nanos:  func(n uint64) any { return n },
	Uint32: func(n uint32) any { return n },
}

// Order must match the columns otelInsertColumns adds after shared.OtelScalarColumns.
func otelNestedValues(row *shared.OtelSpanRow) ([]any, error) {
	values, err := row.NestedValues()
	if err != nil {
		return nil, err
	}
	return append(values, string(row.Group.ResourcePB), string(row.Group.ScopePB), string(row.SpanPB)), nil
}

// ClickHouse rejects one insert that touches more than max_partitions_per_insert_block daily partitions (100 by default).
const (
	clickhouseMaxPartitionsPerInsert = 100
	clickhouseMaxPartitionedInserts  = 8
)

type clickhouseOtelRow struct {
	owner  shared.SpanOwner
	values []any
	day    int64
}

func clickhouseOtelValues(span models.OtelSpan, groups shared.OtelSpanGroups) (clickhouseOtelRow, error) {
	row, err := shared.NewOtelSpanRow(span, groups)
	if err != nil {
		return clickhouseOtelRow{}, err
	}
	values := row.ScalarValues(clickhouseOtelCodec)
	nested, err := otelNestedValues(row)
	if err != nil {
		return clickhouseOtelRow{}, err
	}
	return clickhouseOtelRow{owner: shared.SpanOwner{ProjectId: span.ProjectId, TraceId: span.TraceId, SpanId: span.SpanId}, values: append(values, nested...), day: row.PartitionTime.Unix() / 86400}, nil
}

// partitionedInserts keeps each insert within the partition limit and bounds how many inserts one request may cause.
func partitionedInserts(rows []clickhouseOtelRow) (inserts [][]clickhouseOtelRow, overflow []clickhouseOtelRow) {
	days := make(map[int64]bool)
	for _, row := range rows {
		days[row.day] = true
	}
	if len(days) <= clickhouseMaxPartitionsPerInsert {
		return [][]clickhouseOtelRow{rows}, nil
	}
	slices.SortStableFunc(rows, func(a, b clickhouseOtelRow) int { return cmp.Compare(a.day, b.day) })
	start, distinct := 0, 0
	for i, row := range rows {
		if i > 0 && row.day == rows[i-1].day {
			continue
		}
		if distinct == clickhouseMaxPartitionsPerInsert {
			inserts = append(inserts, rows[start:i])
			start, distinct = i, 0
			if len(inserts) == clickhouseMaxPartitionedInserts {
				return inserts, rows[i:]
			}
		}
		distinct++
	}
	return append(inserts, rows[start:]), nil
}

func (r *otelSpanRepository) InsertAsync(ctx context.Context, spans []models.OtelSpan) (int, error) {
	rejected, err := r.InsertWithRejections(ctx, spans)
	return len(rejected), err
}

func (r *otelSpanRepository) InsertWithRejections(ctx context.Context, spans []models.OtelSpan) ([]shared.SpanOwner, error) {
	var rejected []shared.SpanOwner
	groups, rows := shared.OtelSpanGroups{}, make([]clickhouseOtelRow, 0, len(spans))
	for _, span := range spans {
		row, err := clickhouseOtelValues(span, groups)
		if err != nil {
			shared.RecordRejectedOtelSpan(err)
			rejected = append(rejected, shared.SpanOwner{ProjectId: span.ProjectId, TraceId: span.TraceId, SpanId: span.SpanId})
			continue
		}
		rows = append(rows, row)
	}
	if len(rows) == 0 {
		return rejected, nil
	}
	inserts, overflow := partitionedInserts(rows)
	for _, row := range overflow {
		rejected = append(rejected, row.owner)
		shared.RecordRejectedOtelSpan(fmt.Errorf("one request spans more than %d daily partitions", clickhouseMaxPartitionsPerInsert*clickhouseMaxPartitionedInserts))
	}
	for _, insert := range inserts {
		err := chdb.SendBatch(otelInsertColumns, func(batch driver.Batch) error {
			for _, row := range insert {
				if err := batch.Append(row.values...); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return rejected, nil
}

const otelReadSettings = ` SETTINGS max_execution_time=10, max_rows_to_read=1000000, max_bytes_to_read=268435456, max_memory_usage=268435456, read_overflow_mode='throw', timeout_overflow_mode='throw'`

// One lookup uses equality so index analysis keys on project_id and trace_id with binary search. Several lookups
// name both key sets for the same reason and keep the pair condition, which stops a project matching another's trace.
func clickhouseTracePredicate(lookups []shared.SpanLookup) (string, []any) {
	projects, traces := make([]uuid.UUID, 0, len(lookups)), make([]string, 0, len(lookups))
	pairs := make([]any, 0, len(lookups)*2)
	for _, lookup := range lookups {
		if !slices.Contains(projects, lookup.ProjectId) {
			projects = append(projects, lookup.ProjectId)
		}
		if !slices.Contains(traces, lookup.TraceId) {
			traces = append(traces, lookup.TraceId)
		}
		pairs = append(pairs, lookup.ProjectId, lookup.TraceId)
	}
	if len(lookups) == 1 {
		return "project_id = ? AND trace_id = ?", pairs
	}
	tuples := strings.TrimSuffix(strings.Repeat("(?, ?),", len(lookups)), ",")
	return "project_id IN (?) AND trace_id IN (?) AND (project_id, trace_id) IN (" + tuples + ")", append([]any{projects, traces}, pairs...)
}

func (r *otelSpanRepository) FindTraceTopology(ctx context.Context, lookups []shared.SpanLookup, fromUnixNano uint64) ([]models.OtelSpan, error) {
	if len(lookups) == 0 {
		return nil, nil
	}
	predicate, args := clickhouseTracePredicate(lookups)
	predicate, args = shared.OtelWindowPredicate(predicate, args, lookups, clickhouseOtelCodec.Time)
	query := "SELECT " + shared.OtelTopologyColumns + " FROM " + shared.OtelWinningRows(shared.OtelTopologyColumns, predicate, shared.OtelClickHouseWinnerOrder)
	if fromUnixNano > 0 {
		query += " WHERE start_time_unix_nano >= ?"
		args = append(args, fromUnixNano)
	}
	query += fmt.Sprintf(" ORDER BY start_time_unix_nano ASC, span_id ASC LIMIT %d", shared.MaxOtelGraphRows+1) + otelReadSettings
	rows, err := chdb.Conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanClickhouseTopology(rows, false)
}

const clickhouseStatementPreview = `substringUTF8(if(span_attributes['db.query.text'] != '', span_attributes['db.query.text'], span_attributes['db.statement']), 1, 240)`

func (r *otelSpanRepository) FindTraceOutline(ctx context.Context, lookups []shared.SpanLookup) ([]models.OtelSpan, error) {
	predicate, args := clickhouseTracePredicate(lookups)
	predicate, args = shared.OtelWindowPredicate(predicate, args, lookups, clickhouseOtelCodec.Time)
	query := "SELECT " + shared.OtelTopologyColumns + ", " + clickhouseStatementPreview + " FROM " + shared.OtelWinningRows(shared.OtelTopologyColumns+", span_attributes", predicate, shared.OtelClickHouseWinnerOrder) + shared.OtelImportanceOrder + fmt.Sprintf(" LIMIT %d", shared.MaxOtelGraphRows+1) + otelReadSettings
	rows, err := chdb.Conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanClickhouseTopology(rows, true)
}

func scanClickhouseTopology(rows driver.Rows, withStatement bool) ([]models.OtelSpan, error) {
	result := make([]models.OtelSpan, 0)
	for rows.Next() {
		var span models.OtelSpan
		var nanos uint64
		columns := []any{&span.ProjectId, &span.TraceId, &span.SpanId, &span.ParentSpanId,
			&span.Name, &span.Duration, &span.SpanKind, &span.StatusCode, &span.ServiceName, &span.ScopeName, &nanos}
		if withStatement {
			columns = append(columns, &span.DbStatement)
		}
		if err := rows.Scan(columns...); err != nil {
			return nil, err
		}
		span.StartTime = shared.OtelNanosToTime(nanos)
		span.RecordedAt = span.StartTime
		result = append(result, span)
	}
	return result, rows.Err()
}

// A search reads every span of the project in the range, so it is held by time and memory, not by a row count.
const otelSearchSettings = ` SETTINGS max_execution_time=15, timeout_overflow_mode='throw', max_memory_usage=1073741824`

var clickhouseSearchDialect = shared.OtelSearchDialect{
	WinnerOrder: shared.OtelClickHouseWinnerOrder,
	Project:     func(id uuid.UUID) any { return id },
	Time:        clickhouseOtelCodec.Time,
	Trace:       func(_ uuid.UUID, traceId string) (string, any) { return "trace_id = ?", traceId },
	NameLike:    "positionCaseInsensitive(name, ?) > 0",
	Attribute:   "span_attributes[?] = ?",
	StartOrder:  "start_time_unix_nano",
	ReadSetting: otelSearchSettings,
}

func (r *otelSpanRepository) Search(ctx context.Context, search shared.OtelSpanSearch) ([]models.OtelSpan, uint64, error) {
	page, pageArgs, count, countArgs := shared.OtelSearchQueries(search, clickhouseSearchDialect)
	var total uint64
	if err := chdb.Conn.QueryRow(ctx, count, countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := chdb.Conn.Query(ctx, page, pageArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	spans, err := scanClickhouseTopology(rows, false)
	return spans, total, err
}

func (r *otelSpanRepository) Services(ctx context.Context, project uuid.UUID, from, to time.Time) ([]string, error) {
	query, args := shared.OtelServicesQuery(project, from, to, clickhouseSearchDialect)
	rows, err := chdb.Conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	services := make([]string, 0)
	for rows.Next() {
		var service string
		if err := rows.Scan(&service); err != nil {
			return nil, err
		}
		services = append(services, service)
	}
	return services, rows.Err()
}

func (r *otelSpanRepository) FindSpanAttributes(ctx context.Context, lookup shared.SpanLookup, spanIds []string, limits shared.OtelAttributeLimits) (map[string]shared.OtelSpanAttributes, error) {
	predicate, args := shared.OtelWindowPredicate("project_id = ? AND trace_id = ? AND span_id IN (?)",
		[]any{lookup.ProjectId, lookup.TraceId, spanIds}, []shared.SpanLookup{lookup}, clickhouseOtelCodec.Time)
	query := shared.OtelAttributeQuery("toJSONString(span_attributes)", "length(toJSONString(span_attributes))", "start_time_unix_nano", predicate, shared.OtelClickHouseWinnerOrder, limits) + otelReadSettings
	rows, err := chdb.Conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make(map[string]shared.OtelSpanAttributes)
	for rows.Next() {
		var id, encoded string
		var flag uint8
		if err := rows.Scan(&id, &encoded, &flag); err != nil {
			return nil, err
		}
		if _, seen := result[id]; seen {
			continue
		}
		attributes := shared.OtelSpanAttributes{Omitted: shared.OtelAttributeOmission(int(flag)), Bytes: len(encoded)}
		if attributes.Omitted == "" && encoded != "" {
			if err := json.Unmarshal([]byte(encoded), &attributes.Attributes); err != nil {
				return nil, err
			}
		}
		result[id] = attributes
	}
	return result, rows.Err()
}

var clickhouseReadLimitCodes = map[int32]bool{
	158: true, // TOO_MANY_ROWS
	159: true, // TIMEOUT_EXCEEDED
	160: true, // TOO_SLOW
	241: true, // MEMORY_LIMIT_EXCEEDED
	307: true, // TOO_MANY_BYTES
	396: true, // TOO_MANY_ROWS_OR_BYTES
}

func (r *otelSpanRepository) IsReadLimitError(err error) bool {
	var exception *ch.Exception
	return errors.As(err, &exception) && clickhouseReadLimitCodes[exception.Code]
}

var OtelSpanRepository = &otelSpanRepository{}

func (r *otelSpanRepository) FindOTLP(ctx context.Context, project uuid.UUID, traceID, spanID string, at time.Time) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	from, to := shared.TraceWindowBounds(at.UTC())
	var resourcePB, scopePB, spanPB string
	err := chdb.Conn.QueryRow(ctx, "SELECT resource_pb, scope_pb, span_pb FROM "+shared.SpansTable+" WHERE project_id = ? AND trace_id = ? AND span_id = ? AND recorded_at >= ? AND recorded_at <= ? ORDER BY "+shared.OtelClickHouseWinnerOrder+" LIMIT 1"+otelReadSettings, project, traceID, spanID, from, to).Scan(&resourcePB, &scopePB, &spanPB)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return shared.AssembleOtelPayload([]byte(resourcePB), []byte(scopePB), []byte(spanPB)), nil
}
