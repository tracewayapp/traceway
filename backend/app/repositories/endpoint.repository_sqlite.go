//go:build !pgch

package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/lit/v2"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
)

type endpoint struct {
	Id                 uuid.UUID     `lit:"id"`
	ProjectId          uuid.UUID     `lit:"project_id"`
	Endpoint           string        `lit:"endpoint"`
	Duration           int64         `lit:"duration"`
	RecordedAt         SQLiteTime    `lit:"recorded_at"`
	StatusCode         int16         `lit:"status_code"`
	BodySize           int32         `lit:"body_size"`
	ClientIP           string        `lit:"client_ip"`
	Attributes         SQLiteJSONMap `lit:"attributes"`
	AppVersion         string        `lit:"app_version"`
	ServerName         string        `lit:"server_name"`
	DistributedTraceId *uuid.UUID    `lit:"distributed_trace_id"`
	SpanId             *uuid.UUID    `lit:"span_id"`
	IsStream           bool          `lit:"is_stream"`
	IsRoot             bool          `lit:"is_root"`
}

type groupedEndpointRow struct {
	Endpoint         string  `lit:"endpoint"`
	TotalCount       uint64  `lit:"total_count"`
	AvgDuration      float64 `lit:"avg_duration"`
	LastSeen         string  `lit:"last_seen"`
	OffsetMs         uint32  `lit:"offset_ms"`
	ServerErrorCount uint64  `lit:"server_error_count"`
	ClientErrorCount uint64  `lit:"client_error_count"`
	SatisfiedCount   uint64  `lit:"satisfied_count"`
	ToleratingCount  uint64  `lit:"tolerating_count"`
	BadCount         uint64  `lit:"bad_count"`
	IsStream         bool    `lit:"is_stream"`
	HasRoot          bool    `lit:"has_root"`
	HasNonRoot       bool    `lit:"has_non_root"`
}

type endpointDurationRow struct {
	Duration float64 `lit:"duration"`
}

type slowEndpointRow struct {
	OffsetMs uint32 `lit:"offset_ms"`
	Reason   string `lit:"reason"`
}

type endpointMetricRow struct {
	Endpoint    string  `lit:"endpoint"`
	MetricValue float64 `lit:"metric_value"`
}

type distinctEndpointRow struct {
	Endpoint string `lit:"endpoint"`
}

type endpointDetailStatsRow struct {
	Count               int64   `lit:"count"`
	AvgDurationMs       float64 `lit:"avg_duration_ms"`
	ErrorRate           float64 `lit:"error_rate"`
	SatisfiedTolerating float64 `lit:"satisfied_tolerating"`
}

type isStreamFlagRow struct {
	IsStream bool `lit:"is_stream"`
}

func init() {
	models.ExtensionModelRegistrations = append(models.ExtensionModelRegistrations, func(driver lit.Driver) {
		lit.RegisterModel[endpoint](driver)
		lit.RegisterModel[groupedEndpointRow](driver)
		lit.RegisterModel[endpointDurationRow](driver)
		lit.RegisterModel[slowEndpointRow](driver)
		lit.RegisterModel[endpointMetricRow](driver)
		lit.RegisterModel[distinctEndpointRow](driver)
		lit.RegisterModel[endpointDetailStatsRow](driver)
		lit.RegisterModel[isStreamFlagRow](driver)
	})
}

func endpointToRow(e models.Endpoint) endpoint {
	return endpoint{
		Id:                 e.Id,
		ProjectId:          e.ProjectId,
		Endpoint:           e.Endpoint,
		Duration:           int64(e.Duration),
		RecordedAt:         NewSQLiteTime(e.RecordedAt),
		StatusCode:         e.StatusCode,
		BodySize:           e.BodySize,
		ClientIP:           e.ClientIP,
		Attributes:         NewSQLiteJSONMap(e.Attributes),
		AppVersion:         e.AppVersion,
		ServerName:         e.ServerName,
		DistributedTraceId: e.DistributedTraceId,
		SpanId:             e.SpanId,
		IsStream:           e.IsStream,
		IsRoot:             e.IsRoot,
	}
}

func (r *endpoint) toModel() models.Endpoint {
	e := models.Endpoint{
		Id:                 r.Id,
		ProjectId:          r.ProjectId,
		Endpoint:           r.Endpoint,
		Duration:           time.Duration(r.Duration),
		RecordedAt:         r.RecordedAt.Time,
		StatusCode:         r.StatusCode,
		BodySize:           r.BodySize,
		ClientIP:           r.ClientIP,
		AppVersion:         r.AppVersion,
		ServerName:         r.ServerName,
		DistributedTraceId: r.DistributedTraceId,
		SpanId:             r.SpanId,
		IsStream:           r.IsStream,
		IsRoot:             r.IsRoot,
	}
	if r.Attributes != nil {
		e.Attributes = map[string]string(r.Attributes)
	}
	return e
}

// rootFilterClause returns a SQL fragment ("", " AND <col> = 1", " AND <col> = 0")
// to splice into a WHERE clause based on the rootFilter param. Accepts "all" |
// "root" | "non_root"; defaults to "all" (no filter).
func rootFilterClause(qualifiedCol, rootFilter string) string {
	switch rootFilter {
	case "root":
		return " AND " + qualifiedCol + " = 1"
	case "non_root":
		return " AND " + qualifiedCol + " = 0"
	default:
		return ""
	}
}

type endpointRepository struct{}

