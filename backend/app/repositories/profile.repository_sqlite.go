//go:build !pgch

package repositories

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/lit/v2"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/profiling"
)

type profileGroupRow struct {
	ServiceName  string     `lit:"service_name"`
	ProfileType  string     `lit:"profile_type"`
	ProfileCount int64      `lit:"profile_count"`
	SampleCount  int64      `lit:"sample_count"`
	TotalValue   int64      `lit:"total_value"`
	LastSeen     SQLiteTime `lit:"last_seen"`
}

type labelValueRow struct {
	Value string `lit:"v"`
}

func init() {
	models.ExtensionModelRegistrations = append(models.ExtensionModelRegistrations, func(driver lit.Driver) {
		lit.RegisterModel[profileGroupRow](driver)
		lit.RegisterModel[labelValueRow](driver)
	})
}

type profileRepository struct{}

func (r *profileRepository) InsertStacksAsync(ctx context.Context, stacks []models.ProfileStack) error {
	if len(stacks) == 0 {
		return nil
	}
	tx, err := db.TelemetryDB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, s := range stacks {
		stackJSON, err := json.Marshal(s.Stack)
		if err != nil {
			return err
		}
		query, args, err := lit.ParseNamedQuery(db.Driver,
			"INSERT OR REPLACE INTO profiling_stacks (project_id, service_name, stack_hash, stack, last_seen) VALUES (:project_id, :service_name, :stack_hash, :stack, :last_seen)",
			lit.P{
				"project_id":   s.ProjectId,
				"service_name": s.ServiceName,
				"stack_hash":   int64(s.StackHash),
				"stack":        string(stackJSON),
				"last_seen":    NewSQLiteTime(s.LastSeen),
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

func (r *profileRepository) InsertSamplesAsync(ctx context.Context, samples []models.ProfileSample) error {
	if len(samples) == 0 {
		return nil
	}
	tx, err := db.TelemetryDB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, s := range samples {
		labelsVal, _ := NewSQLiteJSONMap(s.Labels).Value()
		query, args, err := lit.ParseNamedQuery(db.Driver,
			"INSERT INTO profiling_samples (project_id, profile_id, service_name, type, start_time, end_time, stack_hash, value, labels, server_name, app_version, trace_id, span_id) VALUES (:project_id, :profile_id, :service_name, :type, :start_time, :end_time, :stack_hash, :value, :labels, :server_name, :app_version, :trace_id, :span_id)",
			lit.P{
				"project_id":   s.ProjectId,
				"profile_id":   s.ProfileId,
				"service_name": s.ServiceName,
				"type":         s.Type,
				"start_time":   NewSQLiteTime(s.Start),
				"end_time":     NewSQLiteTime(s.End),
				"stack_hash":   int64(s.StackHash),
				"value":        s.Value,
				"labels":       labelsVal,
				"server_name":  s.ServerName,
				"app_version":  s.AppVersion,
				"trace_id":     s.TraceId,
				"span_id":      s.SpanId,
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

func (r *profileRepository) InsertProfilesAsync(ctx context.Context, profiles []models.Profile) error {
	if len(profiles) == 0 {
		return nil
	}
	tx, err := db.TelemetryDB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, p := range profiles {
		attributesVal, _ := NewSQLiteJSONMap(p.Attributes).Value()
		query, args, err := lit.ParseNamedQuery(db.Driver,
			"INSERT INTO profiles (id, project_id, recorded_at, duration, service_name, profile_type, sample_count, total_value, server_name, app_version, attributes, storage_key, trace_id, span_id, distributed_trace_id) VALUES (:id, :project_id, :recorded_at, :duration, :service_name, :profile_type, :sample_count, :total_value, :server_name, :app_version, :attributes, :storage_key, :trace_id, :span_id, :distributed_trace_id)",
			lit.P{
				"id":                   p.Id,
				"project_id":           p.ProjectId,
				"recorded_at":          NewSQLiteTime(p.RecordedAt),
				"duration":             int64(p.Duration),
				"service_name":         p.ServiceName,
				"profile_type":         p.ProfileType,
				"sample_count":         int64(p.SampleCount),
				"total_value":          p.TotalValue,
				"server_name":          p.ServerName,
				"app_version":          p.AppVersion,
				"attributes":           attributesVal,
				"storage_key":          p.StorageKey,
				"trace_id":             p.TraceId,
				"span_id":              p.SpanId,
				"distributed_trace_id": p.DistributedTraceId,
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

func (r *profileRepository) FindGroupedByService(ctx context.Context, projectId uuid.UUID, from, to time.Time, page, pageSize int, orderBy, sortDirection string) ([]models.ProfileGroup, int64, error) {
	params := lit.P{"project_id": projectId, "from": NewSQLiteTime(from), "to": NewSQLiteTime(to)}

	countResult, err := lit.SelectSingleNamed[models.CountResult](db.TelemetryDB,
		`SELECT COUNT(*) AS count FROM (
			SELECT 1 FROM profiles
			WHERE project_id = :project_id AND recorded_at >= :from AND recorded_at <= :to
			GROUP BY service_name, profile_type)`, params)
	if err != nil {
		return nil, 0, err
	}
	total := int64(0)
	if countResult != nil {
		total = int64(countResult.Count)
	}

	orderByMap := map[string]string{
		"total_value":   "total_value",
		"sample_count":  "sample_count",
		"profile_count": "profile_count",
		"last_seen":     "last_seen",
	}
	orderExpr, ok := orderByMap[orderBy]
	if !ok {
		orderExpr = "total_value"
	}
	sortDir := "DESC"
	if sortDirection == "asc" {
		sortDir = "ASC"
	}

	offset := (page - 1) * pageSize
	rows, err := lit.SelectNamed[profileGroupRow](db.TelemetryDB,
		fmt.Sprintf(`SELECT service_name, profile_type,
			COUNT(*) AS profile_count,
			SUM(sample_count) AS sample_count,
			SUM(total_value) AS total_value,
			MAX(recorded_at) AS last_seen
		FROM profiles
		WHERE project_id = :project_id AND recorded_at >= :from AND recorded_at <= :to
		GROUP BY service_name, profile_type
		ORDER BY %s %s
		LIMIT :limit OFFSET :offset`, orderExpr, sortDir),
		lit.P{"project_id": projectId, "from": NewSQLiteTime(from), "to": NewSQLiteTime(to), "limit": pageSize, "offset": offset})
	if err != nil {
		return nil, 0, err
	}

	groups := make([]models.ProfileGroup, 0, len(rows))
	for _, row := range rows {
		groups = append(groups, models.ProfileGroup{
			ServiceName:  row.ServiceName,
			Type:         row.ProfileType,
			ProfileCount: row.ProfileCount,
			SampleCount:  row.SampleCount,
			TotalValue:   row.TotalValue,
			LastSeen:     row.LastSeen.Time,
		})
	}
	return groups, total, nil
}

func (r *profileRepository) GetSeries(ctx context.Context, projectId uuid.UUID, service, profileType string, from, to time.Time, intervalMinutes int) ([]models.TimeSeriesPoint, error) {
	if intervalMinutes < 1 {
		intervalMinutes = 1
	}
	secs := intervalMinutes * 60
	agg := "SUM"
	if profiling.IsGauge(profileType) {
		agg = "AVG"
	}

	results, err := lit.SelectNamed[timeSeriesResult](db.TelemetryDB,
		fmt.Sprintf(`SELECT datetime((strftime('%%s', start_time) / %d) * %d, 'unixepoch') AS bucket,
			CAST(%s(value) AS REAL) AS agg_value
		FROM profiling_samples
		WHERE project_id = :project_id AND type = :type AND service_name = :service
			AND start_time >= :from AND start_time <= :to
		GROUP BY bucket ORDER BY bucket ASC`, secs, secs, agg),
		lit.P{"project_id": projectId, "type": profileType, "service": service, "from": NewSQLiteTime(from), "to": NewSQLiteTime(to)})
	if err != nil {
		return nil, err
	}
	return timeSeriesResultsToPoints(results), nil
}

func (r *profileRepository) GetFlameGraph(ctx context.Context, projectId uuid.UUID, service, profileType string, from, to time.Time, labelFilters map[string]string) ([]models.ProfileStackValue, error) {
	params := lit.P{"project_id": projectId, "type": profileType, "service": service, "from": NewSQLiteTime(from), "to": NewSQLiteTime(to)}
	bareFilter := sqliteLabelFilter("", labelFilters, params)
	aliasFilter := sqliteLabelFilter("s.", labelFilters, params)

	var query string
	if profiling.IsGauge(profileType) {
		query = `WITH latest AS (
			SELECT profile_id AS pid, MAX(start_time) AS mx
			FROM profiling_samples
			WHERE project_id = :project_id AND type = :type AND service_name = :service
				AND start_time >= :from AND start_time <= :to` + bareFilter + `
			GROUP BY server_name)
		SELECT st.stack AS stack_json, CAST(SUM(s.value) AS INTEGER) AS v
		FROM profiling_samples s
		JOIN latest l ON l.pid = s.profile_id
		JOIN profiling_stacks st ON st.project_id = s.project_id AND st.service_name = s.service_name AND st.stack_hash = s.stack_hash
		WHERE s.project_id = :project_id AND s.type = :type AND s.service_name = :service
			AND s.start_time >= :from AND s.start_time <= :to` + aliasFilter + `
		GROUP BY s.stack_hash, st.stack`
	} else {
		query = `SELECT st.stack AS stack_json, CAST(SUM(s.value) AS INTEGER) AS v
		FROM profiling_samples s
		JOIN profiling_stacks st ON st.project_id = s.project_id AND st.service_name = s.service_name AND st.stack_hash = s.stack_hash
		WHERE s.project_id = :project_id AND s.type = :type AND s.service_name = :service
			AND s.start_time >= :from AND s.start_time <= :to` + aliasFilter + `
		GROUP BY s.stack_hash, st.stack`
	}

	parsed, args, err := lit.ParseNamedQuery(db.Driver, query, params)
	if err != nil {
		return nil, err
	}
	sqlRows, err := db.TelemetryDB.QueryContext(ctx, parsed, args...)
	if err != nil {
		return nil, err
	}
	defer sqlRows.Close()

	var out []models.ProfileStackValue
	for sqlRows.Next() {
		var stackJSON string
		var value int64
		if err := sqlRows.Scan(&stackJSON, &value); err != nil {
			return nil, err
		}
		var frames []string
		if err := json.Unmarshal([]byte(stackJSON), &frames); err != nil {
			return nil, err
		}
		out = append(out, models.ProfileStackValue{Stack: frames, Value: value})
	}
	return out, sqlRows.Err()
}

func (r *profileRepository) distinctLabelValues(ctx context.Context, projectId uuid.UUID, service, profileType, key string, from, to time.Time) ([]string, error) {
	results, err := lit.SelectNamed[labelValueRow](db.TelemetryDB,
		fmt.Sprintf(`SELECT DISTINCT json_extract(labels, '$."' || :key || '"') AS v
		FROM profiling_samples
		WHERE project_id = :project_id AND type = :type AND service_name = :service
			AND start_time >= :from AND start_time <= :to
			AND json_extract(labels, '$."' || :key || '"') IS NOT NULL
			AND json_extract(labels, '$."' || :key || '"') != ''
		ORDER BY v ASC
		LIMIT %d`, profiling.MaxLabelValuesPerKey),
		lit.P{"project_id": projectId, "type": profileType, "service": service, "key": key, "from": NewSQLiteTime(from), "to": NewSQLiteTime(to)})
	if err != nil {
		return nil, err
	}

	values := make([]string, 0, len(results))
	for _, row := range results {
		values = append(values, row.Value)
	}
	return values, nil
}

func sqliteLabelFilter(qualifier string, filters map[string]string, params lit.P) string {
	if len(filters) == 0 {
		return ""
	}
	keys := make([]string, 0, len(filters))
	for k := range filters {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	clause := ""
	for i, k := range keys {
		pathKey := fmt.Sprintf("lblpath_%d", i)
		valKey := fmt.Sprintf("lblval_%d", i)
		clause += fmt.Sprintf(" AND json_extract(%slabels, :%s) = :%s", qualifier, pathKey, valKey)
		params[pathKey] = "$.\"" + k + "\""
		params[valKey] = filters[k]
	}
	return clause
}

var ProfileRepository = profileRepository{}
