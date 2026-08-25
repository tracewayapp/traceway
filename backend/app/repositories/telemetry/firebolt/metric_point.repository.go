//go:build telemetry_firebolt

package firebolt

import (
	"context"
	"database/sql"
	"fmt"
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

type tagValueRow struct {
	TagValue string `lit:"tag_value"`
}

func init() {
	registerModels(func(driver lit.Driver) {
		lit.RegisterModel[avgResult](driver)
		lit.RegisterModel[distinctServerResult](driver)
		lit.RegisterModel[tagValueRow](driver)
	})
}

var metricPointColumns = []string{"project_id", "name", "value", "tags", "recorded_at", "server_name"}

func (r *metricPointRepository) InsertAsync(ctx context.Context, points []models.MetricPoint) error {
	if len(points) == 0 {
		return nil
	}

	rows := make([][]any, 0, len(points))
	for _, p := range points {
		tagsJSON, err := attrJSON(p.Tags)
		if err != nil {
			return err
		}
		rows = append(rows, []any{
			p.ProjectId.String(),
			p.Name,
			p.Value,
			tagsJSON,
			p.RecordedAt.UTC(),
			p.Tags["server_name"],
		})
	}
	return insertRows(ctx, "metric_points", metricPointColumns, rows)
}

func (r *metricPointRepository) QueryTimeSeries(ctx context.Context, projectId uuid.UUID, name string, from, to time.Time, intervalMinutes int, aggregation string, tagFilters map[string]string, groupBy string) (map[string][]models.TimeSeriesPoint, error) {
	secs := intervalMinutes * 60
	aggFunc := fireboltAggregationFunc(aggregation)
	hasGroupBy := groupBy != ""

	// The '/host' pointer must appear as a literal to match the index key
	// expression, so only host-tag queries take the indexed path; other
	// tags and the MAX_BY "last" aggregation fall back to the raw scan.
	indexable := aggregation != "last"
	if hasGroupBy && groupBy != "host" {
		indexable = false
	}
	for k := range tagFilters {
		if k != "host" {
			indexable = false
		}
	}

	params := lit.P{
		"project_id": projectId,
		"name":       name,
	}

	var timeFilter, bucketExpr, hostExpr string
	if indexable {
		bindMinuteRange(params, from, to)
		timeFilter = indexMinuteRange("recorded_at")
		bucketExpr = indexBucketExpr("recorded_at", secs)
		hostExpr = "JSON_POINTER_EXTRACT_TEXT(tags, '/host')"
	} else {
		params["from"] = from.UTC()
		params["to"] = to.UTC()
		timeFilter = "recorded_at >= :from AND recorded_at <= :to"
		bucketExpr = timeBucketExpr("recorded_at", secs)
	}

	selectClause := "SELECT " + bucketExpr + " AS bucket"
	if hasGroupBy {
		if indexable {
			selectClause += ", " + hostExpr + " AS group_key"
		} else {
			selectClause += ", " + jsonExtractExpr("tags", ":group_by") + " AS group_key"
			params["group_by"] = jsonPointerEscape(groupBy)
		}
	}
	selectClause += ", " + aggFunc + " AS agg_value FROM metric_points WHERE project_id = :project_id AND name = :name AND " + timeFilter

	filterClauses := ""
	for i, k := range shared.SortedKeys(tagFilters) {
		fv := fmt.Sprintf("fv_%d", i)
		if indexable {
			filterClauses += " AND " + hostExpr + " = :" + fv
		} else {
			fk := fmt.Sprintf("fk_%d", i)
			filterClauses += " AND " + jsonExtractExpr("tags", ":"+fk) + " = :" + fv
			params[fk] = jsonPointerEscape(k)
		}
		params[fv] = tagFilters[k]
	}

	query := selectClause + filterClauses + " GROUP BY bucket"
	if hasGroupBy {
		query += ", group_key"
	}
	query += " ORDER BY bucket ASC"

	parsedQuery, args, err := parseNamed(query, params)
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
		var bucket time.Time
		var value float64
		groupKey := "__all__"

		if hasGroupBy {
			var groupKeyNullable *string
			if err := rows.Scan(&bucket, &groupKeyNullable, &value); err != nil {
				return nil, err
			}
			if groupKeyNullable != nil {
				groupKey = *groupKeyNullable
			}
		} else {
			if err := rows.Scan(&bucket, &value); err != nil {
				return nil, err
			}
		}

		if groupKey == "" {
			groupKey = "(empty)"
		}
		result[groupKey] = append(result[groupKey], models.TimeSeriesPoint{
			Timestamp: bucket,
			Value:     value,
		})
	}
	return result, nil
}

