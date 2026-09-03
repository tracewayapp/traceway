//go:build !telemetry_ch && !telemetry_duckdb

package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry/sqlitetypes"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/lit/v2"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry/shared"
)

type metricPointRepository struct{}

type avgResult struct {
	Value float64 `lit:"agg_value"`
}

type distinctServerResult struct {
	ServerName string `lit:"sn"`
}

func init() {
	models.ExtensionModelRegistrations = append(models.ExtensionModelRegistrations, func(driver lit.Driver) {
		lit.RegisterModel[avgResult](driver)
		lit.RegisterModel[distinctServerResult](driver)
	})
}

func (r *metricPointRepository) InsertAsync(ctx context.Context, points []models.MetricPoint) error {
	if len(points) == 0 {
		return nil
	}

	tx, err := db.TelemetryDB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, p := range points {
		tags := sqlitetypes.NewSQLiteJSONMap(p.Tags)
		tagsVal, _ := tags.Value()
		query, args, err := lit.ParseNamedQuery(db.Driver,
			"INSERT INTO metric_points (project_id, name, value, tags, recorded_at) VALUES (:project_id, :name, :value, :tags, :recorded_at)",
			lit.P{
				"project_id":  p.ProjectId,
				"name":        p.Name,
				"value":       p.Value,
				"tags":        tagsVal,
				"recorded_at": sqlitetypes.NewSQLiteTime(p.RecordedAt),
			})
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, query, args...); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (r *metricPointRepository) QueryTimeSeries(ctx context.Context, projectId uuid.UUID, name string, from, to time.Time, intervalMinutes int, aggregation string, tagFilters map[string]string, groupBy string, maxGroups int) (map[string][]models.TimeSeriesPoint, error) {
	var groupKeys []string
	if groupBy != "" {
		groupKeys = []string{groupBy}
	}
	return r.queryTimeSeries(ctx, projectId, name, from, to, intervalMinutes, aggregation, tagFilters, groupKeys, maxGroups)
}

// QueryTimeSeriesByTags groups by every tag in groupBy at once. Result keys are
// the tag values in that order joined by shared.GroupKeySeparator.
func (r *metricPointRepository) QueryTimeSeriesByTags(ctx context.Context, projectId uuid.UUID, name string, from, to time.Time, intervalMinutes int, aggregation string, tagFilters map[string]string, groupBy []string, maxGroups int) (map[string][]models.TimeSeriesPoint, error) {
	return r.queryTimeSeries(ctx, projectId, name, from, to, intervalMinutes, aggregation, tagFilters, groupBy, maxGroups)
}

func (r *metricPointRepository) queryTimeSeries(ctx context.Context, projectId uuid.UUID, name string, from, to time.Time, intervalMinutes int, aggregation string, tagFilters map[string]string, groupKeys []string, maxGroups int) (map[string][]models.TimeSeriesPoint, error) {
	secs := intervalMinutes * 60
	aggFunc := sqliteAggregationFunc(aggregation)
	hasGroupBy := len(groupKeys) > 0
	isLast := aggregation == "last"

	selectClause := fmt.Sprintf("SELECT datetime((strftime('%%s', recorded_at) / %d) * %d, 'unixepoch') AS bucket", secs, secs)
	if hasGroupBy {
		exprs := make([]string, len(groupKeys))
		for i := range groupKeys {
			exprs[i] = fmt.Sprintf("json_extract(tags, '$.\"' || :group_by_%d || '\"')", i)
		}
		selectClause += ", " + strings.Join(exprs, " || :group_sep || ") + " AS group_key"
	}
	selectClause += ", " + aggFunc + " AS agg_value"
	if isLast {
		selectClause += ", max(recorded_at) AS last_ts"
	}
	isRate := aggregation == "rate"
	if isRate {
		selectClause += " FROM (" + sqliteRateDeltasQuery + ") WHERE recorded_at >= :from AND delta IS NOT NULL"
	} else {
		selectClause += " FROM metric_points WHERE project_id = :project_id AND name = :name AND recorded_at >= :from AND recorded_at <= :to"
	}

	params := lit.P{
		"project_id": projectId,
		"name":       name,
		"from":       sqlitetypes.NewSQLiteTime(from),
		"to":         sqlitetypes.NewSQLiteTime(to),
	}
	if isRate {
		params["lookback"] = sqlitetypes.NewSQLiteTime(from.Add(-shared.RateLookback))
		params["secs"] = float64(secs)
	}
	for i, key := range groupKeys {
		params[fmt.Sprintf("group_by_%d", i)] = key
	}
	if len(groupKeys) > 1 {
		params["group_sep"] = shared.GroupKeySeparator
	}

	filterClauses := ""
	for i, k := range shared.SortedKeys(tagFilters) {
		fk := fmt.Sprintf("fk_%d", i)
		fv := fmt.Sprintf("fv_%d", i)
		filterClauses += fmt.Sprintf(" AND json_extract(tags, '$.\"' || :%s || '\"') = :%s", fk, fv)
		params[fk] = k
		params[fv] = tagFilters[k]
	}

	query := selectClause + filterClauses + " GROUP BY bucket"
	if hasGroupBy {
		query += ", group_key ORDER BY group_key ASC, bucket ASC"
		if maxGroups > 0 {
			query += " LIMIT :row_limit"
			params["row_limit"] = shared.TimeSeriesRowLimit(maxGroups)
		}
	} else {
		query += " ORDER BY bucket ASC"
	}

	parsedQuery, args, err := lit.ParseNamedQuery(db.Driver, query, params)
	if err != nil {
		return nil, err
	}

	rows, err := db.TelemetryDB.QueryContext(ctx, parsedQuery, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string][]models.TimeSeriesPoint)
	for rows.Next() {
		var bucketStr string
		var value float64
		var groupKeyNullable *string
		var lastTs string
		groupKey := "__all__"

		targets := []any{&bucketStr}
		if hasGroupBy {
			targets = append(targets, &groupKeyNullable)
		}
		targets = append(targets, &value)
		if isLast {
			targets = append(targets, &lastTs)
		}
		if err := rows.Scan(targets...); err != nil {
			return nil, err
		}
		if groupKeyNullable != nil {
			groupKey = *groupKeyNullable
		}

		bucket, err := time.Parse("2006-01-02 15:04:05", bucketStr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse bucket time %q: %w", bucketStr, err)
		}

		if groupKey == "" {
			groupKey = "(empty)"
		}
		if shared.GroupCapReached(result, groupKey, maxGroups) {
			break
		}
		result[groupKey] = append(result[groupKey], models.TimeSeriesPoint{
			Timestamp: bucket,
			Value:     value,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func (r *metricPointRepository) DiscoverMetrics(ctx context.Context, projectId uuid.UUID, from, to time.Time) ([]models.DiscoveredMetric, error) {
	query, args, err := lit.ParseNamedQuery(db.Driver,
		`SELECT name, j.key AS tag_key
		FROM metric_points
		LEFT JOIN json_each(tags) j
		WHERE project_id = :project_id AND recorded_at >= :from AND recorded_at <= :to
		GROUP BY name, j.key
		ORDER BY name ASC, j.key ASC`,
		lit.P{"project_id": projectId, "from": sqlitetypes.NewSQLiteTime(from), "to": sqlitetypes.NewSQLiteTime(to)})
	if err != nil {
		return nil, err
	}

	rows, err := db.TelemetryDB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	byName := make(map[string]*models.DiscoveredMetric)
	order := make([]string, 0)
	for rows.Next() {
		var name string
		var tagKey sql.NullString
		if err := rows.Scan(&name, &tagKey); err != nil {
			return nil, err
		}
		m, ok := byName[name]
		if !ok {
			m = &models.DiscoveredMetric{Name: name, TagKeys: []string{}}
			byName[name] = m
			order = append(order, name)
		}
		if tagKey.Valid && tagKey.String != "" {
			m.TagKeys = append(m.TagKeys, tagKey.String)
		}
	}

	metrics := make([]models.DiscoveredMetric, 0, len(order))
	for _, n := range order {
		metrics = append(metrics, *byName[n])
	}
	return metrics, nil
}

func (r *metricPointRepository) DiscoverTagValues(ctx context.Context, projectId uuid.UUID, metricName, tagKey string, from, to time.Time) ([]string, error) {
	type tagValueRow struct {
		TagValue string `lit:"tag_value"`
	}
	lit.RegisterModel[tagValueRow](db.Driver)

	results, err := lit.SelectNamed[tagValueRow](db.TelemetryDB,
		`SELECT DISTINCT json_extract(tags, '$."' || :tag_key || '"') AS tag_value
		FROM metric_points
		WHERE project_id = :project_id AND name = :name AND recorded_at >= :from AND recorded_at <= :to
		AND json_extract(tags, '$."' || :tag_key || '"') IS NOT NULL
		AND json_extract(tags, '$."' || :tag_key || '"') != ''
		ORDER BY tag_value ASC`,
		lit.P{"project_id": projectId, "name": metricName, "tag_key": tagKey, "from": sqlitetypes.NewSQLiteTime(from), "to": sqlitetypes.NewSQLiteTime(to)})
	if err != nil {
		return nil, err
	}

	values := make([]string, 0, len(results))
	for _, r := range results {
		values = append(values, r.TagValue)
	}
	return values, nil
}

func (r *metricPointRepository) GetAverageBetween(ctx context.Context, projectId uuid.UUID, name string, start, end time.Time) (float64, error) {
	result, err := lit.SelectSingleNamed[avgResult](db.TelemetryDB,
		"SELECT COALESCE(avg(value), 0) AS agg_value FROM metric_points WHERE project_id = :project_id AND name = :name AND recorded_at >= :from AND recorded_at <= :to",
		lit.P{"project_id": projectId, "name": name, "from": sqlitetypes.NewSQLiteTime(start), "to": sqlitetypes.NewSQLiteTime(end)})
	if err != nil {
		return 0, err
	}
	if result == nil {
		return 0, nil
	}
	return result.Value, nil
}

func (r *metricPointRepository) GetDistinctServers(ctx context.Context, projectId uuid.UUID, start, end time.Time) ([]string, error) {
	results, err := lit.SelectNamed[distinctServerResult](db.TelemetryDB,
		`SELECT DISTINCT json_extract(tags, '$.server_name') AS sn
		FROM metric_points
		WHERE project_id = :project_id AND recorded_at >= :from AND recorded_at <= :to
		AND json_extract(tags, '$.server_name') IS NOT NULL
		AND json_extract(tags, '$.server_name') != ''
		ORDER BY sn ASC`,
		lit.P{"project_id": projectId, "from": sqlitetypes.NewSQLiteTime(start), "to": sqlitetypes.NewSQLiteTime(end)})
	if err != nil {
		return nil, err
	}

	servers := make([]string, 0, len(results))
	for _, r := range results {
		servers = append(servers, r.ServerName)
	}
	return servers, nil
}

func (r *metricPointRepository) LatestPerServer(ctx context.Context, projectId uuid.UUID, name string, since time.Time) ([]models.ServerLatestPoint, error) {
	query, args, err := lit.ParseNamedQuery(db.Driver,
		`SELECT sn, latest_value, last_reported_at, latest_tags
		FROM (
			SELECT json_extract(tags, '$.server_name') AS sn,
				value AS latest_value,
				recorded_at AS last_reported_at,
				tags AS latest_tags,
				ROW_NUMBER() OVER (
					PARTITION BY json_extract(tags, '$.server_name')
					ORDER BY recorded_at DESC
				) AS row_number
			FROM metric_points
			WHERE project_id = :project_id AND name = :name AND recorded_at >= :since
			AND json_extract(tags, '$.server_name') IS NOT NULL
			AND json_extract(tags, '$.server_name') != ''
		)
		WHERE row_number = 1
		ORDER BY sn ASC`,
		lit.P{"project_id": projectId, "name": name, "since": sqlitetypes.NewSQLiteTime(since)})
	if err != nil {
		return nil, err
	}

	rows, err := db.TelemetryDB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var points []models.ServerLatestPoint
	for rows.Next() {
		var p models.ServerLatestPoint
		var lastReportedAt sqlitetypes.SQLiteTime
		var tags sqlitetypes.SQLiteJSONMap
		if err := rows.Scan(&p.ServerName, &p.Value, &lastReportedAt, &tags); err != nil {
			return nil, err
		}
		p.LastReportedAt = lastReportedAt.Time
		p.Tags = map[string]string(tags)
		points = append(points, p)
	}
	return points, rows.Err()
}

func (r *metricPointRepository) GetAverageByIntervalPerServer(ctx context.Context, projectId uuid.UUID, name string, start, end time.Time, intervalMinutes int, servers []string) (map[string][]models.TimeSeriesPoint, error) {
	secs := intervalMinutes * 60

	params := lit.P{
		"project_id": projectId,
		"name":       name,
		"from":       sqlitetypes.NewSQLiteTime(start),
		"to":         sqlitetypes.NewSQLiteTime(end),
	}

	query := fmt.Sprintf(`SELECT
		datetime((strftime('%%s', recorded_at) / %d) * %d, 'unixepoch') AS bucket,
		json_extract(tags, '$.server_name') AS sn,
		avg(value) AS avg_value
	FROM metric_points
	WHERE project_id = :project_id AND name = :name AND recorded_at >= :from AND recorded_at <= :to`, secs, secs)

	if len(servers) > 0 {
		placeholders := make([]string, len(servers))
		for i, s := range servers {
			key := fmt.Sprintf("srv_%d", i)
			placeholders[i] = ":" + key
			params[key] = s
		}
		query += " AND json_extract(tags, '$.server_name') IN (" + strings.Join(placeholders, ", ") + ")"
	} else {
		query += " AND json_extract(tags, '$.server_name') IS NOT NULL AND json_extract(tags, '$.server_name') != ''"
	}

	query += " GROUP BY bucket, sn ORDER BY bucket ASC, sn ASC"

	parsedQuery, args, err := lit.ParseNamedQuery(db.Driver, query, params)
	if err != nil {
		return nil, err
	}

	rows, err := db.TelemetryDB.QueryContext(ctx, parsedQuery, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string][]models.TimeSeriesPoint)
	for rows.Next() {
		var bucketStr string
		var serverName string
		var value float64
		if err := rows.Scan(&bucketStr, &serverName, &value); err != nil {
			return nil, err
		}

		bucket, err := time.Parse("2006-01-02 15:04:05", bucketStr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse bucket time %q: %w", bucketStr, err)
		}

		result[serverName] = append(result[serverName], models.TimeSeriesPoint{
			Timestamp: bucket,
			Value:     value,
		})
	}
	return result, nil
}

const sqliteRateDeltasQuery = "SELECT recorded_at, tags, value - lag(value) OVER (PARTITION BY tags ORDER BY recorded_at) AS delta FROM metric_points WHERE project_id = :project_id AND name = :name AND recorded_at >= :lookback AND recorded_at <= :to"

func sqliteAggregationFunc(agg string) string {
	switch agg {
	case "rate":
		return "sum(CASE WHEN delta > 0 THEN delta ELSE 0 END) / :secs"
	case "min":
		return "min(value)"
	case "max":
		return "max(value)"
	case "sum":
		return "sum(value)"
	case "count":
		return "CAST(COUNT(*) AS REAL)"
	case "last":
		return "value"
	default:
		return "avg(value)"
	}
}

var MetricPointRepository = &metricPointRepository{}
