//go:build telemetry_firebolt

package firebolt

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/lit/v2"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/profiling"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry/shared"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry/sqlitetypes"
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
	registerModels(func(driver lit.Driver) {
		lit.RegisterModel[profileGroupRow](driver)
		lit.RegisterModel[labelValueRow](driver)
		lit.RegisterModel[profileTypeRow](driver)
		lit.RegisterModel[profileDimValueRow](driver)
	})
}

type profileRepository struct{}

func (r *profileRepository) InsertStacksAsync(ctx context.Context, stacks []models.ProfileStack) error {
	if len(stacks) == 0 {
		return nil
	}

	for _, s := range stacks {
		stackJSON := "[]"
		if len(s.Stack) > 0 {
			b, err := json.Marshal(s.Stack)
			if err != nil {
				return err
			}
			stackJSON = string(b)
		}
		query, args, err := parseNamed(
			"INSERT INTO profiling_stacks (project_id, service_name, stack_hash, stack, last_seen) VALUES (:project_id, :service_name, :stack_hash, :stack, :last_seen) ON CONFLICT (project_id, service_name, stack_hash) DO UPDATE SET last_seen = excluded.last_seen",
			lit.P{
				"project_id":   s.ProjectId.String(),
				"service_name": s.ServiceName,
				"stack_hash":   int64(s.StackHash),
				"stack":        stackJSON,
				"last_seen":    s.LastSeen.UTC(),
			})
		if err != nil {
			return err
		}
		if _, err := db.TelemetryDB.ExecContext(ctx, query, args...); err != nil {
			db.RecordTelemetryInsertFailure()
			return err
		}
	}
	return nil
}

var profilingSampleColumns = []string{"project_id", "profile_id", "service_name", "type", "start_time", "end_time", "stack_hash", "value", "labels", "server_name", "app_version", "trace_id", "span_id", "unit", "is_gauge"}

func (r *profileRepository) InsertSamplesAsync(ctx context.Context, samples []models.ProfileSample) error {
	if len(samples) == 0 {
		return nil
	}

	rows := make([][]any, 0, len(samples))
	for _, s := range samples {
		labelsJSON, err := attrJSON(s.Labels)
		if err != nil {
			return err
		}
		rows = append(rows, []any{
			s.ProjectId.String(),
			s.ProfileId.String(),
			s.ServiceName,
			s.Type,
			s.Start.UTC(),
			s.End.UTC(),
			int64(s.StackHash),
			s.Value,
			labelsJSON,
			s.ServerName,
			s.AppVersion,
			s.TraceId,
			s.SpanId,
			s.Unit,
			boolToInt(s.IsGauge),
		})
	}
	return insertRows(ctx, "profiling_samples", profilingSampleColumns, rows)
}

var profileColumns = []string{"id", "project_id", "recorded_at", "duration", "service_name", "profile_type", "sample_count", "total_value", "server_name", "app_version", "attributes", "storage_key", "trace_id", "span_id", "distributed_trace_id", "unit", "is_gauge"}

func (r *profileRepository) InsertProfilesAsync(ctx context.Context, profiles []models.Profile) error {
	if len(profiles) == 0 {
		return nil
	}

	rows := make([][]any, 0, len(profiles))
	for _, p := range profiles {
		attributesJSON, err := attrJSON(p.Attributes)
		if err != nil {
			return err
		}

		var distributedTraceId *string
		if p.DistributedTraceId != nil {
			v := p.DistributedTraceId.String()
			distributedTraceId = &v
		}

		rows = append(rows, []any{
			p.Id.String(),
			p.ProjectId.String(),
			p.RecordedAt.UTC(),
			int64(p.Duration),
			p.ServiceName,
			p.ProfileType,
			int64(p.SampleCount),
			p.TotalValue,
			p.ServerName,
			p.AppVersion,
			attributesJSON,
			p.StorageKey,
			p.TraceId,
			p.SpanId,
			nullableString(distributedTraceId),
			p.Unit,
			boolToInt(p.IsGauge),
		})
	}
	return insertRows(ctx, "profiles", profileColumns, rows)
}

