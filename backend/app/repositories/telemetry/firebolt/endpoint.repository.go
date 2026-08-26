//go:build telemetry_firebolt

package firebolt

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/lit/v2"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry/shared"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry/sqlitetypes"
)

type endpoint struct {
	Id                 uuid.UUID                 `lit:"id"`
	ProjectId          uuid.UUID                 `lit:"project_id"`
	Endpoint           string                    `lit:"endpoint"`
	Duration           int64                     `lit:"duration"`
	RecordedAt         sqlitetypes.SQLiteTime    `lit:"recorded_at"`
	StatusCode         int16                     `lit:"status_code"`
	BodySize           int32                     `lit:"body_size"`
	ClientIP           string                    `lit:"client_ip"`
	Attributes         sqlitetypes.SQLiteJSONMap `lit:"attributes"`
	AppVersion         string                    `lit:"app_version"`
	ServerName         string                    `lit:"server_name"`
	DistributedTraceId *uuid.UUID                `lit:"distributed_trace_id"`
	SpanId             *uuid.UUID                `lit:"span_id"`
	IsStream           bool                      `lit:"is_stream"`
	IsRoot             bool                      `lit:"is_root"`
}

type groupedEndpointRow struct {
	Endpoint         string
	TotalCount       uint64
	AvgDuration      float64
	LastSeen         time.Time
	OffsetMs         uint32
	ServerErrorCount uint64
	ClientErrorCount uint64
	SatisfiedCount   uint64
	ToleratingCount  uint64
	BadCount         uint64
	IsStream         bool
	HasRoot          bool
	HasNonRoot       bool
	P50              sql.NullFloat64
	P95              sql.NullFloat64
	P99              sql.NullFloat64
}

type slowEndpointRow struct {
	OffsetMs uint32 `lit:"offset_ms"`
	Reason   string `lit:"reason"`
}

type slowEndpointNameRow struct {
	Endpoint string `lit:"endpoint"`
	OffsetMs uint32 `lit:"offset_ms"`
}