func (e *endpointRepository) InsertAsync(ctx context.Context, lines []models.Endpoint) error {
	if len(lines) == 0 {
		return nil
	}

	tx, err := db.TelemetryDB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, ep := range lines {
		row := endpointToRow(ep)
		if err := lit.InsertExistingUuid(tx, &row); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (e *endpointRepository) CountBetween(ctx context.Context, projectId uuid.UUID, start, end time.Time) (int64, error) {
	result, err := lit.SelectSingleNamed[models.CountResult](db.TelemetryDB,
		"SELECT COUNT(*) AS count FROM endpoints WHERE project_id = :project_id AND recorded_at >= :from AND recorded_at <= :to",
		lit.P{"project_id": projectId, "from": NewSQLiteTime(start), "to": NewSQLiteTime(end)})
	if err != nil {
		return 0, err
	}
	if result == nil {
		return 0, nil
	}
	return int64(result.Count), nil
}

func (e *endpointRepository) FindAll(ctx context.Context, projectId uuid.UUID, fromDate, toDate time.Time, page, pageSize int, orderBy string) ([]models.Endpoint, int64, error) {
	params := lit.P{"project_id": projectId, "from": NewSQLiteTime(fromDate), "to": NewSQLiteTime(toDate)}

	countResult, err := lit.SelectSingleNamed[models.CountResult](db.TelemetryDB,
		"SELECT COUNT(*) AS count FROM endpoints WHERE project_id = :project_id AND recorded_at >= :from AND recorded_at <= :to",
		params)
	if err != nil {
		return nil, 0, err
	}
	count := int64(0)
	if countResult != nil {
		count = int64(countResult.Count)
	}

	offset := (page - 1) * pageSize

	allowedOrderBy := map[string]bool{"recorded_at": true, "duration": true, "status_code": true, "body_size": true}
	if !allowedOrderBy[orderBy] {
		orderBy = "recorded_at"
	}

	rows, err := lit.SelectNamed[endpoint](db.TelemetryDB,
		fmt.Sprintf(`SELECT id, project_id, endpoint, duration, recorded_at, status_code, body_size, client_ip, attributes, app_version, server_name, distributed_trace_id
		FROM endpoints WHERE project_id = :project_id AND recorded_at >= :from AND recorded_at <= :to
		ORDER BY %s DESC LIMIT :limit OFFSET :offset`, orderBy),
		lit.P{"project_id": projectId, "from": NewSQLiteTime(fromDate), "to": NewSQLiteTime(toDate), "limit": pageSize, "offset": offset})
	if err != nil {
		return nil, 0, err
	}

	endpoints := make([]models.Endpoint, 0, len(rows))
	for _, row := range rows {
		endpoints = append(endpoints, row.toModel())
	}

	return endpoints, count, nil
}

func (e *endpointRepository) FindGroupedByEndpoint(ctx context.Context, projectId uuid.UUID, fromDate, toDate time.Time, page, pageSize int, orderBy string, sortDirection string, search string, rootFilter string) ([]models.EndpointStats, int64, error) {
	params := lit.P{"project_id": projectId, "from": NewSQLiteTime(fromDate), "to": NewSQLiteTime(toDate)}

	whereClause := "e.project_id = :project_id AND e.recorded_at >= :from AND e.recorded_at <= :to"
	if search != "" {
		whereClause += " AND INSTR(LOWER(e.endpoint), LOWER(:search)) > 0"
		params["search"] = search
	}
	whereClause += rootFilterClause("e.is_root", rootFilter)

	countQuery := "SELECT COUNT(DISTINCT e.endpoint) AS count FROM endpoints e WHERE " + whereClause
	totalResult, err := lit.SelectSingleNamed[models.CountResult](db.TelemetryDB, countQuery, params)
	if err != nil {
		return nil, 0, err
	}
	totalEndpoints := int64(0)
	if totalResult != nil {
		totalEndpoints = int64(totalResult.Count)
	}

	groupQuery := `SELECT
		e.endpoint,
		COUNT(*) as total_count,
		AVG(e.duration) as avg_duration,
		MAX(e.recorded_at) as last_seen,
		COALESCE(s.offset_ms, 0) as offset_ms,
		SUM(CASE WHEN e.status_code >= 500 THEN 1 ELSE 0 END) as server_error_count,
		SUM(CASE WHEN e.status_code >= 400 AND e.status_code < 500 THEN 1 ELSE 0 END) as client_error_count,
		SUM(CASE WHEN e.duration <= (750000000 + COALESCE(s.offset_ms, 0) * 1000000) AND e.status_code < 500 THEN 1 ELSE 0 END) as satisfied_count,
		SUM(CASE WHEN e.duration > (750000000 + COALESCE(s.offset_ms, 0) * 1000000) AND e.duration <= (1500000000 + COALESCE(s.offset_ms, 0) * 1000000) AND e.status_code < 500 THEN 1 ELSE 0 END) as tolerating_count,
		SUM(CASE WHEN e.duration > (1500000000 + COALESCE(s.offset_ms, 0) * 1000000) OR e.status_code >= 500 THEN 1 ELSE 0 END) as bad_count,
		MAX(e.is_stream) as is_stream,
		MAX(e.is_root) as has_root,
		MAX(CASE WHEN e.is_root = 0 THEN 1 ELSE 0 END) as has_non_root
	FROM endpoints e
	LEFT JOIN slow_endpoints s ON e.endpoint = s.endpoint AND e.project_id = s.project_id
	WHERE ` + whereClause + `
	GROUP BY e.endpoint`

	parsedQuery, args, err := lit.ParseNamedQuery(db.Driver, groupQuery, params)
	if err != nil {
		return nil, 0, err
	}

	sqlRows, err := db.TelemetryDB.QueryContext(ctx, parsedQuery, args...)
	if err != nil {
		return nil, 0, err
	}
	defer sqlRows.Close()

	var groups []groupedEndpointRow
	for sqlRows.Next() {
		var g groupedEndpointRow
		if err := sqlRows.Scan(&g.Endpoint, &g.TotalCount, &g.AvgDuration, &g.LastSeen,
			&g.OffsetMs, &g.ServerErrorCount, &g.ClientErrorCount,
			&g.SatisfiedCount, &g.ToleratingCount, &g.BadCount, &g.IsStream, &g.HasRoot, &g.HasNonRoot); err != nil {
			return nil, 0, err
		}
		groups = append(groups, g)
	}

	var stats []models.EndpointStats
	for _, g := range groups {
		lastSeen, _ := time.Parse(time.RFC3339Nano, g.LastSeen)

		// Streaming endpoints are still surfaced (count, error rate, throughput),
		// but their connection lifetime is not request latency — zero out the
		// latency signals. Impact still fires on status-code-driven components
		// (server-error rate, client-error rate, volume-weighted error rate)
		// so a streaming endpoint returning lots of 5xx still ranks.
		if g.IsStream {
			stats = append(stats, models.EndpointStats{
				Endpoint:     g.Endpoint,
				Count:        g.TotalCount,
				LastSeen:     lastSeen,
				Impact:       ComputeStreamImpact(g.Endpoint, g.TotalCount, g.ServerErrorCount, g.ClientErrorCount),
				ImpactReason: ComputeStreamImpactReason(g.Endpoint, g.TotalCount, g.ServerErrorCount, g.ClientErrorCount),
				IsStream:     true,
				HasRoot:      g.HasRoot,
				HasNonRoot:   g.HasNonRoot,
			})
			continue
		}

		durations, err := fetchSortedDurations(ctx, projectId, g.Endpoint, fromDate, toDate)
		if err != nil {
			return nil, 0, err
		}

		p50 := computePercentile(durations, 0.5)
		p95 := computePercentile(durations, 0.95)
		p99 := computePercentile(durations, 0.99)

		impact := ComputeImpactScore(g.Endpoint, g.TotalCount, g.SatisfiedCount, g.ToleratingCount, g.BadCount, g.ClientErrorCount, p99, g.OffsetMs)

		stats = append(stats, models.EndpointStats{
			Endpoint:    g.Endpoint,
			Count:       g.TotalCount,
			P50Duration: time.Duration(p50),
			P95Duration: time.Duration(p95),
			P99Duration: time.Duration(p99),
			AvgDuration: time.Duration(g.AvgDuration),
			LastSeen:    lastSeen,
			Impact:      impact,
			ImpactReason: ComputeImpactReason(g.Endpoint, g.TotalCount, g.SatisfiedCount, g.ToleratingCount,
				g.BadCount, g.ClientErrorCount, p99, g.OffsetMs),
			HasRoot:    g.HasRoot,
			HasNonRoot: g.HasNonRoot,
		})
	}

	sortEndpointStats(stats, orderBy, sortDirection)

	start := (page - 1) * pageSize
	if start > len(stats) {
		start = len(stats)
	}
	endIdx := start + pageSize
	if endIdx > len(stats) {
		endIdx = len(stats)
	}

	return stats[start:endIdx], totalEndpoints, nil
}

func (e *endpointRepository) FindByEndpoint(ctx context.Context, projectId uuid.UUID, endpointName string, fromDate, toDate time.Time, page, pageSize int, orderBy string, sortDirection string) ([]models.Endpoint, int64, error) {
	params := lit.P{"project_id": projectId, "endpoint": endpointName, "from": NewSQLiteTime(fromDate), "to": NewSQLiteTime(toDate)}

	countResult, err := lit.SelectSingleNamed[models.CountResult](db.TelemetryDB,
		"SELECT COUNT(*) AS count FROM endpoints WHERE project_id = :project_id AND endpoint = :endpoint AND recorded_at >= :from AND recorded_at <= :to",
		params)
	if err != nil {
		return nil, 0, err
	}
	count := int64(0)
	if countResult != nil {
		count = int64(countResult.Count)
	}

	offset := (page - 1) * pageSize

	allowedOrderBy := map[string]bool{"recorded_at": true, "duration": true, "status_code": true, "body_size": true}
	if !allowedOrderBy[orderBy] {
		orderBy = "recorded_at"
	}

	sortDir := "DESC"
	if sortDirection == "asc" {
		sortDir = "ASC"
	}

	rows, err := lit.SelectNamed[endpoint](db.TelemetryDB,
		fmt.Sprintf(`SELECT id, project_id, endpoint, duration, recorded_at, status_code, body_size, client_ip, attributes, app_version, server_name, distributed_trace_id
		FROM endpoints WHERE project_id = :project_id AND endpoint = :endpoint AND recorded_at >= :from AND recorded_at <= :to
		ORDER BY %s %s LIMIT :limit OFFSET :offset`, orderBy, sortDir),
		lit.P{"project_id": projectId, "endpoint": endpointName, "from": NewSQLiteTime(fromDate), "to": NewSQLiteTime(toDate), "limit": pageSize, "offset": offset})
	if err != nil {
		return nil, 0, err
	}

	endpoints := make([]models.Endpoint, 0, len(rows))
	for _, row := range rows {
		endpoints = append(endpoints, row.toModel())
	}

	return endpoints, count, nil
}

func (e *endpointRepository) FindById(ctx context.Context, projectId, endpointId uuid.UUID, recordedAt *time.Time) (*models.Endpoint, error) {
	query := `SELECT id, project_id, endpoint, duration, recorded_at, status_code, body_size, client_ip, attributes, app_version, server_name, distributed_trace_id, span_id
		FROM endpoints WHERE project_id = :project_id AND id = :id`
	params := lit.P{"project_id": projectId, "id": endpointId}
	if recordedAt != nil {
		from, to := traceWindowBounds(*recordedAt)
		query += ` AND recorded_at >= :from AND recorded_at <= :to`
		params["from"] = NewSQLiteTime(from)
		params["to"] = NewSQLiteTime(to)
	}
	query += ` LIMIT 1`

	row, err := lit.SelectSingleNamed[endpoint](db.TelemetryDB, query, params)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, nil
	}
	ep := row.toModel()
	return &ep, nil
}

func (e *endpointRepository) CountByHour(ctx context.Context, projectId uuid.UUID, start, end time.Time) ([]models.TimeSeriesPoint, error) {
	results, err := lit.SelectNamed[timeSeriesResult](db.TelemetryDB,
		`SELECT strftime('%Y-%m-%d %H:00:00', recorded_at) as bucket, CAST(COUNT(*) AS REAL) as agg_value
		FROM endpoints WHERE project_id = :project_id AND recorded_at >= :from AND recorded_at <= :to
		GROUP BY bucket ORDER BY bucket ASC`,
		lit.P{"project_id": projectId, "from": NewSQLiteTime(start), "to": NewSQLiteTime(end)})
	if err != nil {
		return nil, err
	}
	return timeSeriesResultsToPoints(results), nil
}

func (e *endpointRepository) AvgDurationByHour(ctx context.Context, projectId uuid.UUID, start, end time.Time) ([]models.TimeSeriesPoint, error) {
	results, err := lit.SelectNamed[timeSeriesResult](db.TelemetryDB,
		`SELECT strftime('%Y-%m-%d %H:00:00', recorded_at) as bucket, AVG(duration) / 1000000.0 as agg_value
		FROM endpoints WHERE project_id = :project_id AND recorded_at >= :from AND recorded_at <= :to AND is_stream = 0
		GROUP BY bucket ORDER BY bucket ASC`,
		lit.P{"project_id": projectId, "from": NewSQLiteTime(start), "to": NewSQLiteTime(end)})
	if err != nil {
		return nil, err
	}
	return timeSeriesResultsToPoints(results), nil
}

func (e *endpointRepository) ErrorRateByHour(ctx context.Context, projectId uuid.UUID, start, end time.Time) ([]models.TimeSeriesPoint, error) {
	results, err := lit.SelectNamed[timeSeriesResult](db.TelemetryDB,
		`SELECT strftime('%Y-%m-%d %H:00:00', recorded_at) as bucket,
		SUM(CASE WHEN status_code >= 500 THEN 1 ELSE 0 END) * 100.0 / COUNT(*) as agg_value
		FROM endpoints WHERE project_id = :project_id AND recorded_at >= :from AND recorded_at <= :to
		GROUP BY bucket ORDER BY bucket ASC`,
		lit.P{"project_id": projectId, "from": NewSQLiteTime(start), "to": NewSQLiteTime(end)})
	if err != nil {
		return nil, err
	}
	return timeSeriesResultsToPoints(results), nil
}

func (e *endpointRepository) CountByInterval(ctx context.Context, projectId uuid.UUID, start, end time.Time, intervalMinutes int) ([]models.TimeSeriesPoint, error) {
	secs := intervalMinutes * 60
	results, err := lit.SelectNamed[timeSeriesResult](db.TelemetryDB,
		fmt.Sprintf(`SELECT datetime((strftime('%%s', recorded_at) / %d) * %d, 'unixepoch') as bucket, CAST(COUNT(*) AS REAL) as agg_value
		FROM endpoints WHERE project_id = :project_id AND recorded_at >= :from AND recorded_at <= :to
		GROUP BY bucket ORDER BY bucket ASC`, secs, secs),
		lit.P{"project_id": projectId, "from": NewSQLiteTime(start), "to": NewSQLiteTime(end)})
	if err != nil {
		return nil, err
	}
	return timeSeriesResultsToPoints(results), nil
}

func (e *endpointRepository) AvgDurationByInterval(ctx context.Context, projectId uuid.UUID, start, end time.Time, intervalMinutes int) ([]models.TimeSeriesPoint, error) {
	secs := intervalMinutes * 60
	results, err := lit.SelectNamed[timeSeriesResult](db.TelemetryDB,
		fmt.Sprintf(`SELECT datetime((strftime('%%s', recorded_at) / %d) * %d, 'unixepoch') as bucket, AVG(duration) / 1000000.0 as agg_value
		FROM endpoints WHERE project_id = :project_id AND recorded_at >= :from AND recorded_at <= :to AND is_stream = 0
		GROUP BY bucket ORDER BY bucket ASC`, secs, secs),
		lit.P{"project_id": projectId, "from": NewSQLiteTime(start), "to": NewSQLiteTime(end)})
	if err != nil {
		return nil, err
	}
	return timeSeriesResultsToPoints(results), nil
}

func (e *endpointRepository) ErrorRateByInterval(ctx context.Context, projectId uuid.UUID, start, end time.Time, intervalMinutes int) ([]models.TimeSeriesPoint, error) {
	secs := intervalMinutes * 60
	results, err := lit.SelectNamed[timeSeriesResult](db.TelemetryDB,
		fmt.Sprintf(`SELECT datetime((strftime('%%s', recorded_at) / %d) * %d, 'unixepoch') as bucket,
		SUM(CASE WHEN status_code >= 500 THEN 1 ELSE 0 END) * 100.0 / COUNT(*) as agg_value
		FROM endpoints WHERE project_id = :project_id AND recorded_at >= :from AND recorded_at <= :to
		GROUP BY bucket ORDER BY bucket ASC`, secs, secs),
		lit.P{"project_id": projectId, "from": NewSQLiteTime(start), "to": NewSQLiteTime(end)})
	if err != nil {
		return nil, err
	}
	return timeSeriesResultsToPoints(results), nil
}

func (e *endpointRepository) FindWorstEndpoints(ctx context.Context, projectId uuid.UUID, start, end time.Time, limit int) ([]models.EndpointStats, error) {
	params := lit.P{"project_id": projectId, "from": NewSQLiteTime(start), "to": NewSQLiteTime(end)}

	groupQuery := `SELECT
		e.endpoint,
		COUNT(*) as total_count,
		AVG(e.duration) as avg_duration,
		MAX(e.recorded_at) as last_seen,
		COALESCE(s.offset_ms, 0) as offset_ms,
		SUM(CASE WHEN e.status_code >= 500 THEN 1 ELSE 0 END) as server_error_count,
		SUM(CASE WHEN e.status_code >= 400 AND e.status_code < 500 THEN 1 ELSE 0 END) as client_error_count,
		SUM(CASE WHEN e.duration <= (750000000 + COALESCE(s.offset_ms, 0) * 1000000) AND e.status_code < 500 THEN 1 ELSE 0 END) as satisfied_count,
		SUM(CASE WHEN e.duration > (750000000 + COALESCE(s.offset_ms, 0) * 1000000) AND e.duration <= (1500000000 + COALESCE(s.offset_ms, 0) * 1000000) AND e.status_code < 500 THEN 1 ELSE 0 END) as tolerating_count,
		SUM(CASE WHEN e.duration > (1500000000 + COALESCE(s.offset_ms, 0) * 1000000) OR e.status_code >= 500 THEN 1 ELSE 0 END) as bad_count,
		MAX(e.is_stream) as is_stream
	FROM endpoints e
	LEFT JOIN slow_endpoints s ON e.endpoint = s.endpoint AND e.project_id = s.project_id
	WHERE e.project_id = :project_id AND e.recorded_at >= :from AND e.recorded_at <= :to AND e.is_stream = 0
	GROUP BY e.endpoint`

	parsedQuery, args, err := lit.ParseNamedQuery(db.Driver, groupQuery, params)
	if err != nil {
		return nil, err
	}

	sqlRows, err := db.TelemetryDB.QueryContext(ctx, parsedQuery, args...)
	if err != nil {
		return nil, err
	}
	defer sqlRows.Close()

	var groups []groupedEndpointRow
	for sqlRows.Next() {
		var g groupedEndpointRow
		if err := sqlRows.Scan(&g.Endpoint, &g.TotalCount, &g.AvgDuration, &g.LastSeen,
			&g.OffsetMs, &g.ServerErrorCount, &g.ClientErrorCount,
			&g.SatisfiedCount, &g.ToleratingCount, &g.BadCount, &g.IsStream); err != nil {
			return nil, err
		}
		groups = append(groups, g)
	}

	var stats []models.EndpointStats
	for _, g := range groups {
		durations, err := fetchSortedDurations(ctx, projectId, g.Endpoint, start, end)
		if err != nil {
			return nil, err
		}

		p50 := computePercentile(durations, 0.5)
		p95 := computePercentile(durations, 0.95)
		p99 := computePercentile(durations, 0.99)

		impact := ComputeImpactScore(g.Endpoint, g.TotalCount, g.SatisfiedCount, g.ToleratingCount, g.BadCount, g.ClientErrorCount, p99, g.OffsetMs)

		lastSeen, _ := time.Parse(time.RFC3339Nano, g.LastSeen)

		stats = append(stats, models.EndpointStats{
			Endpoint:    g.Endpoint,
			Count:       g.TotalCount,
			P50Duration: time.Duration(p50),
			P95Duration: time.Duration(p95),
			P99Duration: time.Duration(p99),
			AvgDuration: time.Duration(g.AvgDuration),
			LastSeen:    lastSeen,
			Impact:      impact,
			ImpactReason: ComputeImpactReason(g.Endpoint, g.TotalCount, g.SatisfiedCount, g.ToleratingCount,
				g.BadCount, g.ClientErrorCount, p99, g.OffsetMs),
		})
	}

	sort.Slice(stats, func(i, j int) bool {
		return stats[i].Impact > stats[j].Impact
	})

	if limit > 0 && limit < len(stats) {
		stats = stats[:limit]
	}

	return stats, nil
}

func (e *endpointRepository) GetEndpointStats(ctx context.Context, projectId uuid.UUID, endpoint string, start, end time.Time) (*models.EndpointDetailStats, error) {
	params := lit.P{"project_id": projectId, "endpoint": endpoint, "from": NewSQLiteTime(start), "to": NewSQLiteTime(end)}

	durationMinutes := end.Sub(start).Minutes()
	if durationMinutes < 1 {
		durationMinutes = 1
	}

	row, err := lit.SelectSingleNamed[endpointDetailStatsRow](db.TelemetryDB,
		`SELECT
			COUNT(*) as count,
			CASE WHEN COUNT(*) > 0 THEN AVG(duration) / 1000000.0 ELSE 0 END as avg_duration_ms,
			CASE WHEN COUNT(*) > 0 THEN SUM(CASE WHEN status_code >= 500 THEN 1 ELSE 0 END) * 100.0 / COUNT(*) ELSE 0 END as error_rate,
			SUM(CASE WHEN duration <= 500000000 AND status_code < 500 THEN 1 ELSE 0 END) +
				SUM(CASE WHEN duration > 500000000 AND duration <= 2000000000 AND status_code < 500 THEN 1 ELSE 0 END) * 0.5 as satisfied_tolerating
		FROM endpoints WHERE project_id = :project_id AND endpoint = :endpoint AND recorded_at >= :from AND recorded_at <= :to`,
		params)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return &models.EndpointDetailStats{}, nil
	}

	isStreamRow, err := lit.SelectSingleNamed[isStreamFlagRow](db.TelemetryDB,
		`SELECT MAX(is_stream) as is_stream FROM endpoints WHERE project_id = :project_id AND endpoint = :endpoint AND recorded_at >= :from AND recorded_at <= :to`,
		params)
	if err != nil {
		return nil, err
	}
	isStream := isStreamRow != nil && isStreamRow.IsStream

	var stats models.EndpointDetailStats
	stats.Count = row.Count
	stats.ErrorRate = row.ErrorRate
	stats.Throughput = float64(row.Count) / durationMinutes
	stats.IsStream = isStream

	if !isStream {
		stats.AvgDuration = row.AvgDurationMs
		if row.Count > 0 {
			stats.Apdex = row.SatisfiedTolerating / float64(row.Count)
		}

		durations, err := fetchSortedDurations(ctx, projectId, endpoint, start, end)
		if err != nil {
			return nil, err
		}

		if len(durations) > 0 {
			stats.MedianDuration = computePercentile(durations, 0.5) / 1000000.0
			stats.P95Duration = computePercentile(durations, 0.95) / 1000000.0
			stats.P99Duration = computePercentile(durations, 0.99) / 1000000.0
		}
	}

	return &stats, nil
}

