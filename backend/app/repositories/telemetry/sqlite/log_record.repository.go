//go:build !telemetry_ch && !telemetry_duckdb

package sqlite

import (
	"context"
	"fmt"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry/shared"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry/sqlitetypes"
	"strings"

	"github.com/google/uuid"
	"github.com/tracewayapp/lit/v2"

	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
)

type logRecord struct {
	Id                 uuid.UUID                 `lit:"id"`
	ProjectId          uuid.UUID                 `lit:"project_id"`
	Timestamp          sqlitetypes.SQLiteTime    `lit:"timestamp"`
	TraceId            string                    `lit:"trace_id"`
	SpanId             string                    `lit:"span_id"`
	TraceFlags         uint8                     `lit:"trace_flags"`
	SeverityText       string                    `lit:"severity_text"`
	SeverityNumber     uint8                     `lit:"severity_number"`
	ServiceName        string                    `lit:"service_name"`
	Body               string                    `lit:"body"`
	ResourceSchemaUrl  string                    `lit:"resource_schema_url"`
	ResourceAttributes sqlitetypes.SQLiteJSONMap `lit:"resource_attributes"`
	ScopeSchemaUrl     string                    `lit:"scope_schema_url"`
	ScopeName          string                    `lit:"scope_name"`
	ScopeVersion       string                    `lit:"scope_version"`
	ScopeAttributes    sqlitetypes.SQLiteJSONMap `lit:"scope_attributes"`
	LogAttributes      sqlitetypes.SQLiteJSONMap `lit:"log_attributes"`
}

func init() {
	models.ExtensionModelRegistrations = append(models.ExtensionModelRegistrations, func(driver lit.Driver) {
		lit.RegisterModel[logRecord](driver)
	})
}

func logRecordToRow(lr models.LogRecord) logRecord {
	return logRecord{
		Id:                 lr.Id,
		ProjectId:          lr.ProjectId,
		Timestamp:          sqlitetypes.NewSQLiteTime(lr.Timestamp),
		TraceId:            lr.TraceId,
		SpanId:             lr.SpanId,
		TraceFlags:         lr.TraceFlags,
		SeverityText:       lr.SeverityText,
		SeverityNumber:     lr.SeverityNumber,
		ServiceName:        lr.ServiceName,
		Body:               lr.Body,
		ResourceSchemaUrl:  lr.ResourceSchemaUrl,
		ResourceAttributes: sqlitetypes.NewSQLiteJSONMap(lr.ResourceAttributes),
		ScopeSchemaUrl:     lr.ScopeSchemaUrl,
		ScopeName:          lr.ScopeName,
		ScopeVersion:       lr.ScopeVersion,
		ScopeAttributes:    sqlitetypes.NewSQLiteJSONMap(lr.ScopeAttributes),
		LogAttributes:      sqlitetypes.NewSQLiteJSONMap(lr.LogAttributes),
	}
}

func (r *logRecord) toModel() models.LogRecord {
	lr := models.LogRecord{
		Id:                r.Id,
		ProjectId:         r.ProjectId,
		Timestamp:         r.Timestamp.Time,
		TraceId:           r.TraceId,
		SpanId:            r.SpanId,
		TraceFlags:        r.TraceFlags,
		SeverityText:      r.SeverityText,
		SeverityNumber:    r.SeverityNumber,
		ServiceName:       r.ServiceName,
		Body:              r.Body,
		ResourceSchemaUrl: r.ResourceSchemaUrl,
		ScopeSchemaUrl:    r.ScopeSchemaUrl,
		ScopeName:         r.ScopeName,
		ScopeVersion:      r.ScopeVersion,
	}
	if r.ResourceAttributes != nil {
		lr.ResourceAttributes = map[string]string(r.ResourceAttributes)
	}
	if r.ScopeAttributes != nil {
		lr.ScopeAttributes = map[string]string(r.ScopeAttributes)
	}
	if r.LogAttributes != nil {
		lr.LogAttributes = map[string]string(r.LogAttributes)
	}
	return lr
}

