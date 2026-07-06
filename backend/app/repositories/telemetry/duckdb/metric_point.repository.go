//go:build telemetry_duckdb

package duckdb

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/duckdb/duckdb-go/v2"
	"github.com/google/uuid"
	"github.com/tracewayapp/lit/v2"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
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
	models.ExtensionModelRegistrations = append(models.ExtensionModelRegistrations, func(driver lit.Driver) {
		lit.RegisterModel[avgResult](driver)
		lit.RegisterModel[distinctServerResult](driver)
		lit.RegisterModel[tagValueRow](driver)
	})
}

func (r *metricPointRepository) InsertAsync(ctx context.Context, points []models.MetricPoint) error {
	if len(points) == 0 {
		return nil
	}

	conn, err := db.DuckDBConnector.Connect(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	appender, err := duckdb.NewAppenderFromConn(conn, "", "metric_points")
	if err != nil {
		return err
	}

	for _, p := range points {
		tagsJSON := "{}"
		if len(p.Tags) > 0 {
			b, err := json.Marshal(p.Tags)
			if err != nil {
				captureDroppedRow("metric_points", err)
				continue
			}
			tagsJSON = string(b)
		}
		if err := appender.AppendRow(p.ProjectId.String(), p.Name, p.Value, tagsJSON, p.RecordedAt.UTC()); err != nil {
			captureDroppedRow("metric_points", err)
		}
	}

	return appender.Close()
}

func (r *metricPointRepository) QueryTimeSeries(ctx context.Context, projectId uuid.UUID, name string, from, to time.Time, intervalMinutes int, aggregation string, tagFilters map[string]string, groupBy string) (map[string][]models.TimeSeriesPoint, error) {
	secs := intervalMinutes * 60
	aggFunc := duckdbAggregationFunc(aggregation)
	hasGroupBy := groupBy != ""

	selectClause := "SELECT " + timeBucketExpr("recorded_at", secs) + " AS bucket"
	if hasGroupBy {
		selectClause += ", json_extract_string(tags, '$.\"' || :group_by || '\"') AS group_key"
	}
	selectClause += ", " + aggFunc + " AS agg_value FROM metric_points WHERE project_id = :project_id AND name = :name AND recorded_at >= :from AND recorded_at <= :to"

	params := lit.P{
		"project_id": projectId,
		"name":       name,
		"from":       from.UTC(),
		"to":         to.UTC(),
	}
	if hasGroupBy {
		params["group_by"] = groupBy
	}

	filterClauses := ""
	for i, k := range sortedKeys(tagFilters) {
		fk := fmt.Sprintf("fk_%d", i)
		fv := fmt.Sprintf("fv_%d", i)
		filterClauses += fmt.Sprintf(" AND json_extract_string(tags, '$.\"' || :%s || '\"') = :%s", fk, fv)
		params[fk] = k
		params[fv] = tagFilters[k]
	}

	query := selectClause + filterClauses + " GROUP BY bucket"
	if hasGroupBy {
		query += ", group_key"
	}
	query += " ORDER BY bucket ASC"

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
	query, args, err := lit.ParseNamedQuery(db.Driver,
		`SELECT name, COALESCE(t.k, '') AS tag_key
		FROM metric_points
		LEFT JOIN LATERAL unnest(json_keys(tags)) AS t(k) ON true
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
		`SELECT DISTINCT json_extract_string(tags, '$."' || :tag_key || '"') AS tag_value
		FROM metric_points
		WHERE project_id = :project_id AND name = :name AND recorded_at >= :from AND recorded_at <= :to
		AND json_extract_string(tags, '$."' || :tag_key || '"') IS NOT NULL
		AND json_extract_string(tags, '$."' || :tag_key || '"') != ''
		ORDER BY tag_value ASC`,
		lit.P{"project_id": projectId, "name": metricName, "tag_key": tagKey, "from": from.UTC(), "to": to.UTC()})
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
		lit.P{"project_id": projectId, "name": name, "from": start.UTC(), "to": end.UTC()})
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
		`SELECT DISTINCT json_extract_string(tags, '$.server_name') AS sn
		FROM metric_points
		WHERE project_id = :project_id AND recorded_at >= :from AND recorded_at <= :to
		AND json_extract_string(tags, '$.server_name') IS NOT NULL
		AND json_extract_string(tags, '$.server_name') != ''
		ORDER BY sn ASC`,
		lit.P{"project_id": projectId, "from": start.UTC(), "to": end.UTC()})
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

	params := lit.P{
		"project_id": projectId,
		"name":       name,
		"from":       start.UTC(),
		"to":         end.UTC(),
	}

	query := fmt.Sprintf(`SELECT
		%s AS bucket,
		json_extract_string(tags, '$.server_name') AS sn,
		avg(value) AS avg_value
	FROM metric_points
	WHERE project_id = :project_id AND name = :name AND recorded_at >= :from AND recorded_at <= :to`, timeBucketExpr("recorded_at", secs))

	if len(servers) > 0 {
		placeholders := make([]string, len(servers))
		for i, s := range servers {
			key := fmt.Sprintf("srv_%d", i)
			placeholders[i] = ":" + key
			params[key] = s
		}
		query += " AND json_extract_string(tags, '$.server_name') IN (" + strings.Join(placeholders, ", ") + ")"
	} else {
		query += " AND json_extract_string(tags, '$.server_name') IS NOT NULL AND json_extract_string(tags, '$.server_name') != ''"
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

func duckdbAggregationFunc(agg string) string {
	switch agg {
	case "min":
		return "min(value)"
	case "max":
		return "max(value)"
	case "sum":
		return "sum(value)"
	case "count":
		return "CAST(COUNT(*) AS DOUBLE)"
	default:
		return "avg(value)"
	}
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

var MetricPointRepository = &metricPointRepository{}