func (e *endpointRepository) GetEndpointStackedChart(ctx context.Context, projectId uuid.UUID, start, end time.Time, intervalMinutes int, metricType string) (*models.EndpointStackedChartResponse, error) {
	params := lit.P{"project_id": projectId, "from": NewSQLiteTime(start), "to": NewSQLiteTime(end)}

	topEndpoints, err := getTopEndpointsByMetric(ctx, projectId, start, end, metricType)
	if err != nil {
		return nil, err
	}

	if len(topEndpoints) == 0 {
		return &models.EndpointStackedChartResponse{
			Endpoints: []string{},
			Series:    []models.EndpointTimeSeriesPoint{},
		}, nil
	}

	secs := intervalMinutes * 60

	if metricType == "p50" || metricType == "p95" || metricType == "p99" || metricType == "" {
		return e.getStackedChartWithPercentiles(ctx, projectId, start, end, secs, topEndpoints, metricType)
	}

	caseExpr := "CASE "
	for i, ep := range topEndpoints {
		key := fmt.Sprintf("ep_%d", i)
		caseExpr += fmt.Sprintf("WHEN endpoint = :%s THEN :%s ", key, key)
		params[key] = ep
	}
	caseExpr += "ELSE 'Other' END"

	var metricExpr string
	switch metricType {
	case "total_time":
		metricExpr = "COUNT(*) * AVG(duration) / 1000000.0"
	default:
		metricExpr = "AVG(duration) / 1000000.0"
	}

	timeSeriesQuery := fmt.Sprintf(`SELECT
		datetime((strftime('%%s', recorded_at) / %d) * %d, 'unixepoch') as bucket,
		%s as endpoint_category,
		%s as metric_value
	FROM endpoints
	WHERE project_id = :project_id AND recorded_at >= :from AND recorded_at <= :to AND is_stream = 0
	GROUP BY bucket, endpoint_category
	ORDER BY bucket ASC, endpoint_category ASC`, secs, secs, caseExpr, metricExpr)

	parsedQuery, args, err := lit.ParseNamedQuery(db.Driver, timeSeriesQuery, params)
	if err != nil {
		return nil, err
	}

	sqlRows, err := db.TelemetryDB.QueryContext(ctx, parsedQuery, args...)
	if err != nil {
		return nil, err
	}
	defer sqlRows.Close()

	var series []models.EndpointTimeSeriesPoint
	for sqlRows.Next() {
		var p models.EndpointTimeSeriesPoint
		var tsStr string
		if err := sqlRows.Scan(&tsStr, &p.Endpoint, &p.Value); err != nil {
			return nil, err
		}
		p.Timestamp, _ = time.Parse("2006-01-02 15:04:05", tsStr)
		series = append(series, p)
	}

	endpointSet := make(map[string]bool)
	for _, p := range series {
		endpointSet[p.Endpoint] = true
	}

	finalEndpoints := make([]string, 0, len(topEndpoints)+1)
	finalEndpoints = append(finalEndpoints, topEndpoints...)
	if endpointSet["Other"] {
		finalEndpoints = append(finalEndpoints, "Other")
	}

	return &models.EndpointStackedChartResponse{
		Endpoints: finalEndpoints,
		Series:    series,
	}, nil
}

