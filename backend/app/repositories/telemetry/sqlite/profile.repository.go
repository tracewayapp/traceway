//go:build !telemetry_ch && !telemetry_duckdb && !telemetry_firebolt

package sqlite

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry/shared"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry/sqlitetypes"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/lit/v2"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/profiling"
)

type profileGroupRow struct {
	ServiceName  string                 `lit:"service_name"`
	ProfileType  string                 `lit:"profile_type"`
	Unit         string                 `lit:"unit"`
	IsGauge      int64                  `lit:"is_gauge"`
	ProfileCount int64                  `lit:"profile_count"`
	SampleCount  int64                  `lit:"sample_count"`
	TotalValue   int64                  `lit:"total_value"`
	LastSeen     sqlitetypes.SQLiteTime `lit:"last_seen"`
}

type labelValueRow struct {
	Value string `lit:"v"`
}

type profileTypeRow struct {
	Type    string `lit:"type"`
	Unit    string `lit:"unit"`
	IsGauge int64  `lit:"is_gauge"`
}

type profileDimValueRow struct {
	Key   *string `lit:"k"`
	Total float64 `lit:"tv"`
}

func init() {
	models.ExtensionModelRegistrations = append(models.ExtensionModelRegistrations, func(driver lit.Driver) {
		lit.RegisterModel[profileGroupRow](driver)
		lit.RegisterModel[labelValueRow](driver)
		lit.RegisterModel[profileTypeRow](driver)
		lit.RegisterModel[profileDimValueRow](driver)
	})
}

type profileRepository struct{}