type endpointMetricRow struct {
	Endpoint    string  `lit:"endpoint"`
	MetricValue float64 `lit:"metric_value"`
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

// endpointTimeSeriesRow scans time-bucketed aggregates whose bucket is a
// timestamp column (Firebolt has no strftime; buckets stay timestamps).
type endpointTimeSeriesRow struct {
	Bucket sqlitetypes.SQLiteTime `lit:"bucket"`
	Value  float64                `lit:"agg_value"`
}

func init() {
	registerModels(func(driver lit.Driver) {
		lit.RegisterModel[endpoint](driver)
		lit.RegisterModel[slowEndpointRow](driver)
		lit.RegisterModel[slowEndpointNameRow](driver)
		lit.RegisterModel[endpointMetricRow](driver)
		lit.RegisterModel[endpointDetailStatsRow](driver)
		lit.RegisterModel[isStreamFlagRow](driver)
		lit.RegisterModel[endpointTimeSeriesRow](driver)
	})
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

func endpointTimeSeriesRowsToPoints(rows []*endpointTimeSeriesRow) []models.TimeSeriesPoint {
	points := make([]models.TimeSeriesPoint, 0, len(rows))
	for _, r := range rows {
		points = append(points, models.TimeSeriesPoint{Timestamp: r.Bucket.Time.UTC(), Value: r.Value})
	}
	return points
}

type endpointRepository struct{}

var endpointColumns = []string{"id", "project_id", "endpoint", "duration", "recorded_at", "status_code", "body_size", "client_ip", "attributes", "app_version", "server_name", "distributed_trace_id", "span_id", "is_stream", "is_root"}

func (e *endpointRepository) InsertAsync(ctx context.Context, lines []models.Endpoint) error {
	if len(lines) == 0 {
		return nil
	}

	rows := make([][]any, 0, len(lines))
	for _, ep := range lines {
		attributesJSON, err := attrJSON(ep.Attributes)
		if err != nil {
			return err
		}

		var distributedTraceId *string
		if ep.DistributedTraceId != nil {
			v := ep.DistributedTraceId.String()
			distributedTraceId = &v
		}
		var spanId *string
		if ep.SpanId != nil {
			v := ep.SpanId.String()
			spanId = &v
		}

		isStream := boolToInt(ep.IsStream)
		isRoot := boolToInt(ep.IsRoot)

		rows = append(rows, []any{
			ep.Id.String(),
			ep.ProjectId.String(),
			ep.Endpoint,
			int64(ep.Duration),
			ep.RecordedAt.UTC(),
			int64(ep.StatusCode),
			int64(ep.BodySize),
			ep.ClientIP,
			attributesJSON,
			ep.AppVersion,
			ep.ServerName,
			nullableString(distributedTraceId),
			nullableString(spanId),
			isStream,
			isRoot,
		})
	}
	return insertRows(ctx, "endpoints", endpointColumns, rows)
}

func (e *endpointRepository) CountBetween(ctx context.Context, projectId uuid.UUID, start, end time.Time) (int64, error) {
	result, err := lit.SelectSingleNamed[fbCountResult](db.TelemetryDB,
		"SELECT COUNT(*) AS count FROM endpoints WHERE project_id = :project_id AND DATE_TRUNC('minute', recorded_at) >= :from_min AND DATE_TRUNC('minute', recorded_at) <= :to_min",
		minuteRangeParams(projectId, start, end))
	if err != nil {
		return 0, err
	}
	if result == nil {
		return 0, nil
	}
	return int64(result.Count), nil
}

func (e *endpointRepository) FindAll(ctx context.Context, projectId uuid.UUID, fromDate, toDate time.Time, page, pageSize int, orderBy string) ([]models.Endpoint, int64, error) {
	params := lit.P{"project_id": projectId, "from": fromDate.UTC(), "to": toDate.UTC()}

	countResult, err := lit.SelectSingleNamed[fbCountResult](db.TelemetryDB,
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
		fmt.Sprintf(`SELECT e.id, e.project_id, e.endpoint, e.duration, e.recorded_at, e.status_code, e.body_size, e.client_ip, e.attributes, e.app_version, e.server_name, e.distributed_trace_id
		FROM (
			SELECT id AS page_id FROM endpoints
			WHERE project_id = :project_id AND recorded_at >= :from AND recorded_at <= :to
			ORDER BY %s DESC LIMIT :limit OFFSET :offset
		) p JOIN endpoints e ON e.id = p.page_id
		WHERE e.project_id = :project_id
		ORDER BY e.%s DESC`, orderBy, orderBy),
		lit.P{"project_id": projectId, "from": fromDate.UTC(), "to": toDate.UTC(), "limit": pageSize, "offset": offset})
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
	params := minuteRangeParams(projectId, fromDate, toDate)

	// Filters reference only aggregating-index keys so the grouping merges
	// index states instead of scanning rows; offset-configured endpoints
	// are corrected afterwards by refineOffsetEndpoints.
	whereClause := "e.project_id = :project_id AND " + indexMinuteRange("e.recorded_at")
	if search != "" {
		whereClause += " AND STRPOS(LOWER(e.endpoint), LOWER(:search)) > 0"
		params["search"] = search
	}
	whereClause += shared.RootFilterClause("e.is_root", rootFilter)

	groups, err := queryGroupedEndpointsIndexed(ctx, whereClause, params, true)
	if err != nil {
		return nil, 0, err
	}

	if err := refineOffsetEndpoints(ctx, projectId, fromDate, toDate, whereClause, params, groups); err != nil {
		return nil, 0, err
	}

	var stats []models.EndpointStats
	for _, g := range groups {
		lastSeen := g.LastSeen

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
				Impact:       shared.ComputeStreamImpact(g.Endpoint, g.TotalCount, g.ServerErrorCount, g.ClientErrorCount),
				ImpactReason: shared.ComputeStreamImpactReason(g.Endpoint, g.TotalCount, g.ServerErrorCount, g.ClientErrorCount),
				IsStream:     true,
				HasRoot:      g.HasRoot,
				HasNonRoot:   g.HasNonRoot,
			})
			continue
		}

		p50 := g.P50.Float64
		p95 := g.P95.Float64
		p99 := g.P99.Float64

		impact := shared.ComputeImpactScore(g.Endpoint, g.TotalCount, g.SatisfiedCount, g.ToleratingCount, g.BadCount, g.ClientErrorCount, p99, g.OffsetMs)

		stats = append(stats, models.EndpointStats{
			Endpoint:    g.Endpoint,
			Count:       g.TotalCount,
			P50Duration: time.Duration(p50),
			P95Duration: time.Duration(p95),
			P99Duration: time.Duration(p99),
			AvgDuration: time.Duration(g.AvgDuration),
			LastSeen:    lastSeen,
			Impact:      impact,
			ImpactReason: shared.ComputeImpactReason(g.Endpoint, g.TotalCount, g.SatisfiedCount, g.ToleratingCount,
				g.BadCount, g.ClientErrorCount, p99, g.OffsetMs),
			HasRoot:    g.HasRoot,
			HasNonRoot: g.HasNonRoot,
		})
	}

	sortEndpointStats(stats, orderBy, sortDirection)

	// Pagination is in-Go, so the group query already returned every group —
	// no separate COUNT(DISTINCT) scan needed.
	totalEndpoints := int64(len(stats))

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
	params := lit.P{"project_id": projectId, "endpoint": endpointName, "from": fromDate.UTC(), "to": toDate.UTC()}

	countResult, err := lit.SelectSingleNamed[fbCountResult](db.TelemetryDB,
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
		fmt.Sprintf(`SELECT e.id, e.project_id, e.endpoint, e.duration, e.recorded_at, e.status_code, e.body_size, e.client_ip, e.attributes, e.app_version, e.server_name, e.distributed_trace_id
		FROM (
			SELECT id AS page_id FROM endpoints
			WHERE project_id = :project_id AND endpoint = :endpoint AND recorded_at >= :from AND recorded_at <= :to
			ORDER BY %s %s LIMIT :limit OFFSET :offset
		) p JOIN endpoints e ON e.id = p.page_id
		WHERE e.project_id = :project_id
		ORDER BY e.%s %s`, orderBy, sortDir, orderBy, sortDir),
		lit.P{"project_id": projectId, "endpoint": endpointName, "from": fromDate.UTC(), "to": toDate.UTC(), "limit": pageSize, "offset": offset})
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
		from, to := shared.TraceWindowBounds(*recordedAt)
		query += ` AND recorded_at >= :from AND recorded_at <= :to`
		params["from"] = from.UTC()
		params["to"] = to.UTC()
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
	results, err := lit.SelectNamed[endpointTimeSeriesRow](db.TelemetryDB,
		fmt.Sprintf(`SELECT %s as bucket, CAST(COUNT(*) AS DOUBLE PRECISION) as agg_value
		FROM endpoints WHERE project_id = :project_id AND DATE_TRUNC('minute', recorded_at) >= :from_min AND DATE_TRUNC('minute', recorded_at) <= :to_min
		GROUP BY bucket ORDER BY bucket ASC`, indexBucketExpr("recorded_at", 3600)),
		minuteRangeParams(projectId, start, end))
	if err != nil {
		return nil, err
	}
	return endpointTimeSeriesRowsToPoints(results), nil
}