func (e *endpointRepository) getStackedChartWithPercentiles(ctx context.Context, projectId uuid.UUID, start, end time.Time, bucketSecs int, topEndpoints []string, metricType string) (*models.EndpointStackedChartResponse, error) {
	percentile := 0.5
	switch metricType {
	case "p95":
		percentile = 0.95
	case "p99":
		percentile = 0.99
	}

	topSet := make(map[string]bool, len(topEndpoints))
	for _, ep := range topEndpoints {
		topSet[ep] = true
	}

	query := fmt.Sprintf(`SELECT
		datetime((strftime('%%s', recorded_at) / %d) * %d, 'unixepoch') as bucket,
		endpoint,
		duration
	FROM endpoints
	WHERE project_id = :project_id AND recorded_at >= :from AND recorded_at <= :to AND is_stream = 0
	ORDER BY bucket ASC, endpoint ASC, duration ASC`, bucketSecs, bucketSecs)

	parsedQuery, args, err := lit.ParseNamedQuery(db.Driver, query,
		lit.P{"project_id": projectId, "from": NewSQLiteTime(start), "to": NewSQLiteTime(end)})
	if err != nil {
		return nil, err
	}

	sqlRows, err := db.TelemetryDB.QueryContext(ctx, parsedQuery, args...)
	if err != nil {
		return nil, err
	}
	defer sqlRows.Close()

	type bucketKey struct {
		bucket   string
		category string
	}
	durationMap := make(map[bucketKey][]float64)

	for sqlRows.Next() {
		var bucketStr, epName string
		var duration float64
		if err := sqlRows.Scan(&bucketStr, &epName, &duration); err != nil {
			return nil, err
		}

		category := "Other"
		if topSet[epName] {
			category = epName
		}

		key := bucketKey{bucket: bucketStr, category: category}
		durationMap[key] = append(durationMap[key], duration)
	}

	var series []models.EndpointTimeSeriesPoint
	for key, durations := range durationMap {
		sort.Float64s(durations)
		val := computePercentile(durations, percentile) / 1000000.0

		ts, _ := time.Parse("2006-01-02 15:04:05", key.bucket)
		series = append(series, models.EndpointTimeSeriesPoint{
			Timestamp: ts,
			Endpoint:  key.category,
			Value:     val,
		})
	}

	sort.Slice(series, func(i, j int) bool {
		if series[i].Timestamp.Equal(series[j].Timestamp) {
			return series[i].Endpoint < series[j].Endpoint
		}
		return series[i].Timestamp.Before(series[j].Timestamp)
	})

	endpointSet := make(map[string]bool)
	for _, p := range series {
		endpointSet[p.Endpoint] = true
	}

	finalEndpoints := make([]string, 0, len(topEndpoints)+1)
	finalEndpoints = append(finalEndpoints, topEndpoints...)
	if endpointSet["Other"] {
		finalEndpoints = append(finalEndpoints, "Other")
	}

	return &models.EndpointStackedChartResponse{
		Endpoints: finalEndpoints,
		Series:    series,
	}, nil
}

