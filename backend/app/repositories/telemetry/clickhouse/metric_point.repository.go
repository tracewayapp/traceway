//go:build telemetry_ch

package clickhouse

import (
	"context"
	"github.com/tracewayapp/traceway/backend/app/chdb"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry/shared"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/google/uuid"
)

type metricPointRepository struct{}

func (r *metricPointRepository) InsertAsync(ctx context.Context, points []models.MetricPoint) error {
	if len(points) == 0 {
		return nil
	}
	return chdb.SendBatch("INSERT INTO metric_points (project_id, name, value, tags, recorded_at)", func(batch driver.Batch) error {
		for _, p := range points {
			if err := batch.Append(p.ProjectId, p.Name, p.Value, p.Tags, p.RecordedAt); err != nil {
				return err
			}
		}
		return nil
	})
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
	table := selectTable(to.Sub(from))
	isRate := aggregation == "rate"
	if aggregation == "last" || isRate {
		table = "metric_points"
	}

	aggFunc := aggregationFunc(aggregation, table)

	query := "SELECT toStartOfInterval(recorded_at, INTERVAL ? MINUTE) AS bucket"

	args := []interface{}{intervalMinutes}

	hasGroupBy := len(groupKeys) > 0
	if hasGroupBy {
		for i, key := range groupKeys {
			if i > 0 {
				query += " || ? || "
				args = append(args, shared.GroupKeySeparator)
			} else {
				query += ", "
			}
			query += "tags[?]"
			args = append(args, key)
		}
		query += " AS group_key"
	}

	if isRate {
		query += ", sum(greatest(delta, 0)) / ? AS agg_value FROM (" + chRateDeltasSelect + " WHERE project_id = ? AND name = ? AND recorded_at >= ? AND recorded_at <= ?"
		args = append(args, float64(intervalMinutes*60), projectId, name, from.Add(-shared.RateLookback), to)
	} else {
		query += ", " + aggFunc + " AS agg_value FROM " + table + " WHERE project_id = ? AND name = ? AND recorded_at >= ? AND recorded_at <= ?"
		args = append(args, projectId, name, from, to)
	}

	for k, v := range tagFilters {
		query += " AND tags[?] = ?"
		args = append(args, k, v)
	}

	if isRate {
		query += " " + chRateWindow + ") WHERE rn > 1 AND recorded_at >= ?"
		args = append(args, from)
	}

	query += " GROUP BY bucket"
	if hasGroupBy {
		query += ", group_key ORDER BY group_key ASC, bucket ASC"
		if maxGroups > 0 {
			query += " LIMIT ?"
			args = append(args, shared.TimeSeriesRowLimit(maxGroups))
		}
	} else {
		query += " ORDER BY bucket ASC"
	}

	rows, err := chdb.Conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string][]models.TimeSeriesPoint)
	for rows.Next() {
		var bucket time.Time
		var value float64
		groupKey := "__all__"

		if hasGroupBy {
			if err := rows.Scan(&bucket, &groupKey, &value); err != nil {
				return nil, err
			}
		} else {
			if err := rows.Scan(&bucket, &value); err != nil {
				return nil, err
			}
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
	query := `SELECT name, groupUniqArrayArray(mapKeys(tags)) AS tag_keys
		FROM metric_points
		WHERE project_id = ? AND recorded_at >= ? AND recorded_at <= ?
		GROUP BY name
		ORDER BY name ASC`

	rows, err := chdb.Conn.Query(ctx, query, projectId, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var metrics []models.DiscoveredMetric
	for rows.Next() {
		var m models.DiscoveredMetric
		if err := rows.Scan(&m.Name, &m.TagKeys); err != nil {
			return nil, err
		}
		metrics = append(metrics, m)
	}
	return metrics, nil
}

func (r *metricPointRepository) DiscoverTagValues(ctx context.Context, projectId uuid.UUID, metricName, tagKey string, from, to time.Time) ([]string, error) {
	query := `SELECT DISTINCT tags[?] AS tag_value
		FROM metric_points
		WHERE project_id = ? AND name = ? AND recorded_at >= ? AND recorded_at <= ?
		AND tags[?] != ''
		ORDER BY tag_value ASC`

	rows, err := chdb.Conn.Query(ctx, query, tagKey, projectId, metricName, from, to, tagKey)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var values []string
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		values = append(values, v)
	}
	return values, nil
}

func selectTable(duration time.Duration) string {
	switch {
	case duration <= 6*time.Hour:
		return "metric_points"
	case duration <= 72*time.Hour:
		return "metric_points_1m"
	case duration <= 720*time.Hour:
		return "metric_points_1h"
	default:
		return "metric_points_1d"
	}
}

const chRateDeltasSelect = "SELECT recorded_at, tags, value - lagInFrame(value) OVER w AS delta, row_number() OVER w AS rn FROM metric_points"

const chRateWindow = "WINDOW w AS (PARTITION BY mapSort(tags) ORDER BY recorded_at ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW)"

func aggregationFunc(agg string, table string) string {
	if table == "metric_points" {
		switch agg {
		case "min":
			return "min(value)"
		case "max":
			return "max(value)"
		case "sum":
			return "sum(value)"
		case "count":
			return "toFloat64(count())"
		case "last":
			return "argMax(value, recorded_at)"
		default:
			return "avg(value)"
		}
	}
	switch agg {
	case "min":
		return "minMerge(min_val)"
	case "max":
		return "maxMerge(max_val)"
	case "sum":
		return "sumMerge(sum_val)"
	case "count":
		return "toFloat64(countMerge(count_val))"
	default:
		return "sumMerge(sum_val) / countMerge(count_val)"
	}
}

func (r *metricPointRepository) GetAverageBetween(ctx context.Context, projectId uuid.UUID, name string, start, end time.Time) (float64, error) {
	table := selectTable(end.Sub(start))
	var query string
	if table == "metric_points" {
		query = "SELECT coalesce(avg(value), 0) FROM metric_points WHERE project_id = ? AND name = ? AND recorded_at >= ? AND recorded_at <= ?"
	} else {
		query = "SELECT coalesce(sumMerge(sum_val) / countMerge(count_val), 0) FROM " + table + " WHERE project_id = ? AND name = ? AND recorded_at >= ? AND recorded_at <= ?"
	}
	var avg float64
	err := chdb.Conn.QueryRow(ctx, query, projectId, name, start, end).Scan(&avg)
	return avg, err
}

func (r *metricPointRepository) GetDistinctServers(ctx context.Context, projectId uuid.UUID, start, end time.Time) ([]string, error) {
	query := `SELECT DISTINCT tags['server_name'] AS sn
		FROM metric_points
		WHERE project_id = ? AND recorded_at >= ? AND recorded_at <= ?
		AND tags['server_name'] != ''
		ORDER BY sn ASC`

	rows, err := chdb.Conn.Query(ctx, query, projectId, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var servers []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		servers = append(servers, s)
	}
	return servers, nil
}

func (r *metricPointRepository) LatestPerServer(ctx context.Context, projectId uuid.UUID, name string, since time.Time) ([]models.ServerLatestPoint, error) {
	query := `SELECT tags['server_name'] AS sn,
		argMax(value, recorded_at) AS latest_value,
		max(recorded_at) AS last_reported_at,
		argMax(tags, recorded_at) AS latest_tags
	FROM metric_points
	WHERE project_id = ? AND name = ? AND recorded_at >= ?
	AND tags['server_name'] != ''
	GROUP BY sn ORDER BY sn ASC`

	rows, err := chdb.Conn.Query(ctx, query, projectId, name, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var points []models.ServerLatestPoint
	for rows.Next() {
		var p models.ServerLatestPoint
		if err := rows.Scan(&p.ServerName, &p.Value, &p.LastReportedAt, &p.Tags); err != nil {
			return nil, err
		}
		points = append(points, p)
	}
	return points, rows.Err()
}

func (r *metricPointRepository) GetAverageByIntervalPerServer(ctx context.Context, projectId uuid.UUID, name string, start, end time.Time, intervalMinutes int, servers []string) (map[string][]models.TimeSeriesPoint, error) {
	table := selectTable(end.Sub(start))
	aggFunc := aggregationFunc("avg", table)

	query := `SELECT
		toStartOfInterval(recorded_at, INTERVAL ? MINUTE) as bucket,
		tags['server_name'] AS sn,
		` + aggFunc + ` as avg_value
	FROM ` + table + `
	WHERE project_id = ? AND name = ? AND recorded_at >= ? AND recorded_at <= ?`

	args := []interface{}{intervalMinutes, projectId, name, start, end}

	if len(servers) > 0 {
		query += " AND tags['server_name'] IN (?)"
		args = append(args, servers)
	} else {
		query += " AND tags['server_name'] != ''"
	}

	query += " GROUP BY bucket, sn ORDER BY bucket ASC, sn ASC"

	rows, err := chdb.Conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string][]models.TimeSeriesPoint)
	for rows.Next() {
		var bucket time.Time
		var serverName string
		var value float64
		if err := rows.Scan(&bucket, &serverName, &value); err != nil {
			return nil, err
		}
		result[serverName] = append(result[serverName], models.TimeSeriesPoint{
			Timestamp: bucket,
			Value:     value,
		})
	}
	return result, nil
}

var MetricPointRepository = &metricPointRepository{}
