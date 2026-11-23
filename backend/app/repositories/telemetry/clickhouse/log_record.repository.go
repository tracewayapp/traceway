//go:build telemetry_ch

package clickhouse

import (
	"context"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/tracewayapp/traceway/backend/v2/app/repositories/telemetry/shared"
	"github.com/tracewayapp/traceway/backend/v2/app/traceway"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/tracewayapp/traceway/backend/v2/app/chdb"
	"github.com/tracewayapp/traceway/backend/v2/app/models"
)

type logRecordRepository struct{}

func (r *logRecordRepository) InsertAsync(ctx context.Context, records []models.LogRecord) error {
	if len(records) == 0 {
		return nil
	}

	// SendBatch retries a dead pooled connection; a bare PrepareBatch turned
	// each one into a 503 for the exporter and an issue in self-monitoring.
	return chdb.SendBatch(ctx, "INSERT INTO log_records (id, project_id, timestamp, trace_id, span_id, trace_flags, severity_text, severity_number, service_name, body, resource_schema_url, resource_attributes, scope_schema_url, scope_name, scope_version, scope_attributes, log_attributes)", func(batch driver.Batch) error {
		for _, lr := range records {
			resAttrs := lr.ResourceAttributes
			if resAttrs == nil {
				resAttrs = map[string]string{}
			}
			scopeAttrs := lr.ScopeAttributes
			if scopeAttrs == nil {
				scopeAttrs = map[string]string{}
			}
			logAttrs := lr.LogAttributes
			if logAttrs == nil {
				logAttrs = map[string]string{}
			}

			if err := batch.Append(
				lr.Id,
				lr.ProjectId,
				lr.Timestamp,
				lr.TraceId,
				lr.SpanId,
				lr.TraceFlags,
				lr.SeverityText,
				lr.SeverityNumber,
				lr.ServiceName,
				lr.Body,
				lr.ResourceSchemaUrl,
				resAttrs,
				lr.ScopeSchemaUrl,
				lr.ScopeName,
				lr.ScopeVersion,
				scopeAttrs,
				logAttrs,
			); err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *logRecordRepository) Search(ctx context.Context, params shared.LogSearchParams) ([]models.LogRecord, int64, error) {
	where, args := r.buildWhere(params)

	countQuery := "SELECT count() FROM log_records WHERE " + where
	var count uint64
	countCtx, countSpan := traceway.StartSpan(ctx, "logs.count")
	err := chdb.Conn.QueryRow(countCtx, countQuery, args...).Scan(&count)
	countSpan.End()
	if err != nil {
		return nil, 0, err
	}
	if count == 0 {
		return nil, 0, nil
	}

	orderBy := r.resolveOrderBy(params.OrderBy)
	direction := "DESC"
	if strings.EqualFold(params.SortDirection, "asc") {
		direction = "ASC"
	}

	if params.PageSize <= 0 {
		params.PageSize = 50
	}
	if params.Page <= 0 {
		params.Page = 1
	}
	offset := (params.Page - 1) * params.PageSize

	query := `SELECT id, project_id, timestamp, trace_id, span_id, trace_flags,
		severity_text, severity_number, service_name, body,
		resource_schema_url, resource_attributes,
		scope_schema_url, scope_name, scope_version, scope_attributes,
		log_attributes
	FROM log_records
	WHERE ` + where + ` ORDER BY ` + orderBy + ` ` + direction + `, id LIMIT ? OFFSET ?`

	args = append(args, params.PageSize, offset)

	fetchCtx, fetchSpan := traceway.StartSpan(ctx, "logs.fetch")
	defer fetchSpan.End()
	rows, err := chdb.Conn.Query(fetchCtx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var records []models.LogRecord
	for rows.Next() {
		var lr models.LogRecord
		if err := rows.Scan(
			&lr.Id,
			&lr.ProjectId,
			&lr.Timestamp,
			&lr.TraceId,
			&lr.SpanId,
			&lr.TraceFlags,
			&lr.SeverityText,
			&lr.SeverityNumber,
			&lr.ServiceName,
			&lr.Body,
			&lr.ResourceSchemaUrl,
			&lr.ResourceAttributes,
			&lr.ScopeSchemaUrl,
			&lr.ScopeName,
			&lr.ScopeVersion,
			&lr.ScopeAttributes,
			&lr.LogAttributes,
		); err != nil {
			return nil, 0, err
		}
		records = append(records, lr)
	}

	return records, int64(count), nil
}

// Histogram counts the logs Search would match per step-second bucket and level.
func (r *logRecordRepository) Histogram(ctx context.Context, params shared.LogSearchParams, step int64) ([]shared.LogHistogramRow, error) {
	where, args := r.buildWhere(params)
	query := `SELECT ` + epochBucket("timestamp", int(step)) + ` AS bucket,
		toUInt8(` + shared.LogLevelSQL + `) AS level, count() AS count
	FROM log_records WHERE ` + where + `
	GROUP BY bucket, level`
	rows, err := chdb.Conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []shared.LogHistogramRow
	for rows.Next() {
		var bucket int64
		var level uint8
		var count uint64
		if err := rows.Scan(&bucket, &level, &count); err != nil {
			return nil, err
		}
		out = append(out, shared.LogHistogramRow{Bucket: bucket, Level: int(level), Count: int64(count)})
	}
	return out, rows.Err()
}

// attributeValues is the subquery every attribute chart reads: one row per
// matching log that carries the attribute, with its bucket, whether its value
// is a number, and the number. step 0 puts every row in bucket 0.
func (r *logRecordRepository) attributeValues(params shared.LogSearchParams, target shared.LogAttributeTarget, step int64) (string, []interface{}, bool) {
	col := attrColumn(target.Scope)
	if col == "" {
		return "", nil, false
	}
	where, whereArgs := r.buildWhere(params)
	bucket := "toInt64(0)"
	if step > 0 {
		bucket = epochBucket("timestamp", int(step))
	}
	query := `SELECT bucket, match(raw, '` + shared.LogAttributeNumberPattern + `') AS num, if(num, toFloat64OrZero(raw), 0) AS v
		FROM (SELECT ` + bucket + ` AS bucket, ` + col + `[?] AS raw
			FROM log_records WHERE ` + where + ` AND mapContains(` + col + `, ?))`
	args := append([]interface{}{target.Key}, whereArgs...)
	args = append(args, target.Key)
	return query, args, true
}

// AttributeStats summarises the numeric values of one attribute per step-second
// bucket of the logs Search would match (step 0: one row for the whole range).
func (r *logRecordRepository) AttributeStats(ctx context.Context, params shared.LogSearchParams, target shared.LogAttributeTarget, step int64) ([]shared.LogAttributeStatsRow, error) {
	values, args, ok := r.attributeValues(params, target, step)
	if !ok {
		return nil, nil
	}
	query := `SELECT bucket, toInt64(count()) AS present, toInt64(countIf(num)) AS numeric,
		toInt64(countIf(num AND v = floor(v))) AS integral,
		sumIf(v, num) AS sum, minIf(v, num) AS min, maxIf(v, num) AS max,
		quantilesIf(0.5, 0.95, 0.99)(v, num) AS q
	FROM (` + values + `)
	GROUP BY bucket`
	rows, err := chdb.Conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []shared.LogAttributeStatsRow
	for rows.Next() {
		var row shared.LogAttributeStatsRow
		var q []float64
		if err := rows.Scan(&row.Bucket, &row.Present, &row.Numeric, &row.Integral, &row.Sum, &row.Min, &row.Max, &q); err != nil {
			return nil, err
		}
		if row.Numeric > 0 && len(q) == len(shared.LogAttributeQuantiles) {
			row.P50, row.P95, row.P99 = q[0], q[1], q[2]
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// AttributeBins counts the numeric values of one attribute per bin.
func (r *logRecordRepository) AttributeBins(ctx context.Context, params shared.LogSearchParams, target shared.LogAttributeTarget, binning shared.LogAttributeBinning) ([]shared.LogAttributeBinRow, error) {
	values, valueArgs, ok := r.attributeValues(params, target, 0)
	if !ok {
		return nil, nil
	}
	query := `SELECT toInt64(greatest(0, least(toInt64(floor((v - ?) / ?)), ?))) AS bin, toInt64(count()) AS count
	FROM (` + values + `) WHERE num
	GROUP BY bin`
	args := append([]interface{}{binning.Start, binning.Width, int64(binning.Count - 1)}, valueArgs...)
	rows, err := chdb.Conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []shared.LogAttributeBinRow
	for rows.Next() {
		var index, count int64
		if err := rows.Scan(&index, &count); err != nil {
			return nil, err
		}
		out = append(out, shared.LogAttributeBinRow{Index: int(index), Count: count})
	}
	return out, rows.Err()
}

func (r *logRecordRepository) FindByTraceId(ctx context.Context, projectId uuid.UUID, traceId string) ([]models.LogRecord, error) {
	query := `SELECT ` + chLogRecordColumns + `
	FROM log_records
	WHERE project_id = ? AND trace_id = ?
	ORDER BY timestamp ASC`

	rows, err := chdb.Conn.Query(ctx, query, projectId, traceId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanChLogRecords(rows)
}

// FindBySessionId returns a browser session's log records in a time window, oldest first; it reads through the
// materialized session_id column and its bloom filter.
func (r *logRecordRepository) FindBySessionId(ctx context.Context, q shared.SessionBreadcrumbQuery) ([]models.LogRecord, error) {
	// timestamp_date prunes partitions, timestamp alone cannot; the day of slack is the same as in buildWhere.
	query := `SELECT ` + chLogRecordColumns + `
	FROM log_records
	WHERE project_id = ? AND session_id = ? AND timestamp_date >= ? AND timestamp_date <= ?
		AND timestamp >= ? AND timestamp <= ?
		AND log_attributes['event.name'] NOT IN (` + shared.SessionTimelineSkippedEventsSQL + `)
	ORDER BY timestamp ` + q.Order() + ` LIMIT ?`

	rows, err := chdb.Conn.Query(ctx, query, q.ProjectId, q.SessionId,
		q.From.UTC().AddDate(0, 0, -1).Format(time.DateOnly), q.To.UTC().AddDate(0, 0, 1).Format(time.DateOnly),
		q.From.UTC(), q.To.UTC(), q.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	records, err := scanChLogRecords(rows)
	if err != nil {
		return nil, err
	}
	return shared.SessionBreadcrumbsChronological(q, records), nil
}

const chLogRecordColumns = `id, project_id, timestamp, trace_id, span_id, trace_flags,
		severity_text, severity_number, service_name, body,
		resource_schema_url, resource_attributes,
		scope_schema_url, scope_name, scope_version, scope_attributes,
		log_attributes`

func scanChLogRecords(rows driver.Rows) ([]models.LogRecord, error) {
	var records []models.LogRecord
	for rows.Next() {
		var lr models.LogRecord
		if err := rows.Scan(
			&lr.Id,
			&lr.ProjectId,
			&lr.Timestamp,
			&lr.TraceId,
			&lr.SpanId,
			&lr.TraceFlags,
			&lr.SeverityText,
			&lr.SeverityNumber,
			&lr.ServiceName,
			&lr.Body,
			&lr.ResourceSchemaUrl,
			&lr.ResourceAttributes,
			&lr.ScopeSchemaUrl,
			&lr.ScopeName,
			&lr.ScopeVersion,
			&lr.ScopeAttributes,
			&lr.LogAttributes,
		); err != nil {
			return nil, err
		}
		records = append(records, lr)
	}
	return records, nil
}

func (r *logRecordRepository) buildWhere(params shared.LogSearchParams) (string, []interface{}) {
	clauses := []string{"project_id = ?", "timestamp >= ?", "timestamp <= ?"}
	args := []interface{}{params.ProjectId, params.FromDate, params.ToDate}
	if len(params.ProjectIds) > 0 {
		placeholders := make([]string, len(params.ProjectIds))
		args = make([]interface{}, 0, len(params.ProjectIds)+2)
		for i, id := range params.ProjectIds {
			placeholders[i] = "?"
			args = append(args, id)
		}
		clauses[0] = "project_id IN (" + strings.Join(placeholders, ", ") + ")"
		args = append(args, params.FromDate, params.ToDate)
	}

	// log_records is partitioned by the stored timestamp_date column, which a
	// filter on timestamp alone cannot prune. The day of slack on each side
	// absorbs any timezone difference between this and the insert-time toDate.
	clauses = append(clauses, "timestamp_date >= ?", "timestamp_date <= ?")
	args = append(args,
		params.FromDate.UTC().AddDate(0, 0, -1).Format(time.DateOnly),
		params.ToDate.UTC().AddDate(0, 0, 1).Format(time.DateOnly))

	if params.MinSeverity > 0 {
		clauses = append(clauses, "severity_number >= ?")
		args = append(args, params.MinSeverity)
	}
	if params.ServiceName != "" {
		clauses = append(clauses, "service_name = ?")
		args = append(args, params.ServiceName)
	}
	if len(params.TraceIds) > 0 {
		placeholders := make([]string, len(params.TraceIds))
		for i, tid := range params.TraceIds {
			placeholders[i] = "?"
			args = append(args, tid)
		}
		clauses = append(clauses, "trace_id IN ("+strings.Join(placeholders, ", ")+")")
	} else if params.TraceId != "" {
		clauses = append(clauses, "trace_id = ?")
		args = append(args, params.TraceId)
	}
	if params.SpanId != "" {
		clauses = append(clauses, "span_id = ?")
		args = append(args, params.SpanId)
	}
	if params.ScopeName != "" {
		clauses = append(clauses, "scope_name = ?")
		args = append(args, params.ScopeName)
	}
	if params.Body != "" {
		clauses = append(clauses, "body = ?")
		args = append(args, params.Body)
	}

	for _, f := range params.AttributeFilters {
		col := attrColumn(f.Scope)
		if col == "" {
			continue
		}
		if f.Exists {
			if f.Exclude {
				clauses = append(clauses, "NOT mapContains("+col+", ?)")
			} else {
				clauses = append(clauses, "mapContains("+col+", ?)")
			}
			args = append(args, f.Key)
			continue
		}
		if f.Contains {
			// Case-insensitive substring match. Map columns default missing
			// keys to '', which contains nothing, so the negated form keeps
			// rows that don't carry the attribute at all.
			op := "> 0"
			if f.Exclude {
				op = "= 0"
			}
			clauses = append(clauses, "positionCaseInsensitive("+col+"[?], ?) "+op)
		} else {
			op := "="
			if f.Exclude {
				// Map columns default missing keys to '', so != also keeps rows
				// that don't carry the attribute at all.
				op = "!="
			}
			clauses = append(clauses, col+"[?] "+op+" ?")
		}
		args = append(args, f.Key, f.Value)
	}

	if params.Search != "" {
		switch params.SearchType {
		case "service":
			clauses = append(clauses, "service_name ILIKE ?")
			args = append(args, "%"+params.Search+"%")
		case "trace":
			clauses = append(clauses, "trace_id = ?")
			args = append(args, params.Search)
		default:
			// Prefer hasToken() for single-word searches — it actually uses the
			// tokenbf_v1 index on body. Fall back to ILIKE when the term is not
			// a single alphanumeric token (multi-word, punctuation, underscores, etc.).
			if isSingleToken(params.Search) {
				clauses = append(clauses, "hasToken(body, ?)")
				args = append(args, params.Search)
			} else {
				clauses = append(clauses, "body ILIKE ?")
				args = append(args, "%"+params.Search+"%")
			}
		}
	}

	return strings.Join(clauses, " AND "), args
}

func attrColumn(scope string) string {
	switch scope {
	case "resource":
		return "resource_attributes"
	case "scope":
		return "scope_attributes"
	case "log":
		return "log_attributes"
	default:
		return ""
	}
}

// isSingleToken reports whether s is a single ASCII alphanumeric token, the
// only shape hasToken() accepts: it treats every other ASCII character,
// underscore included, as a separator and fails the query (code 36) on a
// needle that contains one.
func isSingleToken(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')) {
			return false
		}
	}
	return true
}

func (r *logRecordRepository) resolveOrderBy(orderBy string) string {
	allowed := map[string]string{
		"timestamp":       "timestamp",
		"severity_number": "severity_number",
		"service_name":    "service_name",
	}
	if col, ok := allowed[orderBy]; ok {
		return col
	}
	return "timestamp"
}

var LogRecordRepository = &logRecordRepository{}