type logRecordRepository struct{}

const logRecordColumns = `id, project_id, timestamp, trace_id, span_id, trace_flags,
		severity_text, severity_number, service_name, body,
		resource_schema_url, resource_attributes,
		scope_schema_url, scope_name, scope_version, scope_attributes,
		log_attributes`

// SQLite caps a compound SELECT at 500 terms; past this many trace ids the
// page query falls back to the IN form.
const maxTraceIdBranches = 100

func (r *logRecordRepository) InsertAsync(ctx context.Context, records []models.LogRecord) error {
	if len(records) == 0 {
		return nil
	}

	tx, err := db.TelemetryDB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, lr := range records {
		row := logRecordToRow(lr)
		if err := lit.InsertExistingUuid(tx, &row); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *logRecordRepository) Search(ctx context.Context, params shared.LogSearchParams) ([]models.LogRecord, int64, error) {
	where, args := r.buildWhere(params)

	traceIdIn := ""
	if len(params.TraceIds) > 0 {
		placeholders := make([]string, len(params.TraceIds))
		for i, tid := range params.TraceIds {
			placeholders[i] = fmt.Sprintf(":tid%d", i)
			args[fmt.Sprintf("tid%d", i)] = tid
		}
		traceIdIn = " AND trace_id IN (" + strings.Join(placeholders, ", ") + ")"
	}

	countResult, err := lit.SelectSingleNamed[models.CountResult](db.TelemetryDB,
		"SELECT COUNT(*) AS count FROM log_records WHERE "+where+traceIdIn, args)
	if err != nil {
		return nil, 0, err
	}
	count := int64(0)
	if countResult != nil {
		count = int64(countResult.Count)
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

	args["limit"] = params.PageSize
	args["offset"] = offset

	// An IN list over trace ids cannot serve ORDER BY from the trace index, so without sqlite_stat1 the
	// planner walks the whole time window (#353): seek each id, keep enough rows to reach the offset,
	// merge. Past the branch cap the + on the sort column forces the same index plus an in-memory sort.
	var query string
	switch {
	case len(params.TraceIds) == 0:
		query = fmt.Sprintf(`SELECT %s FROM log_records WHERE %s ORDER BY %s %s, id LIMIT :limit OFFSET :offset`,
			logRecordColumns, where, orderBy, direction)
	case len(params.TraceIds) <= maxTraceIdBranches:
		args["branch_limit"] = offset + params.PageSize
		branches := make([]string, len(params.TraceIds))
		for i := range branches {
			branches[i] = fmt.Sprintf(`SELECT * FROM (SELECT %s FROM log_records WHERE %s AND trace_id = :tid%d ORDER BY %s %s, id LIMIT :branch_limit)`,
				logRecordColumns, where, i, orderBy, direction)
		}
		query = strings.Join(branches, " UNION ALL ") + fmt.Sprintf(" ORDER BY %s %s, id LIMIT :limit OFFSET :offset", orderBy, direction)
	default:
		query = fmt.Sprintf(`SELECT %s FROM log_records WHERE %s ORDER BY +%s %s, id LIMIT :limit OFFSET :offset`,
			logRecordColumns, where+traceIdIn, orderBy, direction)
	}

	rows, err := lit.SelectNamed[logRecord](db.TelemetryDB, query, args)
	if err != nil {
		return nil, 0, err
	}

	records := make([]models.LogRecord, 0, len(rows))
	for _, row := range rows {
		records = append(records, row.toModel())
	}
	return records, count, nil
}

func (r *logRecordRepository) FindByTraceId(ctx context.Context, projectId uuid.UUID, traceId string) ([]models.LogRecord, error) {
	rows, err := lit.SelectNamed[logRecord](db.TelemetryDB,
		`SELECT `+logRecordColumns+` FROM log_records
		WHERE project_id = :project_id AND trace_id = :trace_id
		ORDER BY timestamp ASC`,
		lit.P{"project_id": projectId, "trace_id": traceId})
	if err != nil {
		return nil, err
	}

	records := make([]models.LogRecord, 0, len(rows))
	for _, row := range rows {
		records = append(records, row.toModel())
	}
	return records, nil
}

func (r *logRecordRepository) buildWhere(params shared.LogSearchParams) (string, lit.P) {
	clauses := []string{"project_id = :project_id", "timestamp >= :from", "timestamp <= :to"}
	args := lit.P{
		"project_id": params.ProjectId,
		"from":       sqlitetypes.NewSQLiteTime(params.FromDate),
		"to":         sqlitetypes.NewSQLiteTime(params.ToDate),
	}
	if len(params.ProjectIds) > 0 {
		placeholders := make([]string, len(params.ProjectIds))
		for i, id := range params.ProjectIds {
			key := fmt.Sprintf("project%d", i)
			placeholders[i] = ":" + key
			args[key] = id
		}
		clauses[0] = "project_id IN (" + strings.Join(placeholders, ", ") + ")"
	}

	if params.MinSeverity > 0 {
		clauses = append(clauses, "severity_number >= :min_severity")
		args["min_severity"] = params.MinSeverity
	}
	if params.ServiceName != "" {
		clauses = append(clauses, "service_name = :service_name")
		args["service_name"] = params.ServiceName
	}
	if params.TraceId != "" {
		clauses = append(clauses, "trace_id = :trace_id")
		args["trace_id"] = params.TraceId
	}
	if params.SpanId != "" {
		clauses = append(clauses, "span_id = :span_id")
		args["span_id"] = params.SpanId
	}
	if params.ScopeName != "" {
		clauses = append(clauses, "scope_name = :scope_name")
		args["scope_name"] = params.ScopeName
	}
	if params.Body != "" {
		clauses = append(clauses, "body = :body")
		args["body"] = params.Body
	}

	for i, f := range params.AttributeFilters {
		col := attrColumn(f.Scope)
		if col == "" {
			continue
		}
		keyPH := fmt.Sprintf("attr_k%d", i)
		valPH := fmt.Sprintf("attr_v%d", i)
		if f.Contains {
			// Case-insensitive substring match. COALESCE keeps the negated
			// form true for rows that don't carry the attribute at all
			// (json_extract yields NULL there, and INSTR on NULL is NULL).
			expr := fmt.Sprintf("INSTR(LOWER(COALESCE(json_extract(%s, '$.\"' || :%s || '\"'), '')), LOWER(:%s))", col, keyPH, valPH)
			if f.Exclude {
				clauses = append(clauses, expr+" = 0")
			} else {
				clauses = append(clauses, expr+" > 0")
			}
		} else if f.Exclude {
			// COALESCE keeps rows that don't carry the attribute at all
			// (json_extract yields NULL there, and NULL != value is NULL).
			clauses = append(clauses,
				fmt.Sprintf("COALESCE(json_extract(%s, '$.\"' || :%s || '\"'), '') != :%s", col, keyPH, valPH))
		} else {
			clauses = append(clauses,
				fmt.Sprintf("json_extract(%s, '$.\"' || :%s || '\"') = :%s", col, keyPH, valPH))
		}
		args[keyPH] = f.Key
		args[valPH] = f.Value
	}

	if params.Search != "" {
		switch params.SearchType {
		case "service":
			clauses = append(clauses, "INSTR(LOWER(service_name), LOWER(:search)) > 0")
			args["search"] = params.Search
		case "trace":
			if _, exists := args["trace_id"]; !exists {
				clauses = append(clauses, "trace_id = :search")
				args["search"] = params.Search
			}
		default:
			clauses = append(clauses, "INSTR(LOWER(body), LOWER(:search)) > 0")
			args["search"] = params.Search
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