func (e *endpointRepository) AvgDurationByHour(ctx context.Context, projectId uuid.UUID, start, end time.Time) ([]models.TimeSeriesPoint, error) {
	results, err := lit.SelectNamed[endpointTimeSeriesRow](db.TelemetryDB,
		fmt.Sprintf(`SELECT %s as bucket, AVG(duration) / 1000000.0 as agg_value
		FROM endpoints WHERE project_id = :project_id AND DATE_TRUNC('minute', recorded_at) >= :from_min AND DATE_TRUNC('minute', recorded_at) <= :to_min AND is_stream = 0
		GROUP BY bucket ORDER BY bucket ASC`, indexBucketExpr("recorded_at", 3600)),
		minuteRangeParams(projectId, start, end))
	if err != nil {
		return nil, err
	}
	return endpointTimeSeriesRowsToPoints(results), nil
}

func (e *endpointRepository) ErrorRateByHour(ctx context.Context, projectId uuid.UUID, start, end time.Time) ([]models.TimeSeriesPoint, error) {
	results, err := lit.SelectNamed[endpointTimeSeriesRow](db.TelemetryDB,
		fmt.Sprintf(`SELECT %s as bucket,
		SUM(CASE WHEN status_code >= 500 THEN 1 ELSE 0 END) * 100.0 / COUNT(*) as agg_value
		FROM endpoints WHERE project_id = :project_id AND DATE_TRUNC('minute', recorded_at) >= :from_min AND DATE_TRUNC('minute', recorded_at) <= :to_min
		GROUP BY bucket ORDER BY bucket ASC`, indexBucketExpr("recorded_at", 3600)),
		minuteRangeParams(projectId, start, end))
	if err != nil {
		return nil, err
	}
	return endpointTimeSeriesRowsToPoints(results), nil
}