func (e *endpointRepository) GetSlowEndpoint(ctx context.Context, projectId uuid.UUID, endpoint string) (uint32, string, error) {
	result, err := lit.SelectSingleNamed[slowEndpointRow](db.TelemetryDB,
		"SELECT offset_ms, reason FROM slow_endpoints WHERE project_id = :project_id AND endpoint = :endpoint",
		lit.P{"project_id": projectId, "endpoint": endpoint})
	if err != nil {
		return 0, "", err
	}
	if result == nil {
		return 0, "", sql.ErrNoRows
	}
	return result.OffsetMs, result.Reason, nil
}

func (e *endpointRepository) UpsertSlowEndpoint(ctx context.Context, projectId uuid.UUID, endpoint string, offsetMs uint32, reason string) error {
	query, args, err := lit.ParseNamedQuery(db.Driver,
		"INSERT OR REPLACE INTO slow_endpoints (project_id, endpoint, offset_ms, reason) VALUES (:project_id, :endpoint, :offset_ms, :reason)",
		lit.P{"project_id": projectId, "endpoint": endpoint, "offset_ms": offsetMs, "reason": reason})
	if err != nil {
		return err
	}
	_, err = db.TelemetryDB.ExecContext(ctx, query, args...)
	return err
}

// --- helpers ---

