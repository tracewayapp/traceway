//go:build telemetry_firebolt

package firebolt

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/tracewayapp/lit/v2"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry/shared"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry/sqlitetypes"
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
	Total              int64                     `lit:"total"`
}

func init() {
	registerModels(func(driver lit.Driver) {
		lit.RegisterModel[logRecord](driver)
	})
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

var logRecordColumns = []string{"id", "project_id", "timestamp", "trace_id", "span_id", "trace_flags",
	"severity_text", "severity_number", "service_name", "body",
	"resource_schema_url", "resource_attributes",
	"scope_schema_url", "scope_name", "scope_version", "scope_attributes",
	"log_attributes"}

func (r *logRecordRepository) InsertAsync(ctx context.Context, records []models.LogRecord) error {
	if len(records) == 0 {
		return nil
	}

	rows := make([][]any, 0, len(records))
	for _, lr := range records {
		resourceJSON, err := attrJSON(lr.ResourceAttributes)
		if err != nil {
			return err
		}
		scopeJSON, err := attrJSON(lr.ScopeAttributes)
		if err != nil {
			return err
		}
		logJSON, err := attrJSON(lr.LogAttributes)
		if err != nil {
			return err
		}

		rows = append(rows, []any{
			lr.Id.String(),
			lr.ProjectId.String(),
			lr.Timestamp.UTC(),
			lr.TraceId,
			lr.SpanId,
			int64(lr.TraceFlags),
			lr.SeverityText,
			int64(lr.SeverityNumber),
			lr.ServiceName,
			lr.Body,
			lr.ResourceSchemaUrl,
			resourceJSON,
			lr.ScopeSchemaUrl,
			lr.ScopeName,
			lr.ScopeVersion,
			scopeJSON,
			logJSON,
		})
	}
	return insertRows(ctx, "log_records", logRecordColumns, rows)
}

func (r *logRecordRepository) Search(ctx context.Context, params shared.LogSearchParams) ([]models.LogRecord, int64, error) {
	where, args := r.buildWhere(params)

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

	// COUNT(*) OVER () counts the filtered rows before LIMIT, so one scan
	// yields both the page and the total.
	query := fmt.Sprintf(`SELECT id, project_id, timestamp, trace_id, span_id, trace_flags,
		severity_text, severity_number, service_name, body,
		resource_schema_url, resource_attributes,
		scope_schema_url, scope_name, scope_version, scope_attributes,
		log_attributes, COUNT(*) OVER () AS total
	FROM log_records
	WHERE %s
	ORDER BY %s %s, id
	LIMIT :limit OFFSET :offset`, where, orderBy, direction)

	rows, err := lit.SelectNamed[logRecord](db.TelemetryDB, query, args)
	if err != nil {
		return nil, 0, err
	}

	count := int64(0)
	if len(rows) > 0 {
		count = rows[0].Total
	} else if params.Page > 1 {
		// An offset past the last row returns no rows and thus no window
		// total; only then fall back to a separate count scan.
		countQuery, countArgs := "SELECT COUNT(*) AS count FROM log_records WHERE "+where, args
		if isIndexCountable(params) {
			// Minute-aligned so log_records_count_idx serves the count; the
			// snap can shift the total by <60s of edge rows (see indexMinuteRange).
			countQuery, countArgs = r.indexCountQuery(params)
		}
		countResult, err := lit.SelectSingleNamed[fbCountResult](db.TelemetryDB, countQuery, countArgs)
		if err != nil {
			return nil, 0, err
		}
		if countResult != nil {
			count = int64(countResult.Count)
		}
	}

	records := make([]models.LogRecord, 0, len(rows))
	for _, row := range rows {
		records = append(records, row.toModel())
	}
	return records, count, nil
}

// isIndexCountable reports whether the search filters reference only
// log_records_count_idx keys (project_id, the minute-truncated timestamp,
// service_name equality), so the standalone COUNT(*) can merge index states
// instead of scanning rows. Body search, trace/span/scope filters, attribute
// filters, and MinSeverity (which compares severity_number, a non-key column)
// all reference raw columns and keep the raw count. severity_text is an index
// key, but buildWhere never filters on it, so it needs no check here.
func isIndexCountable(params shared.LogSearchParams) bool {
	return params.MinSeverity == 0 &&
		params.TraceId == "" && len(params.TraceIds) == 0 &&
		params.SpanId == "" && params.ScopeName == "" &&
		params.Body == "" && len(params.AttributeFilters) == 0 &&
		params.Search == ""
}

// indexCountQuery builds the index-served count: every referenced expression
// is a log_records_count_idx key, with the time filter minute-truncated via
// indexMinuteRange (raw timestamp would force a full scan).
func (r *logRecordRepository) indexCountQuery(params shared.LogSearchParams) (string, lit.P) {
	where := "project_id = :project_id AND " + indexMinuteRange("timestamp")
	args := minuteRangeParams(params.ProjectId, params.FromDate, params.ToDate)
	if params.ServiceName != "" {
		where += " AND service_name = :service_name"
		args["service_name"] = params.ServiceName
	}
	return "SELECT COUNT(*) AS count FROM log_records WHERE " + where, args
}

func (r *logRecordRepository) FindByTraceId(ctx context.Context, projectId uuid.UUID, traceId string) ([]models.LogRecord, error) {
	rows, err := lit.SelectNamed[logRecord](db.TelemetryDB,
		`SELECT id, project_id, timestamp, trace_id, span_id, trace_flags,
			severity_text, severity_number, service_name, body,
			resource_schema_url, resource_attributes,
			scope_schema_url, scope_name, scope_version, scope_attributes,
			log_attributes
		FROM log_records
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
		"from":       params.FromDate.UTC(),
		"to":         params.ToDate.UTC(),
	}

	if params.MinSeverity > 0 {
		clauses = append(clauses, "severity_number >= :min_severity")
		args["min_severity"] = params.MinSeverity
	}
	if params.ServiceName != "" {
		clauses = append(clauses, "service_name = :service_name")
		args["service_name"] = params.ServiceName
	}
	if len(params.TraceIds) > 0 {
		placeholders := make([]string, len(params.TraceIds))
		for i, tid := range params.TraceIds {
			key := fmt.Sprintf("tid%d", i)
			placeholders[i] = ":" + key
			args[key] = tid
		}
		clauses = append(clauses, "trace_id IN ("+strings.Join(placeholders, ", ")+")")
	} else if params.TraceId != "" {
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
			// (JSON_POINTER_EXTRACT_TEXT yields NULL there, and STRPOS on NULL is NULL).
			expr := fmt.Sprintf("STRPOS(LOWER(COALESCE(%s, '')), LOWER(:%s))", jsonExtractExpr(col, ":"+keyPH), valPH)
			if f.Exclude {
				clauses = append(clauses, expr+" = 0")
			} else {
				clauses = append(clauses, expr+" > 0")
			}
		} else if f.Exclude {
			// COALESCE keeps rows that don't carry the attribute at all
			// (JSON_POINTER_EXTRACT_TEXT yields NULL there, and NULL != value is NULL).
			clauses = append(clauses,
				fmt.Sprintf("COALESCE(%s, '') != :%s", jsonExtractExpr(col, ":"+keyPH), valPH))
		} else {
			clauses = append(clauses,
				fmt.Sprintf("%s = :%s", jsonExtractExpr(col, ":"+keyPH), valPH))
		}
		args[keyPH] = jsonPointerEscape(f.Key)
		args[valPH] = f.Value
	}

	if params.Search != "" {
		switch params.SearchType {
		case "service":
			clauses = append(clauses, "STRPOS(LOWER(service_name), LOWER(:search)) > 0")
			args["search"] = params.Search
		case "trace":
			if _, exists := args["trace_id"]; !exists {
				clauses = append(clauses, "trace_id = :search")
				args["search"] = params.Search
			}
		default:
			clauses = append(clauses, "STRPOS(LOWER(body), LOWER(:search)) > 0")
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