func (r *profileRepository) FindGroupedByService(ctx context.Context, projectId uuid.UUID, from, to time.Time, page, pageSize int, orderBy, sortDirection, search string) ([]models.ProfileGroup, int64, error) {
	searchClause := ""
	countParams := lit.P{"project_id": projectId, "from": from.UTC(), "to": to.UTC()}
	if search != "" {
		searchClause = " AND STRPOS(lower(service_name), lower(:search)) > 0"
		countParams["search"] = search
	}

	countResult, err := lit.SelectSingleNamed[fbCountResult](db.TelemetryDB,
		`SELECT COUNT(*) AS count FROM (
			SELECT 1 FROM profiles
			WHERE project_id = :project_id AND recorded_at >= :from AND recorded_at <= :to`+searchClause+`
			GROUP BY service_name, profile_type) AS sub`, countParams)
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
	pageParams := lit.P{"project_id": projectId, "from": from.UTC(), "to": to.UTC(), "limit": pageSize, "offset": offset}
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

	query, args, err := parseNamed(
		fmt.Sprintf(`SELECT service_name, profile_type,
			CAST(FLOOR(EXTRACT(EPOCH FROM recorded_at) / %d) AS BIGINT) * %d AS bucket,
			CAST(SUM(total_value) AS DOUBLE PRECISION) AS v
		FROM profiles
		WHERE project_id = :project_id AND recorded_at >= :from AND recorded_at <= :to
		GROUP BY service_name, profile_type, bucket`, bucketSecs, bucketSecs),
		lit.P{"project_id": projectId, "from": from.UTC(), "to": to.UTC()})
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
		lit.P{"project_id": projectId, "service": service, "from": from.UTC(), "to": to.UTC()})
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

	params := lit.P{"project_id": projectId, "type": profileType, "service": service, "from": from.UTC(), "to": to.UTC()}
	filter := fireboltLabelFilter("", labelFilters, params)
	cohort := fireboltCohortFilter("", appVersion, serverName, params)

	query, args, err := parseNamed(
		fmt.Sprintf(`SELECT %s AS bucket,
			CAST(%s(value) AS DOUBLE PRECISION) AS agg_value
		FROM profiling_samples
		WHERE project_id = :project_id AND type = :type AND service_name = :service
			AND start_time >= :from AND start_time <= :to`+filter+cohort+`
		GROUP BY bucket ORDER BY bucket ASC`, timeBucketExpr("start_time", secs), agg),
		params)
	if err != nil {
		return nil, err
	}
	sqlRows, err := db.TelemetryDB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	points, err := scanProfileSeriesPoints(sqlRows)
	if err != nil {
		return nil, err
	}
	shared.NormalizeSeriesPoints(points, normalize, isGauge, intervalMinutes)
	return points, nil
}