func fetchSortedDurations(ctx context.Context, projectId uuid.UUID, endpoint string, from, to time.Time) ([]float64, error) {
	results, err := lit.SelectNamed[endpointDurationRow](db.TelemetryDB,
		"SELECT duration FROM endpoints WHERE project_id = :project_id AND endpoint = :endpoint AND recorded_at >= :from AND recorded_at <= :to AND is_stream = 0 ORDER BY duration ASC",
		lit.P{"project_id": projectId, "endpoint": endpoint, "from": NewSQLiteTime(from), "to": NewSQLiteTime(to)})
	if err != nil {
		return nil, err
	}
	durations := make([]float64, 0, len(results))
	for _, r := range results {
		durations = append(durations, r.Duration)
	}
	return durations, nil
}

func ComputeImpactScore(endpoint string, total, satisfiedCount, toleratingCount, badCount, clientErrorCount uint64, p99Ns float64, offsetMs uint32) float64 {
	if total == 0 {
		return 0
	}

	totalF := float64(total)
	badF := float64(badCount)
	clientF := float64(clientErrorCount)
	offsetNs := float64(offsetMs) * 1_000_000

	apdexScore := 1.0 - (float64(satisfiedCount)+float64(toleratingCount)*0.5)/totalF

	badRate := badF / totalF
	var errorRateScore float64
	switch {
	case badRate > 0.33:
		errorRateScore = 0.75
	case badRate > 0.20:
		errorRateScore = 0.50
	case badRate > 0.10:
		errorRateScore = 0.25
	}

	adjustedP99 := p99Ns - offsetNs
	var p99Score float64
	switch {
	case adjustedP99 > 8_000_000_000:
		p99Score = 0.75
	case adjustedP99 > 6_000_000_000:
		p99Score = 0.50
	case adjustedP99 > 3_000_000_000:
		p99Score = 0.25
	}

	var clientErrorScore float64
	if endpoint != "UNMATCHED" && total > 10 {
		clientRate := clientF / totalF
		switch {
		case clientRate > 0.50:
			clientErrorScore = 0.75
		case clientRate > 0.25:
			clientErrorScore = 0.50
		}
	}

	var volumeScore float64
	switch {
	case badRate > 0.10 && badCount >= 500:
		volumeScore = 0.75
	case badRate > 0.10 && badCount >= 50:
		volumeScore = 0.50
	case badRate > 0.05 && badCount >= 2000:
		volumeScore = 0.75
	case badRate > 0.05 && badCount >= 500:
		volumeScore = 0.50
	case badRate > 0.05 && badCount >= 50:
		volumeScore = 0.25
	case badRate > 0.01 && badCount >= 10000:
		volumeScore = 0.75
	case badRate > 0.01 && badCount >= 2000:
		volumeScore = 0.50
	case badRate > 0.01 && badCount >= 500:
		volumeScore = 0.25
	}

	return math.Max(apdexScore, math.Max(errorRateScore, math.Max(p99Score, math.Max(clientErrorScore, volumeScore))))
}