func (r *metricPointRepository) DiscoverMetrics(ctx context.Context, projectId uuid.UUID, from, to time.Time) ([]models.DiscoveredMetric, error) {
	query, args, err := parseNamed(
		`SELECT name, COALESCE(t.k, '') AS tag_key
		FROM metric_points
		LEFT JOIN UNNEST(JSON_POINTER_EXTRACT_KEYS(tags, '')) AS t(k) ON true
		WHERE project_id = :project_id AND recorded_at >= :from AND recorded_at <= :to
		GROUP BY name, t.k
		ORDER BY name ASC, t.k ASC`,
		lit.P{"project_id": projectId, "from": from.UTC(), "to": to.UTC()})
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
	results, err := lit.SelectNamed[tagValueRow](db.TelemetryDB,
		`SELECT DISTINCT `+jsonExtractExpr("tags", ":tag_key")+` AS tag_value
		FROM metric_points
		WHERE project_id = :project_id AND name = :name AND recorded_at >= :from AND recorded_at <= :to
		AND `+jsonExtractExpr("tags", ":tag_key")+` IS NOT NULL
		AND `+jsonExtractExpr("tags", ":tag_key")+` != ''
		ORDER BY tag_value ASC`,
		lit.P{"project_id": projectId, "name": metricName, "tag_key": jsonPointerEscape(tagKey), "from": from.UTC(), "to": to.UTC()})
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
		"SELECT COALESCE(avg(value), 0) AS agg_value FROM metric_points WHERE project_id = :project_id AND name = :name AND "+indexMinuteRange("recorded_at"),
		func() lit.P {
			p := minuteRangeParams(projectId, start, end)
			p["name"] = name
			return p
		}())
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
		`SELECT DISTINCT server_name AS sn
		FROM metric_points
		WHERE project_id = :project_id AND DATE_TRUNC('minute', recorded_at) >= :from_min AND DATE_TRUNC('minute', recorded_at) <= :to_min
		AND server_name != ''
		ORDER BY sn ASC`,
		minuteRangeParams(projectId, start, end))
	if err != nil {
		return nil, err
	}

	servers := make([]string, 0, len(results))
	for _, r := range results {
		servers = append(servers, r.ServerName)
	}
	return servers, nil
}

func (r *metricPointRepository) GetAverageByIntervalPerServer(ctx context.Context, projectId uuid.UUID, name string, start, end time.Time, intervalMinutes int, servers []string) (map[string][]models.TimeSeriesPoint, error) {
	secs := intervalMinutes * 60

	params := minuteRangeParams(projectId, start, end)
	params["name"] = name

	query := fmt.Sprintf(`SELECT
		%s AS bucket,
		server_name AS sn,
		avg(value) AS avg_value
	FROM metric_points
	WHERE project_id = :project_id AND name = :name AND DATE_TRUNC('minute', recorded_at) >= :from_min AND DATE_TRUNC('minute', recorded_at) <= :to_min`, indexBucketExpr("recorded_at", secs))

	if len(servers) > 0 {
		placeholders := make([]string, len(servers))
		for i, s := range servers {
			key := fmt.Sprintf("srv_%d", i)
			placeholders[i] = ":" + key
			params[key] = s
		}
		query += " AND server_name IN (" + strings.Join(placeholders, ", ") + ")"
	} else {
		query += " AND server_name != ''"
	}

	query += " GROUP BY bucket, sn ORDER BY bucket ASC, sn ASC"

	parsedQuery, args, err := parseNamed(query, params)
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

func fireboltAggregationFunc(agg string) string {
	switch agg {
	case "min":
		return "min(value)"
	case "max":
		return "max(value)"
	case "sum":
		return "sum(value)"
	case "count":
		return "CAST(COUNT(*) AS DOUBLE PRECISION)"
	case "last":
		return "MAX_BY(value, recorded_at)"
	default:
		return "avg(value)"
	}
}

var MetricPointRepository = &metricPointRepository{}