func (r *profileRepository) GetFlameGraph(ctx context.Context, projectId uuid.UUID, service, profileType string, from, to time.Time, labelFilters map[string]string, isGauge bool, appVersion, serverName string) ([]models.ProfileStackValue, error) {
	params := lit.P{"project_id": projectId, "type": profileType, "service": service, "from": from.UTC(), "to": to.UTC()}
	bareFilter := fireboltLabelFilter("", labelFilters, params)
	aliasFilter := fireboltLabelFilter("s.", labelFilters, params)
	bareCohort := fireboltCohortFilter("", appVersion, serverName, params)
	aliasCohort := fireboltCohortFilter("s.", appVersion, serverName, params)

	var query string
	if isGauge {
		// MAX_BY picks the profile_id of the latest sample per server (one even on ties),
		// matching SQLite's bare-column MIN/MAX semantics under strict GROUP BY.
		query = `WITH latest AS (
			SELECT MAX_BY(profile_id, start_time) AS pid
			FROM profiling_samples
			WHERE project_id = :project_id AND type = :type AND service_name = :service
				AND start_time >= :from AND start_time <= :to` + bareFilter + bareCohort + `
			GROUP BY server_name)
		SELECT st.stack AS stack_json, CAST(SUM(s.value) AS BIGINT) AS v
		FROM profiling_samples s
		JOIN latest l ON l.pid = s.profile_id
		JOIN profiling_stacks st ON st.project_id = s.project_id AND st.service_name = s.service_name AND st.stack_hash = s.stack_hash
		WHERE s.project_id = :project_id AND s.type = :type AND s.service_name = :service
			AND s.start_time >= :from AND s.start_time <= :to` + aliasFilter + aliasCohort + `
		GROUP BY s.stack_hash, st.stack
		ORDER BY v DESC LIMIT 50000`
	} else {
		query = `SELECT st.stack AS stack_json, CAST(SUM(s.value) AS BIGINT) AS v
		FROM profiling_samples s
		JOIN profiling_stacks st ON st.project_id = s.project_id AND st.service_name = s.service_name AND st.stack_hash = s.stack_hash
		WHERE s.project_id = :project_id AND s.type = :type AND s.service_name = :service
			AND s.start_time >= :from AND s.start_time <= :to` + aliasFilter + aliasCohort + `
		GROUP BY s.stack_hash, st.stack
		ORDER BY v DESC LIMIT 50000`
	}

	parsed, args, err := parseNamed(query, params)
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
	labelExpr := jsonExtractExpr("labels", ":key")
	results, err := lit.SelectNamed[labelValueRow](db.TelemetryDB,
		fmt.Sprintf(`SELECT DISTINCT %s AS v
		FROM profiling_samples
		WHERE project_id = :project_id AND type = :type AND service_name = :service
			AND start_time >= :from AND start_time <= :to
			AND %s IS NOT NULL
			AND %s != ''
		ORDER BY v ASC
		LIMIT %d`, labelExpr, labelExpr, labelExpr, profiling.MaxLabelValuesPerKey),
		lit.P{"project_id": projectId, "type": profileType, "service": service, "key": jsonPointerEscape(key), "from": from.UTC(), "to": to.UTC()})
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

	params := lit.P{"project_id": projectId, "type": profileType, "service": service, "from": from.UTC(), "to": to.UTC(), "limit": limit}
	filter := fireboltLabelFilter("", labelFilters, params)
	cohort := fireboltCohortFilter("", appVersion, serverName, params)
	dimExpr, ok := fireboltDimExpr(dimension, params)
	if !ok {
		return nil, fmt.Errorf("unsupported breakdown dimension: %s", dimension)
	}

	topRows, err := lit.SelectNamed[profileDimValueRow](db.TelemetryDB,
		fmt.Sprintf(`SELECT %s AS k, CAST(%s(value) AS DOUBLE PRECISION) AS tv
		FROM profiling_samples
		WHERE project_id = :project_id AND type = :type AND service_name = :service
			AND start_time >= :from AND start_time <= :to`+filter+cohort+`
			AND %s != ''
		GROUP BY k ORDER BY tv DESC LIMIT :limit`, dimExpr, agg, dimExpr),
		params)
	if err != nil {
		return nil, err
	}

	seriesQuery := fmt.Sprintf(`SELECT %s AS bucket,
		CAST(%s(value) AS DOUBLE PRECISION) AS agg_value
	FROM profiling_samples
	WHERE project_id = :project_id AND type = :type AND service_name = :service
		AND start_time >= :from AND start_time <= :to`+filter+cohort+`
		AND %s = :dimval
	GROUP BY bucket ORDER BY bucket ASC`, timeBucketExpr("start_time", secs), agg, dimExpr)

	groups := make([]models.ProfileSeriesGroup, 0, len(topRows))
	for _, row := range topRows {
		if row.Key == nil {
			continue
		}
		key := *row.Key
		params["dimval"] = key
		parsed, args, err := parseNamed(seriesQuery, params)
		if err != nil {
			return nil, err
		}
		sqlRows, err := db.TelemetryDB.QueryContext(ctx, parsed, args...)
		if err != nil {
			return nil, err
		}
		points, err := scanProfileSeriesPoints(sqlRows)
		if err != nil {
			return nil, err
		}
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
		lit.P{"project_id": projectId, "service": service, "type": profileType, "from": from.UTC(), "to": to.UTC()})
	if err != nil {
		return nil, err
	}
	values := make([]string, 0, len(rows))
	for _, row := range rows {
		values = append(values, row.Value)
	}
	return values, nil
}

func scanProfileSeriesPoints(rows *sql.Rows) ([]models.TimeSeriesPoint, error) {
	defer rows.Close()
	points := make([]models.TimeSeriesPoint, 0)
	for rows.Next() {
		var bucket time.Time
		var value float64
		if err := rows.Scan(&bucket, &value); err != nil {
			return nil, err
		}
		points = append(points, models.TimeSeriesPoint{Timestamp: bucket, Value: value})
	}
	return points, rows.Err()
}

func fireboltCohortFilter(qualifier, appVersion, serverName string, params lit.P) string {
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

func fireboltDimExpr(dimension string, params lit.P) (string, bool) {
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
	params["dimpath"] = jsonPointerEscape(key)
	return jsonExtractExpr("labels", ":dimpath"), true
}

func fireboltLabelFilter(qualifier string, filters map[string]string, params lit.P) string {
	if len(filters) == 0 {
		return ""
	}
	clause := ""
	for i, k := range shared.SortedKeys(filters) {
		pathKey := fmt.Sprintf("lblpath_%d", i)
		valKey := fmt.Sprintf("lblval_%d", i)
		clause += " AND " + jsonExtractExpr(qualifier+"labels", ":"+pathKey) + " = :" + valKey
		params[pathKey] = jsonPointerEscape(k)
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