func sortEndpointStats(stats []models.EndpointStats, orderBy string, sortDirection string) {
	desc := sortDirection != "asc"

	sort.Slice(stats, func(i, j int) bool {
		var less bool
		switch orderBy {
		case "count":
			less = stats[i].Count < stats[j].Count
		case "p50_duration":
			less = stats[i].P50Duration < stats[j].P50Duration
		case "p95_duration":
			less = stats[i].P95Duration < stats[j].P95Duration
		case "p99_duration":
			less = stats[i].P99Duration < stats[j].P99Duration
		case "avg_duration":
			less = stats[i].AvgDuration < stats[j].AvgDuration
		case "last_seen":
			less = stats[i].LastSeen.Before(stats[j].LastSeen)
		default:
			less = stats[i].Impact < stats[j].Impact
		}
		if desc {
			return !less
		}
		return less
	})
}

func getTopEndpointsByMetric(ctx context.Context, projectId uuid.UUID, from, to time.Time, metricType string) ([]string, error) {
	params := lit.P{"project_id": projectId, "from": NewSQLiteTime(from), "to": NewSQLiteTime(to)}

	if metricType == "total_time" {
		results, err := lit.SelectNamed[endpointMetricRow](db.TelemetryDB,
			`SELECT endpoint, COUNT(*) * AVG(duration) / 1000000.0 as metric_value
			FROM endpoints WHERE project_id = :project_id AND recorded_at >= :from AND recorded_at <= :to AND is_stream = 0
			GROUP BY endpoint ORDER BY metric_value DESC LIMIT 5`,
			params)
		if err != nil {
			return nil, err
		}

		endpoints := make([]string, 0, len(results))
		for _, r := range results {
			endpoints = append(endpoints, r.Endpoint)
		}
		return endpoints, nil
	}

	percentile := 0.5
	switch metricType {
	case "p95":
		percentile = 0.95
	case "p99":
		percentile = 0.99
	}

	epRows, err := lit.SelectNamed[distinctEndpointRow](db.TelemetryDB,
		`SELECT DISTINCT endpoint FROM endpoints WHERE project_id = :project_id AND recorded_at >= :from AND recorded_at <= :to AND is_stream = 0`,
		params)
	if err != nil {
		return nil, err
	}

	type epMetric struct {
		endpoint string
		value    float64
	}

	var metrics []epMetric
	for _, ep := range epRows {
		durations, err := fetchSortedDurations(ctx, projectId, ep.Endpoint, from, to)
		if err != nil {
			return nil, err
		}
		val := computePercentile(durations, percentile) / 1000000.0
		metrics = append(metrics, epMetric{endpoint: ep.Endpoint, value: val})
	}

	sort.Slice(metrics, func(i, j int) bool {
		return metrics[i].value > metrics[j].value
	})

	limit := 5
	if limit > len(metrics) {
		limit = len(metrics)
	}

	result := make([]string, limit)
	for i := 0; i < limit; i++ {
		result[i] = metrics[i].endpoint
	}

	return result, nil
}