func boolToInt(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

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
				"last_seen":    sqlitetypes.NewSQLiteTime(s.LastSeen),
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
		labelsVal, _ := sqlitetypes.NewSQLiteJSONMap(s.Labels).Value()
		query, args, err := lit.ParseNamedQuery(db.Driver,
			"INSERT INTO profiling_samples (project_id, profile_id, service_name, type, unit, is_gauge, start_time, end_time, stack_hash, value, labels, server_name, app_version, trace_id, span_id) VALUES (:project_id, :profile_id, :service_name, :type, :unit, :is_gauge, :start_time, :end_time, :stack_hash, :value, :labels, :server_name, :app_version, :trace_id, :span_id)",
			lit.P{
				"project_id":   s.ProjectId,
				"profile_id":   s.ProfileId,
				"service_name": s.ServiceName,
				"type":         s.Type,
				"unit":         s.Unit,
				"is_gauge":     boolToInt(s.IsGauge),
				"start_time":   sqlitetypes.NewSQLiteTime(s.Start),
				"end_time":     sqlitetypes.NewSQLiteTime(s.End),
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
		attributesVal, _ := sqlitetypes.NewSQLiteJSONMap(p.Attributes).Value()
		query, args, err := lit.ParseNamedQuery(db.Driver,
			"INSERT INTO profiles (id, project_id, recorded_at, duration, service_name, profile_type, unit, is_gauge, sample_count, total_value, server_name, app_version, attributes, storage_key, trace_id, span_id, distributed_trace_id) VALUES (:id, :project_id, :recorded_at, :duration, :service_name, :profile_type, :unit, :is_gauge, :sample_count, :total_value, :server_name, :app_version, :attributes, :storage_key, :trace_id, :span_id, :distributed_trace_id)",
			lit.P{
				"id":                   p.Id,
				"project_id":           p.ProjectId,
				"recorded_at":          sqlitetypes.NewSQLiteTime(p.RecordedAt),
				"duration":             int64(p.Duration),
				"service_name":         p.ServiceName,
				"profile_type":         p.ProfileType,
				"unit":                 p.Unit,
				"is_gauge":             boolToInt(p.IsGauge),
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

func (r *profileRepository) FindGroupedByService(ctx context.Context, projectId uuid.UUID, from, to time.Time, page, pageSize int, orderBy, sortDirection, search string) ([]models.ProfileGroup, int64, error) {
	searchClause := ""
	countParams := lit.P{"project_id": projectId, "from": sqlitetypes.NewSQLiteTime(from), "to": sqlitetypes.NewSQLiteTime(to)}
	if search != "" {
		searchClause = " AND instr(lower(service_name), lower(:search)) > 0"
		countParams["search"] = search
	}

	countResult, err := lit.SelectSingleNamed[models.CountResult](db.TelemetryDB,
		`SELECT COUNT(*) AS count FROM (
			SELECT 1 FROM profiles
			WHERE project_id = :project_id AND recorded_at >= :from AND recorded_at <= :to`+searchClause+`
			GROUP BY service_name, profile_type)`, countParams)
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
	pageParams := lit.P{"project_id": projectId, "from": sqlitetypes.NewSQLiteTime(from), "to": sqlitetypes.NewSQLiteTime(to), "limit": pageSize, "offset": offset}
	if search != "" {
		pageParams["search"] = search
	}
	rows, err := lit.SelectNamed[profileGroupRow](db.TelemetryDB,
		fmt.Sprintf(`SELECT service_name, profile_type,
			MAX(unit) AS unit,
			MAX(is_gauge) AS is_gauge,
			COUNT(*) AS profile_count,
			SUM(sample_count) AS sample_count,
			SUM(total_value) AS total_value,
			MAX(recorded_at) AS last_seen
		FROM profiles
		WHERE project_id = :project_id AND recorded_at >= :from AND recorded_at <= :to`+searchClause+`
		GROUP BY service_name, profile_type
		ORDER BY %s %s
		LIMIT :limit OFFSET :offset`, orderExpr, sortDir),
		pageParams)
	if err != nil {
		return nil, 0, err
	}

	groups := make([]models.ProfileGroup, 0, len(rows))
	for _, row := range rows {
		groups = append(groups, models.ProfileGroup{
			ServiceName:  row.ServiceName,
			Type:         row.ProfileType,
			Unit:         row.Unit,
			IsGauge:      row.IsGauge != 0,
			ProfileCount: row.ProfileCount,
			SampleCount:  row.SampleCount,
			TotalValue:   row.TotalValue,
			LastSeen:     row.LastSeen.Time,
		})
	}

	if err := r.fillSparklines(ctx, projectId, from, to, groups); err != nil {
		return nil, 0, err
	}
	return groups, total, nil
}

func (r *profileRepository) fillSparklines(ctx context.Context, projectId uuid.UUID, from, to time.Time, groups []models.ProfileGroup) error {
	if len(groups) == 0 {
		return nil
	}
	const buckets = 16
	bucketSecs := int(to.Sub(from).Seconds()) / buckets
	if bucketSecs < 1 {
		bucketSecs = 1
	}
	alignedFrom := (from.Unix() / int64(bucketSecs)) * int64(bucketSecs)

	query, args, err := lit.ParseNamedQuery(db.Driver,
		fmt.Sprintf(`SELECT service_name, profile_type,
			CAST((strftime('%%s', recorded_at) / %d) * %d AS INTEGER) AS bucket,
			CAST(SUM(total_value) AS REAL) AS v
		FROM profiles
		WHERE project_id = :project_id AND recorded_at >= :from AND recorded_at <= :to
		GROUP BY service_name, profile_type, bucket`, bucketSecs, bucketSecs),
		lit.P{"project_id": projectId, "from": sqlitetypes.NewSQLiteTime(from), "to": sqlitetypes.NewSQLiteTime(to)})
	if err != nil {
		return err
	}
	sqlRows, err := db.TelemetryDB.QueryContext(ctx, query, args...)
	if err != nil {
		return err
	}
	defer sqlRows.Close()

	acc := map[string][]float64{}
	for sqlRows.Next() {
		var service, ptype string
		var bucketEpoch int64
		var v float64
		if err := sqlRows.Scan(&service, &ptype, &bucketEpoch, &v); err != nil {
			return err
		}
		idx := int((bucketEpoch - alignedFrom) / int64(bucketSecs))
		if idx < 0 {
			idx = 0
		}
		if idx >= buckets {
			idx = buckets - 1
		}
		key := service + "\x00" + ptype
		arr, ok := acc[key]
		if !ok {
			arr = make([]float64, buckets)
			acc[key] = arr
		}
		arr[idx] += v
	}
	if err := sqlRows.Err(); err != nil {
		return err
	}

	for i := range groups {
		if arr, ok := acc[groups[i].ServiceName+"\x00"+groups[i].Type]; ok {
			groups[i].Sparkline = arr
		} else {
			groups[i].Sparkline = []float64{}
		}
	}
	return nil
}

func (r *profileRepository) DiscoverTypes(ctx context.Context, projectId uuid.UUID, service string, from, to time.Time) ([]models.ProfileTypeInfo, error) {
	rows, err := lit.SelectNamed[profileTypeRow](db.TelemetryDB,
		`SELECT type, MAX(unit) AS unit, MAX(is_gauge) AS is_gauge
		FROM profiling_samples
		WHERE project_id = :project_id AND service_name = :service
			AND start_time >= :from AND start_time <= :to
		GROUP BY type
		ORDER BY type ASC`,
		lit.P{"project_id": projectId, "service": service, "from": sqlitetypes.NewSQLiteTime(from), "to": sqlitetypes.NewSQLiteTime(to)})
	if err != nil {
		return nil, err
	}

	out := make([]models.ProfileTypeInfo, 0, len(rows))
	for _, row := range rows {
		out = append(out, models.ProfileTypeInfo{Type: row.Type, Unit: row.Unit, IsGauge: row.IsGauge != 0})
	}
	return out, nil
}

func (r *profileRepository) GetSeries(ctx context.Context, projectId uuid.UUID, service, profileType string, from, to time.Time, intervalMinutes int, isGauge bool, labelFilters map[string]string, appVersion, serverName string, normalize bool) ([]models.TimeSeriesPoint, error) {
	if intervalMinutes < 1 {
		intervalMinutes = 1
	}
	secs := intervalMinutes * 60
	agg := "SUM"
	if isGauge {
		agg = "AVG"
	}

	params := lit.P{"project_id": projectId, "type": profileType, "service": service, "from": sqlitetypes.NewSQLiteTime(from), "to": sqlitetypes.NewSQLiteTime(to)}
	filter := sqliteLabelFilter("", labelFilters, params)
	cohort := sqliteCohortFilter("", appVersion, serverName, params)

	results, err := lit.SelectNamed[sqlitetypes.TimeSeriesResult](db.TelemetryDB,
		fmt.Sprintf(`SELECT datetime((strftime('%%s', start_time) / %d) * %d, 'unixepoch') AS bucket,
			CAST(%s(value) AS REAL) AS agg_value
		FROM profiling_samples
		WHERE project_id = :project_id AND type = :type AND service_name = :service
			AND start_time >= :from AND start_time <= :to`+filter+cohort+`
		GROUP BY bucket ORDER BY bucket ASC`, secs, secs, agg),
		params)
	if err != nil {
		return nil, err
	}
	points := sqlitetypes.TimeSeriesResultsToPoints(results)
	shared.NormalizeSeriesPoints(points, normalize, isGauge, intervalMinutes)
	return points, nil
}

func (r *profileRepository) GetFlameGraph(ctx context.Context, projectId uuid.UUID, service, profileType string, from, to time.Time, labelFilters map[string]string, isGauge bool, appVersion, serverName string) ([]models.ProfileStackValue, error) {
	params := lit.P{"project_id": projectId, "type": profileType, "service": service, "from": sqlitetypes.NewSQLiteTime(from), "to": sqlitetypes.NewSQLiteTime(to)}
	bareFilter := sqliteLabelFilter("", labelFilters, params)
	aliasFilter := sqliteLabelFilter("s.", labelFilters, params)
	bareCohort := sqliteCohortFilter("", appVersion, serverName, params)
	aliasCohort := sqliteCohortFilter("s.", appVersion, serverName, params)

	var query string
	if isGauge {
		query = `WITH latest AS (
			SELECT profile_id AS pid, MAX(start_time) AS mx
			FROM profiling_samples
			WHERE project_id = :project_id AND type = :type AND service_name = :service
				AND start_time >= :from AND start_time <= :to` + bareFilter + bareCohort + `
			GROUP BY server_name)
		SELECT st.stack AS stack_json, CAST(SUM(s.value) AS INTEGER) AS v
		FROM profiling_samples s
		JOIN latest l ON l.pid = s.profile_id
		JOIN profiling_stacks st ON st.project_id = s.project_id AND st.service_name = s.service_name AND st.stack_hash = s.stack_hash
		WHERE s.project_id = :project_id AND s.type = :type AND s.service_name = :service
			AND s.start_time >= :from AND s.start_time <= :to` + aliasFilter + aliasCohort + `
		GROUP BY s.stack_hash, st.stack
		ORDER BY v DESC LIMIT 50000`
	} else {
		query = `SELECT st.stack AS stack_json, CAST(SUM(s.value) AS INTEGER) AS v
		FROM profiling_samples s
		JOIN profiling_stacks st ON st.project_id = s.project_id AND st.service_name = s.service_name AND st.stack_hash = s.stack_hash
		WHERE s.project_id = :project_id AND s.type = :type AND s.service_name = :service
			AND s.start_time >= :from AND s.start_time <= :to` + aliasFilter + aliasCohort + `
		GROUP BY s.stack_hash, st.stack
		ORDER BY v DESC LIMIT 50000`
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
		lit.P{"project_id": projectId, "type": profileType, "service": service, "key": key, "from": sqlitetypes.NewSQLiteTime(from), "to": sqlitetypes.NewSQLiteTime(to)})
	if err != nil {
		return nil, err
	}

	values := make([]string, 0, len(results))
	for _, row := range results {
		values = append(values, row.Value)
	}
	return values, nil
}

func (r *profileRepository) GetSeriesBreakdown(ctx context.Context, projectId uuid.UUID, service, profileType string, from, to time.Time, intervalMinutes int, isGauge bool, labelFilters map[string]string, appVersion, serverName string, normalize bool, dimension string, limit int) ([]models.ProfileSeriesGroup, error) {
	if intervalMinutes < 1 {
		intervalMinutes = 1
	}
	secs := intervalMinutes * 60
	limit = shared.ClampBreakdownLimit(limit)
	agg := "SUM"
	if isGauge {
		agg = "AVG"
	}

	params := lit.P{"project_id": projectId, "type": profileType, "service": service, "from": sqlitetypes.NewSQLiteTime(from), "to": sqlitetypes.NewSQLiteTime(to), "limit": limit}
	filter := sqliteLabelFilter("", labelFilters, params)
	cohort := sqliteCohortFilter("", appVersion, serverName, params)
	dimExpr, ok := sqliteDimExpr(dimension, params)
	if !ok {
		return nil, fmt.Errorf("unsupported breakdown dimension: %s", dimension)
	}

	topRows, err := lit.SelectNamed[profileDimValueRow](db.TelemetryDB,
		fmt.Sprintf(`SELECT %s AS k, CAST(%s(value) AS REAL) AS tv
		FROM profiling_samples
		WHERE project_id = :project_id AND type = :type AND service_name = :service
			AND start_time >= :from AND start_time <= :to`+filter+cohort+`
			AND %s != ''
		GROUP BY k ORDER BY tv DESC LIMIT :limit`, dimExpr, agg, dimExpr),
		params)
	if err != nil {
		return nil, err
	}

	seriesQuery := fmt.Sprintf(`SELECT datetime((strftime('%%s', start_time) / %d) * %d, 'unixepoch') AS bucket,
		CAST(%s(value) AS REAL) AS agg_value
	FROM profiling_samples
	WHERE project_id = :project_id AND type = :type AND service_name = :service
		AND start_time >= :from AND start_time <= :to`+filter+cohort+`
		AND %s = :dimval
	GROUP BY bucket ORDER BY bucket ASC`, secs, secs, agg, dimExpr)

	groups := make([]models.ProfileSeriesGroup, 0, len(topRows))
	for _, row := range topRows {
		if row.Key == nil {
			continue
		}
		key := *row.Key
		params["dimval"] = key
		results, err := lit.SelectNamed[sqlitetypes.TimeSeriesResult](db.TelemetryDB, seriesQuery, params)
		if err != nil {
			return nil, err
		}
		points := sqlitetypes.TimeSeriesResultsToPoints(results)
		shared.NormalizeSeriesPoints(points, normalize, isGauge, intervalMinutes)
		groups = append(groups, models.ProfileSeriesGroup{Key: key, Points: points})
	}
	return groups, nil
}

func (r *profileRepository) DiscoverDimensions(ctx context.Context, projectId uuid.UUID, service, profileType string, from, to time.Time) ([]string, []string, error) {
	appVersions, err := r.distinctDimensionValues(ctx, projectId, service, profileType, "app_version", from, to)
	if err != nil {
		return nil, nil, err
	}
	serverNames, err := r.distinctDimensionValues(ctx, projectId, service, profileType, "server_name", from, to)
	if err != nil {
		return nil, nil, err
	}
	return appVersions, serverNames, nil
}

func (r *profileRepository) distinctDimensionValues(ctx context.Context, projectId uuid.UUID, service, profileType, column string, from, to time.Time) ([]string, error) {
	rows, err := lit.SelectNamed[labelValueRow](db.TelemetryDB,
		fmt.Sprintf(`SELECT DISTINCT %s AS v
		FROM profiling_samples
		WHERE project_id = :project_id AND service_name = :service AND type = :type
			AND start_time >= :from AND start_time <= :to AND %s != ''
		ORDER BY %s ASC LIMIT 100`, column, column, column),
		lit.P{"project_id": projectId, "service": service, "type": profileType, "from": sqlitetypes.NewSQLiteTime(from), "to": sqlitetypes.NewSQLiteTime(to)})
	if err != nil {
		return nil, err
	}
	values := make([]string, 0, len(rows))
	for _, row := range rows {
		values = append(values, row.Value)
	}
	return values, nil
}

func sqliteCohortFilter(qualifier, appVersion, serverName string, params lit.P) string {
	clause := ""
	if appVersion != "" {
		clause += fmt.Sprintf(" AND %sapp_version = :app_version", qualifier)
		params["app_version"] = appVersion
	}
	if serverName != "" {
		clause += fmt.Sprintf(" AND %sserver_name = :server_name", qualifier)
		params["server_name"] = serverName
	}
	return clause
}

func sqliteDimExpr(dimension string, params lit.P) (string, bool) {
	switch dimension {
	case "app_version":
		return "app_version", true
	case "server_name":
		return "server_name", true
	}
	key, ok := shared.BreakdownLabelKey(dimension)
	if !ok || key == "" {
		return "", false
	}
	params["dimpath"] = "$.\"" + key + "\""
	return "json_extract(labels, :dimpath)", true
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

var ProfileRepository = &profileRepository{}

func (r *profileRepository) DiscoverLabels(ctx context.Context, projectId uuid.UUID, service, profileType string, from, to time.Time, allowKeys []string) (map[string][]string, error) {
	return shared.DiscoverLabels(ctx, allowKeys, func(ctx context.Context, key string) ([]string, error) {
		return r.distinctLabelValues(ctx, projectId, service, profileType, key, from, to)
	})
}