func (e *endpointRepository) CountByInterval(ctx context.Context, projectId uuid.UUID, start, end time.Time, intervalMinutes int) ([]models.TimeSeriesPoint, error) {
	results, err := lit.SelectNamed[endpointTimeSeriesRow](db.TelemetryDB,
		fmt.Sprintf(`SELECT %s as bucket, CAST(COUNT(*) AS DOUBLE PRECISION) as agg_value
		FROM endpoints WHERE project_id = :project_id AND DATE_TRUNC('minute', recorded_at) >= :from_min AND DATE_TRUNC('minute', recorded_at) <= :to_min
		GROUP BY bucket ORDER BY bucket ASC`, indexBucketExpr("recorded_at", intervalMinutes*60)),
		minuteRangeParams(projectId, start, end))
	if err != nil {
		return nil, err
	}
	return endpointTimeSeriesRowsToPoints(results), nil
}

func (e *endpointRepository) AvgDurationByInterval(ctx context.Context, projectId uuid.UUID, start, end time.Time, intervalMinutes int) ([]models.TimeSeriesPoint, error) {
	results, err := lit.SelectNamed[endpointTimeSeriesRow](db.TelemetryDB,
		fmt.Sprintf(`SELECT %s as bucket, AVG(duration) / 1000000.0 as agg_value
		FROM endpoints WHERE project_id = :project_id AND DATE_TRUNC('minute', recorded_at) >= :from_min AND DATE_TRUNC('minute', recorded_at) <= :to_min AND is_stream = 0
		GROUP BY bucket ORDER BY bucket ASC`, indexBucketExpr("recorded_at", intervalMinutes*60)),
		minuteRangeParams(projectId, start, end))
	if err != nil {
		return nil, err
	}
	return endpointTimeSeriesRowsToPoints(results), nil
}

func (e *endpointRepository) ErrorRateByInterval(ctx context.Context, projectId uuid.UUID, start, end time.Time, intervalMinutes int) ([]models.TimeSeriesPoint, error) {
	results, err := lit.SelectNamed[endpointTimeSeriesRow](db.TelemetryDB,
		fmt.Sprintf(`SELECT %s as bucket,
		SUM(CASE WHEN status_code >= 500 THEN 1 ELSE 0 END) * 100.0 / COUNT(*) as agg_value
		FROM endpoints WHERE project_id = :project_id AND DATE_TRUNC('minute', recorded_at) >= :from_min AND DATE_TRUNC('minute', recorded_at) <= :to_min
		GROUP BY bucket ORDER BY bucket ASC`, indexBucketExpr("recorded_at", intervalMinutes*60)),
		minuteRangeParams(projectId, start, end))
	if err != nil {
		return nil, err
	}
	return endpointTimeSeriesRowsToPoints(results), nil
}