func (e *endpointRepository) FindByDistributedTraceId(ctx context.Context, distributedTraceId uuid.UUID, projectIds []uuid.UUID, recordedAt *time.Time) ([]models.Endpoint, error) {
	if len(projectIds) == 0 {
		return nil, nil
	}
	params := lit.P{"trace_id": distributedTraceId}
	placeholders := make([]string, len(projectIds))
	for i, pid := range projectIds {
		key := fmt.Sprintf("pid_%d", i)
		placeholders[i] = ":" + key
		params[key] = pid
	}
	query := `SELECT id, project_id, endpoint, duration, recorded_at, status_code, body_size, client_ip, attributes, app_version, server_name, distributed_trace_id
		FROM endpoints WHERE distributed_trace_id = :trace_id AND project_id IN (` + strings.Join(placeholders, ",") + `)`
	if recordedAt != nil {
		from, to := distributedTraceWindowBounds(*recordedAt)
		query += ` AND recorded_at >= :from AND recorded_at <= :to`
		params["from"] = NewSQLiteTime(from)
		params["to"] = NewSQLiteTime(to)
	}
	query += ` ORDER BY recorded_at ASC`

	parsedQuery, args, err := lit.ParseNamedQuery(db.Driver, query, params)
	if err != nil {
		return nil, err
	}

	sqlRows, err := db.TelemetryDB.QueryContext(ctx, parsedQuery, args...)
	if err != nil {
		return nil, err
	}
	defer sqlRows.Close()

	var endpoints []models.Endpoint
	for sqlRows.Next() {
		var row endpoint
		if err := sqlRows.Scan(&row.Id, &row.ProjectId, &row.Endpoint, &row.Duration, &row.RecordedAt,
			&row.StatusCode, &row.BodySize, &row.ClientIP, &row.Attributes, &row.AppVersion, &row.ServerName, &row.DistributedTraceId); err != nil {
			return nil, err
		}
		endpoints = append(endpoints, row.toModel())
	}
	return endpoints, nil
}

var EndpointRepository = endpointRepository{}
