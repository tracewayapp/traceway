//go:build telemetry_ch

package clickhouse

import (
	"context"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry/shared"
	"strings"

	"github.com/google/uuid"

	"github.com/tracewayapp/traceway/backend/app/chdb"
	"github.com/tracewayapp/traceway/backend/app/models"
)

type logRecordRepository struct{}

func (r *logRecordRepository) InsertAsync(ctx context.Context, records []models.LogRecord) error {
	if len(records) == 0 {
		return nil
	}

	batch, err := chdb.Conn.PrepareBatch(
		chdb.BatchCtx(),
		"INSERT INTO log_records (id, project_id, timestamp, trace_id, span_id, trace_flags, severity_text, severity_number, service_name, body, resource_schema_url, resource_attributes, scope_schema_url, scope_name, scope_version, scope_attributes, log_attributes)",
	)
	if err != nil {
		return err
	}

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

	return batch.Send()
}

func (r *logRecordRepository) Search(ctx context.Context, params shared.LogSearchParams) ([]models.LogRecord, int64, error) {
	where, args := r.buildWhere(params)

	countQuery := "SELECT count() FROM log_records WHERE " + where
	var count uint64
	if err := chdb.Conn.QueryRow(ctx, countQuery, args...).Scan(&count); err != nil {
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
	WHERE ` + where + ` ORDER BY ` + orderBy + ` ` + direction + ` LIMIT ? OFFSET ?`

	args = append(args, params.PageSize, offset)

	rows, err := chdb.Conn.Query(ctx, query, args...)
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

func (r *logRecordRepository) FindByTraceId(ctx context.Context, projectId uuid.UUID, traceId string) ([]models.LogRecord, error) {
	query := `SELECT id, project_id, timestamp, trace_id, span_id, trace_flags,
		severity_text, severity_number, service_name, body,
		resource_schema_url, resource_attributes,
		scope_schema_url, scope_name, scope_version, scope_attributes,
		log_attributes
	FROM log_records
	WHERE project_id = ? AND trace_id = ?
	ORDER BY timestamp ASC`

	rows, err := chdb.Conn.Query(ctx, query, projectId, traceId)
	if err != nil {
		return nil, err
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
			return nil, err
		}
		records = append(records, lr)
	}
	return records, nil
}

func (r *logRecordRepository) buildWhere(params shared.LogSearchParams) (string, []interface{}) {
	clauses := []string{"project_id = ?", "timestamp >= ?", "timestamp <= ?"}
	args := []interface{}{params.ProjectId, params.FromDate, params.ToDate}

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
		op := "="
		if f.Exclude {
			// Map columns default missing keys to '', so != also keeps rows
			// that don't carry the attribute at all.
			op = "!="
		}
		clauses = append(clauses, col+"[?] "+op+" ?")
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
			// a single alphanumeric token (multi-word, punctuation, etc.).
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

// isSingleToken reports whether s is a single alphanumeric/underscore token —
// the only shape that tokenbf_v1 on body can accelerate via hasToken().
func isSingleToken(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !(r == '_' || (r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')) {
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