func (e *endpointRepository) FindWorstEndpoints(ctx context.Context, projectId uuid.UUID, start, end time.Time, limit int) ([]models.EndpointStats, error) {
	params := minuteRangeParams(projectId, start, end)
	whereClause := "e.project_id = :project_id AND " + indexMinuteRange("e.recorded_at") + " AND e.is_stream = 0"

	groups, err := queryGroupedEndpointsIndexed(ctx, whereClause, params, true)
	if err != nil {
		return nil, err
	}

	if err := refineOffsetEndpoints(ctx, projectId, start, end, whereClause, params, groups); err != nil {
		return nil, err
	}

	var stats []models.EndpointStats
	for _, g := range groups {
		p50 := g.P50.Float64
		p95 := g.P95.Float64
		p99 := g.P99.Float64

		impact := shared.ComputeImpactScore(g.Endpoint, g.TotalCount, g.SatisfiedCount, g.ToleratingCount, g.BadCount, g.ClientErrorCount, p99, g.OffsetMs)

		lastSeen := g.LastSeen

		stats = append(stats, models.EndpointStats{
			Endpoint:    g.Endpoint,
			Count:       g.TotalCount,
			P50Duration: time.Duration(p50),
			P95Duration: time.Duration(p95),
			P99Duration: time.Duration(p99),
			AvgDuration: time.Duration(g.AvgDuration),
			LastSeen:    lastSeen,
			Impact:      impact,
			ImpactReason: shared.ComputeImpactReason(g.Endpoint, g.TotalCount, g.SatisfiedCount, g.ToleratingCount,
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
	params := minuteRangeParams(projectId, start, end)
	params["endpoint"] = endpoint

	durationMinutes := end.Sub(start).Minutes()
	if durationMinutes < 1 {
		durationMinutes = 1
	}

	row, err := lit.SelectSingleNamed[endpointDetailStatsRow](db.TelemetryDB,
		`SELECT
			COUNT(*) as count,
			CASE WHEN COUNT(*) > 0 THEN AVG(duration) / 1000000.0 ELSE 0 END as avg_duration_ms,
			CASE WHEN COUNT(*) > 0 THEN SUM(CASE WHEN status_code >= 500 THEN 1 ELSE 0 END) * 100.0 / COUNT(*) ELSE 0 END as error_rate,
			COALESCE(SUM(CASE WHEN duration <= 500000000 AND status_code < 500 THEN 1 ELSE 0 END) +
				SUM(CASE WHEN duration > 500000000 AND duration <= 2000000000 AND status_code < 500 THEN 1 ELSE 0 END) * 0.5, 0) as satisfied_tolerating
		FROM endpoints WHERE project_id = :project_id AND endpoint = :endpoint AND DATE_TRUNC('minute', recorded_at) >= :from_min AND DATE_TRUNC('minute', recorded_at) <= :to_min`,
		params)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return &models.EndpointDetailStats{}, nil
	}

	isStreamRow, err := lit.SelectSingleNamed[isStreamFlagRow](db.TelemetryDB,
		`SELECT COALESCE(MAX(is_stream), 0) as is_stream FROM endpoints WHERE project_id = :project_id AND endpoint = :endpoint AND DATE_TRUNC('minute', recorded_at) >= :from_min AND DATE_TRUNC('minute', recorded_at) <= :to_min`,
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

		pctQuery, pctArgs, err := parseNamed(
			`SELECT PERCENTILE_CONT(0.5) WITHIN GROUP (ORDER BY duration) AS p50, PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY duration) AS p95, PERCENTILE_CONT(0.99) WITHIN GROUP (ORDER BY duration) AS p99
			FROM endpoints WHERE project_id = :project_id AND endpoint = :endpoint AND DATE_TRUNC('minute', recorded_at) >= :from_min AND DATE_TRUNC('minute', recorded_at) <= :to_min AND is_stream = 0`,
			params)
		if err != nil {
			return nil, err
		}

		var p50, p95, p99 sql.NullFloat64
		if err := db.TelemetryDB.QueryRowContext(ctx, pctQuery, pctArgs...).Scan(&p50, &p95, &p99); err != nil {
			return nil, err
		}

		stats.MedianDuration = p50.Float64 / 1000000.0
		stats.P95Duration = p95.Float64 / 1000000.0
		stats.P99Duration = p99.Float64 / 1000000.0
	}

	return &stats, nil
}

func (e *endpointRepository) GetEndpointStackedChart(ctx context.Context, projectId uuid.UUID, start, end time.Time, intervalMinutes int, metricType string) (*models.EndpointStackedChartResponse, error) {
	params := minuteRangeParams(projectId, start, end)

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
		%s as bucket,
		%s as endpoint_category,
		%s as metric_value
	FROM endpoints
	WHERE project_id = :project_id AND DATE_TRUNC('minute', recorded_at) >= :from_min AND DATE_TRUNC('minute', recorded_at) <= :to_min AND is_stream = 0
	GROUP BY bucket, endpoint_category
	ORDER BY bucket ASC, endpoint_category ASC`, indexBucketExpr("recorded_at", secs), caseExpr, metricExpr)

	parsedQuery, args, err := parseNamed(timeSeriesQuery, params)
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
		var bucket time.Time
		if err := sqlRows.Scan(&bucket, &p.Endpoint, &p.Value); err != nil {
			return nil, err
		}
		p.Timestamp = bucket
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

	params := minuteRangeParams(projectId, start, end)

	caseExpr := "CASE "
	for i, ep := range topEndpoints {
		key := fmt.Sprintf("ep_%d", i)
		caseExpr += fmt.Sprintf("WHEN endpoint = :%s THEN :%s ", key, key)
		params[key] = ep
	}
	caseExpr += "ELSE 'Other' END"

	query := fmt.Sprintf(`SELECT
		%s as bucket,
		%s as endpoint_category,
		PERCENTILE_CONT(%g) WITHIN GROUP (ORDER BY duration) / 1000000.0 as metric_value
	FROM endpoints
	WHERE project_id = :project_id AND DATE_TRUNC('minute', recorded_at) >= :from_min AND DATE_TRUNC('minute', recorded_at) <= :to_min AND is_stream = 0
	GROUP BY bucket, endpoint_category
	ORDER BY bucket ASC, endpoint_category ASC`, indexBucketExpr("recorded_at", bucketSecs), caseExpr, percentile)

	parsedQuery, args, err := parseNamed(query, params)
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
		var bucket time.Time
		var category string
		var value float64
		if err := sqlRows.Scan(&bucket, &category, &value); err != nil {
			return nil, err
		}
		series = append(series, models.EndpointTimeSeriesPoint{
			Timestamp: bucket.UTC(),
			Endpoint:  category,
			Value:     value,
		})
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
	query, args, err := parseNamed(
		"INSERT INTO slow_endpoints (project_id, endpoint, offset_ms, reason) VALUES (:project_id, :endpoint, :offset_ms, :reason) ON CONFLICT (project_id, endpoint) DO UPDATE SET offset_ms = excluded.offset_ms, reason = excluded.reason",
		lit.P{"project_id": projectId, "endpoint": endpoint, "offset_ms": offsetMs, "reason": reason})
	if err != nil {
		return err
	}
	if _, err := db.TelemetryDB.ExecContext(ctx, query, args...); err != nil {
		return err
	}
	return nil
}

// --- helpers ---

// queryGroupedEndpointsIndexed splits the grouped read into two joined
// halves because FILTER on the percentiles would block the aggregating-index
// rewrite — the percentile half filters is_stream via the index key instead.
// offset_ms scans as 0; refineOffsetEndpoints corrects offset endpoints.
func queryGroupedEndpointsIndexed(ctx context.Context, whereClause string, params lit.P, _ bool) ([]*groupedEndpointRow, error) {
	groupQuery := `SELECT c.endpoint, c.total_count, c.avg_duration, c.last_seen, 0 as offset_ms,
		c.server_error_count, c.client_error_count, c.satisfied_count, c.tolerating_count, c.bad_count,
		c.is_stream, c.has_root, c.has_non_root, p.p50, p.p95, p.p99
	FROM (
		SELECT e.endpoint,
			COUNT(*) as total_count,
			AVG(e.duration) as avg_duration,
			MAX(e.recorded_at) as last_seen,
			CAST(SUM(CASE WHEN e.status_code >= 500 THEN 1 ELSE 0 END) AS BIGINT) as server_error_count,
			CAST(SUM(CASE WHEN e.status_code >= 400 AND e.status_code < 500 THEN 1 ELSE 0 END) AS BIGINT) as client_error_count,
			CAST(SUM(CASE WHEN e.duration <= 750000000 AND e.status_code < 500 THEN 1 ELSE 0 END) AS BIGINT) as satisfied_count,
			CAST(SUM(CASE WHEN e.duration > 750000000 AND e.duration <= 1500000000 AND e.status_code < 500 THEN 1 ELSE 0 END) AS BIGINT) as tolerating_count,
			CAST(SUM(CASE WHEN e.duration > 1500000000 OR e.status_code >= 500 THEN 1 ELSE 0 END) AS BIGINT) as bad_count,
			MAX(e.is_stream) as is_stream,
			MAX(e.is_root) as has_root,
			MAX(CASE WHEN e.is_root = 0 THEN 1 ELSE 0 END) as has_non_root
		FROM endpoints e
		WHERE ` + whereClause + `
		GROUP BY e.endpoint
	) c LEFT JOIN (
		SELECT e.endpoint,
			PERCENTILE_CONT(0.50) WITHIN GROUP (ORDER BY e.duration) AS p50,
			PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY e.duration) AS p95,
			PERCENTILE_CONT(0.99) WITHIN GROUP (ORDER BY e.duration) AS p99
		FROM endpoints e
		WHERE ` + whereClause + ` AND e.is_stream = 0
		GROUP BY e.endpoint
	) p ON c.endpoint = p.endpoint`

	parsedQuery, args, err := parseNamed(groupQuery, params)
	if err != nil {
		return nil, err
	}

	sqlRows, err := db.TelemetryDB.QueryContext(ctx, parsedQuery, args...)
	if err != nil {
		return nil, err
	}
	defer sqlRows.Close()

	var groups []*groupedEndpointRow
	for sqlRows.Next() {
		var g groupedEndpointRow
		if err := sqlRows.Scan(&g.Endpoint, &g.TotalCount, &g.AvgDuration, &g.LastSeen,
			&g.OffsetMs, &g.ServerErrorCount, &g.ClientErrorCount,
			&g.SatisfiedCount, &g.ToleratingCount, &g.BadCount, &g.IsStream, &g.HasRoot, &g.HasNonRoot,
			&g.P50, &g.P95, &g.P99); err != nil {
			return nil, err
		}
		groups = append(groups, &g)
	}
	return groups, sqlRows.Err()
}

// refineOffsetEndpoints re-queries endpoints that have a slow-endpoint
// offset with an offset-aware raw scan bounded to just those endpoints —
// offsets are dynamic data the aggregating index cannot encode. Reusing
// whereClause/params keeps both passes on the identical time window.
func refineOffsetEndpoints(ctx context.Context, projectId uuid.UUID, from, to time.Time, whereClause string, params lit.P, groups []*groupedEndpointRow) error {
	byEndpoint := make(map[string]*groupedEndpointRow, len(groups))
	for _, g := range groups {
		byEndpoint[g.Endpoint] = g
	}

	offsetRows, err := lit.SelectNamed[slowEndpointNameRow](db.TelemetryDB,
		"SELECT endpoint, offset_ms FROM slow_endpoints WHERE project_id = :project_id AND offset_ms > 0",
		lit.P{"project_id": projectId})
	if err != nil {
		return err
	}

	refineParams := lit.P{}
	for k, v := range params {
		refineParams[k] = v
	}
	var placeholders []string
	for i, r := range offsetRows {
		if _, shown := byEndpoint[r.Endpoint]; !shown {
			continue
		}
		key := fmt.Sprintf("se_%d", i)
		placeholders = append(placeholders, ":"+key)
		refineParams[key] = r.Endpoint
	}
	if len(placeholders) == 0 {
		return nil
	}

	refineQuery := `SELECT
		e.endpoint,
		COUNT(*) as total_count,
		AVG(e.duration) as avg_duration,
		MAX(e.recorded_at) as last_seen,
		COALESCE(s.offset_ms, 0) as offset_ms,
		CAST(SUM(CASE WHEN e.status_code >= 500 THEN 1 ELSE 0 END) AS BIGINT) as server_error_count,
		CAST(SUM(CASE WHEN e.status_code >= 400 AND e.status_code < 500 THEN 1 ELSE 0 END) AS BIGINT) as client_error_count,
		CAST(SUM(CASE WHEN e.duration <= (750000000 + COALESCE(s.offset_ms, 0) * 1000000) AND e.status_code < 500 THEN 1 ELSE 0 END) AS BIGINT) as satisfied_count,
		CAST(SUM(CASE WHEN e.duration > (750000000 + COALESCE(s.offset_ms, 0) * 1000000) AND e.duration <= (1500000000 + COALESCE(s.offset_ms, 0) * 1000000) AND e.status_code < 500 THEN 1 ELSE 0 END) AS BIGINT) as tolerating_count,
		CAST(SUM(CASE WHEN e.duration > (1500000000 + COALESCE(s.offset_ms, 0) * 1000000) OR e.status_code >= 500 THEN 1 ELSE 0 END) AS BIGINT) as bad_count,
		MAX(e.is_stream) as is_stream,
		MAX(e.is_root) as has_root,
		MAX(CASE WHEN e.is_root = 0 THEN 1 ELSE 0 END) as has_non_root,
		PERCENTILE_CONT(0.50) WITHIN GROUP (ORDER BY e.duration) FILTER (WHERE e.is_stream = 0) AS p50,
		PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY e.duration) FILTER (WHERE e.is_stream = 0) AS p95,
		PERCENTILE_CONT(0.99) WITHIN GROUP (ORDER BY e.duration) FILTER (WHERE e.is_stream = 0) AS p99
	FROM endpoints e
	LEFT JOIN slow_endpoints s ON e.endpoint = s.endpoint AND e.project_id = s.project_id
	WHERE ` + whereClause + ` AND e.endpoint IN (` + strings.Join(placeholders, ",") + `)
	GROUP BY e.endpoint, s.offset_ms`

	parsedQuery, args, err := parseNamed(refineQuery, refineParams)
	if err != nil {
		return err
	}

	sqlRows, err := db.TelemetryDB.QueryContext(ctx, parsedQuery, args...)
	if err != nil {
		return err
	}
	defer sqlRows.Close()

	for sqlRows.Next() {
		var g groupedEndpointRow
		if err := sqlRows.Scan(&g.Endpoint, &g.TotalCount, &g.AvgDuration, &g.LastSeen,
			&g.OffsetMs, &g.ServerErrorCount, &g.ClientErrorCount,
			&g.SatisfiedCount, &g.ToleratingCount, &g.BadCount, &g.IsStream, &g.HasRoot, &g.HasNonRoot,
			&g.P50, &g.P95, &g.P99); err != nil {
			return err
		}
		if existing, ok := byEndpoint[g.Endpoint]; ok {
			*existing = g
		}
	}
	return sqlRows.Err()
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
	params := minuteRangeParams(projectId, from, to)

	if metricType == "total_time" {
		results, err := lit.SelectNamed[endpointMetricRow](db.TelemetryDB,
			`SELECT endpoint, COUNT(*) * AVG(duration) / 1000000.0 as metric_value
			FROM endpoints WHERE project_id = :project_id AND DATE_TRUNC('minute', recorded_at) >= :from_min AND DATE_TRUNC('minute', recorded_at) <= :to_min AND is_stream = 0
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

	results, err := lit.SelectNamed[endpointMetricRow](db.TelemetryDB,
		fmt.Sprintf(`SELECT endpoint, PERCENTILE_CONT(%g) WITHIN GROUP (ORDER BY duration) / 1000000.0 as metric_value
		FROM endpoints WHERE project_id = :project_id AND DATE_TRUNC('minute', recorded_at) >= :from_min AND DATE_TRUNC('minute', recorded_at) <= :to_min AND is_stream = 0
		GROUP BY endpoint ORDER BY metric_value DESC LIMIT 5`, percentile),
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
		from, to := shared.DistributedTraceWindowBounds(*recordedAt)
		query += ` AND recorded_at >= :from AND recorded_at <= :to`
		params["from"] = from.UTC()
		params["to"] = to.UTC()
	}
	query += ` ORDER BY recorded_at ASC`

	parsedQuery, args, err := parseNamed(query, params)
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

var EndpointRepository = &endpointRepository{}
