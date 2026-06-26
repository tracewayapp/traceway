//go:build pgch

package repositories

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/chdb"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/profiling"
)

type profileRepository struct{}

const profileStackDedup = `(SELECT project_id, service_name, stack_hash, any(stack) AS stack FROM profiling_stacks WHERE project_id = ? AND service_name = ? GROUP BY project_id, service_name, stack_hash)`

func (r *profileRepository) InsertStacksAsync(ctx context.Context, stacks []models.ProfileStack) error {
	if len(stacks) == 0 {
		return nil
	}
	batch, err := chdb.Conn.PrepareBatch(chdb.BatchCtx(),
		"INSERT INTO profiling_stacks (project_id, service_name, stack_hash, stack, last_seen)")
	if err != nil {
		return err
	}
	for _, s := range stacks {
		if err := batch.Append(s.ProjectId, s.ServiceName, s.StackHash, s.Stack, s.LastSeen); err != nil {
			return err
		}
	}
	return batch.Send()
}

func (r *profileRepository) InsertSamplesAsync(ctx context.Context, samples []models.ProfileSample) error {
	if len(samples) == 0 {
		return nil
	}
	batch, err := chdb.Conn.PrepareBatch(chdb.BatchCtx(),
		"INSERT INTO profiling_samples (project_id, profile_id, service_name, type, unit, is_gauge, start_time, end_time, stack_hash, value, labels, server_name, app_version, trace_id, span_id)")
	if err != nil {
		return err
	}
	for _, s := range samples {
		labels := s.Labels
		if labels == nil {
			labels = map[string]string{}
		}
		if err := batch.Append(
			s.ProjectId, s.ProfileId, s.ServiceName, s.Type, s.Unit, boolToUInt8(s.IsGauge), s.Start, s.End,
			s.StackHash, s.Value, labels, s.ServerName, s.AppVersion, s.TraceId, s.SpanId,
		); err != nil {
			return err
		}
	}
	return batch.Send()
}

func boolToUInt8(b bool) uint8 {
	if b {
		return 1
	}
	return 0
}

func (r *profileRepository) InsertProfilesAsync(ctx context.Context, profiles []models.Profile) error {
	if len(profiles) == 0 {
		return nil
	}
	batch, err := chdb.Conn.PrepareBatch(chdb.BatchCtx(),
		"INSERT INTO profiles (id, project_id, recorded_at, duration, service_name, profile_type, unit, is_gauge, sample_count, total_value, server_name, app_version, attributes, storage_key, trace_id, span_id, distributed_trace_id)")
	if err != nil {
		return err
	}
	for _, p := range profiles {
		attributesJSON := "{}"
		if len(p.Attributes) != 0 {
			if b, err := json.Marshal(p.Attributes); err == nil {
				attributesJSON = string(b)
			}
		}
		if err := batch.Append(
			p.Id, p.ProjectId, p.RecordedAt, int64(p.Duration), p.ServiceName, p.ProfileType, p.Unit, boolToUInt8(p.IsGauge),
			p.SampleCount, p.TotalValue, p.ServerName, p.AppVersion, attributesJSON, p.StorageKey,
			p.TraceId, p.SpanId, p.DistributedTraceId,
		); err != nil {
			return err
		}
	}
	return batch.Send()
}

func (r *profileRepository) FindGroupedByService(ctx context.Context, projectId uuid.UUID, from, to time.Time, page, pageSize int, orderBy, sortDirection, search string) ([]models.ProfileGroup, int64, error) {
	searchClause := ""
	if search != "" {
		searchClause = " AND positionCaseInsensitive(service_name, ?) > 0"
	}

	countArgs := []interface{}{projectId, from, to}
	if search != "" {
		countArgs = append(countArgs, search)
	}
	var count uint64
	err := chdb.Conn.QueryRow(ctx,
		`SELECT count() FROM (SELECT service_name, profile_type FROM profiles
			WHERE project_id = ? AND recorded_at >= ? AND recorded_at <= ?`+searchClause+`
			GROUP BY service_name, profile_type)`, countArgs...).Scan(&count)
	if err != nil {
		return nil, 0, err
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

	query := fmt.Sprintf(`SELECT service_name, profile_type,
			any(unit) AS unit,
			max(is_gauge) AS is_gauge,
			count() AS profile_count,
			sum(sample_count) AS sample_count,
			sum(total_value) AS total_value,
			max(recorded_at) AS last_seen
		FROM profiles
		WHERE project_id = ? AND recorded_at >= ? AND recorded_at <= ?`+searchClause+`
		GROUP BY service_name, profile_type
		ORDER BY %s %s
		LIMIT ? OFFSET ?`, orderExpr, sortDir)

	pageArgs := []interface{}{projectId, from, to}
	if search != "" {
		pageArgs = append(pageArgs, search)
	}
	pageArgs = append(pageArgs, pageSize, offset)

	rows, err := chdb.Conn.Query(ctx, query, pageArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var groups []models.ProfileGroup
	for rows.Next() {
		var g models.ProfileGroup
		var profileCount, sampleCount uint64
		var isGauge uint8
		if err := rows.Scan(&g.ServiceName, &g.Type, &g.Unit, &isGauge, &profileCount, &sampleCount, &g.TotalValue, &g.LastSeen); err != nil {
			return nil, 0, err
		}
		g.IsGauge = isGauge != 0
		g.ProfileCount = int64(profileCount)
		g.SampleCount = int64(sampleCount)
		groups = append(groups, g)
	}

	if err := r.fillSparklines(ctx, projectId, from, to, groups); err != nil {
		return nil, 0, err
	}
	return groups, int64(count), nil
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

	rows, err := chdb.Conn.Query(ctx, `SELECT service_name, profile_type,
			toStartOfInterval(recorded_at, INTERVAL ? SECOND) AS bucket,
			toFloat64(sum(total_value)) AS v
		FROM profiles
		WHERE project_id = ? AND recorded_at >= ? AND recorded_at <= ?
		GROUP BY service_name, profile_type, bucket`, bucketSecs, projectId, from, to)
	if err != nil {
		return err
	}
	defer rows.Close()

	acc := map[string][]float64{}
	for rows.Next() {
		var service, ptype string
		var bucket time.Time
		var v float64
		if err := rows.Scan(&service, &ptype, &bucket, &v); err != nil {
			return err
		}
		idx := int((bucket.Unix() - alignedFrom) / int64(bucketSecs))
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
	rows, err := chdb.Conn.Query(ctx,
		`SELECT type, any(unit) AS unit, max(is_gauge) AS is_gauge
		FROM profiling_samples
		WHERE project_id = ? AND service_name = ? AND start_time >= ? AND start_time <= ?
		GROUP BY type
		ORDER BY type ASC`, projectId, service, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.ProfileTypeInfo
	for rows.Next() {
		var info models.ProfileTypeInfo
		var isGauge uint8
		if err := rows.Scan(&info.Type, &info.Unit, &isGauge); err != nil {
			return nil, err
		}
		info.IsGauge = isGauge != 0
		out = append(out, info)
	}
	return out, nil
}

func (r *profileRepository) GetSeries(ctx context.Context, projectId uuid.UUID, service, profileType string, from, to time.Time, intervalMinutes int, isGauge bool, labelFilters map[string]string, appVersion, serverName string, normalize bool) ([]models.TimeSeriesPoint, error) {
	if intervalMinutes < 1 {
		intervalMinutes = 1
	}
	agg := "sum"
	if isGauge {
		agg = "avg"
	}

	filter, filterArgs := chLabelFilter("", labelFilters)
	cohort, cohortArgs := chCohortFilter("", appVersion, serverName)

	query := fmt.Sprintf(`SELECT toStartOfInterval(start_time, INTERVAL ? MINUTE) AS bucket,
			toFloat64(%s(value)) AS agg_value
		FROM profiling_samples
		WHERE project_id = ? AND type = ? AND service_name = ? AND start_time >= ? AND start_time <= ?`+filter+cohort+`
		GROUP BY bucket ORDER BY bucket ASC`, agg)

	args := []interface{}{intervalMinutes, projectId, profileType, service, from, to}
	args = append(args, filterArgs...)
	args = append(args, cohortArgs...)

	rows, err := chdb.Conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var points []models.TimeSeriesPoint
	for rows.Next() {
		var p models.TimeSeriesPoint
		if err := rows.Scan(&p.Timestamp, &p.Value); err != nil {
			return nil, err
		}
		points = append(points, p)
	}
	normalizeSeriesPoints(points, normalize, isGauge, intervalMinutes)
	return points, nil
}

func (r *profileRepository) GetFlameGraph(ctx context.Context, projectId uuid.UUID, service, profileType string, from, to time.Time, labelFilters map[string]string, isGauge bool, appVersion, serverName string) ([]models.ProfileStackValue, error) {
	bareFilter, bareArgs := chLabelFilter("", labelFilters)
	aliasFilter, aliasArgs := chLabelFilter("s.", labelFilters)
	bareCohort, bareCohortArgs := chCohortFilter("", appVersion, serverName)
	aliasCohort, aliasCohortArgs := chCohortFilter("s.", appVersion, serverName)

	var query string
	var args []interface{}
	if isGauge {
		query = `WITH latest AS (
			SELECT argMax(profile_id, start_time) AS pid FROM profiling_samples
			WHERE project_id = ? AND type = ? AND service_name = ? AND start_time >= ? AND start_time <= ?` + bareFilter + bareCohort + `
			GROUP BY server_name)
		SELECT st.stack, sum(s.value) AS v
		FROM profiling_samples s
		INNER JOIN latest l ON l.pid = s.profile_id
		INNER JOIN ` + profileStackDedup + ` st ON st.project_id = s.project_id AND st.service_name = s.service_name AND st.stack_hash = s.stack_hash
		WHERE s.project_id = ? AND s.type = ? AND s.service_name = ? AND s.start_time >= ? AND s.start_time <= ?` + aliasFilter + aliasCohort + `
		GROUP BY s.stack_hash, st.stack
		ORDER BY v DESC LIMIT 50000`
		args = append(args, projectId, profileType, service, from, to)
		args = append(args, bareArgs...)
		args = append(args, bareCohortArgs...)
		args = append(args, projectId, service)
		args = append(args, projectId, profileType, service, from, to)
		args = append(args, aliasArgs...)
		args = append(args, aliasCohortArgs...)
	} else {
		query = `SELECT st.stack, sum(s.value) AS v
		FROM profiling_samples s
		INNER JOIN ` + profileStackDedup + ` st ON st.project_id = s.project_id AND st.service_name = s.service_name AND st.stack_hash = s.stack_hash
		WHERE s.project_id = ? AND s.type = ? AND s.service_name = ? AND s.start_time >= ? AND s.start_time <= ?` + aliasFilter + aliasCohort + `
		GROUP BY s.stack_hash, st.stack
		ORDER BY v DESC LIMIT 50000`
		args = append(args, projectId, service)
		args = append(args, projectId, profileType, service, from, to)
		args = append(args, aliasArgs...)
		args = append(args, aliasCohortArgs...)
	}

	rows, err := chdb.Conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.ProfileStackValue
	for rows.Next() {
		var frames []string
		var value int64
		if err := rows.Scan(&frames, &value); err != nil {
			return nil, err
		}
		out = append(out, models.ProfileStackValue{Stack: frames, Value: value})
	}
	return out, nil
}

func (r *profileRepository) distinctLabelValues(ctx context.Context, projectId uuid.UUID, service, profileType, key string, from, to time.Time) ([]string, error) {
	rows, err := chdb.Conn.Query(ctx,
		fmt.Sprintf(`SELECT DISTINCT labels[?] AS v
		FROM profiling_samples
		WHERE project_id = ? AND type = ? AND service_name = ? AND start_time >= ? AND start_time <= ?
			AND labels[?] != ''
		ORDER BY v ASC
		LIMIT %d`, profiling.MaxLabelValuesPerKey),
		key, projectId, profileType, service, from, to, key)
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

func (r *profileRepository) GetSeriesBreakdown(ctx context.Context, projectId uuid.UUID, service, profileType string, from, to time.Time, intervalMinutes int, isGauge bool, labelFilters map[string]string, appVersion, serverName string, normalize bool, dimension string, limit int) ([]models.ProfileSeriesGroup, error) {
	if intervalMinutes < 1 {
		intervalMinutes = 1
	}
	limit = clampBreakdownLimit(limit)
	agg := "sum"
	if isGauge {
		agg = "avg"
	}

	filter, filterArgs := chLabelFilter("", labelFilters)
	cohort, cohortArgs := chCohortFilter("", appVersion, serverName)
	dimExpr, dimArgs, ok := chDimExpr(dimension)
	if !ok {
		return nil, fmt.Errorf("unsupported breakdown dimension: %s", dimension)
	}

	topQuery := fmt.Sprintf(`SELECT %s AS k, toFloat64(%s(value)) AS tv
		FROM profiling_samples
		WHERE project_id = ? AND type = ? AND service_name = ? AND start_time >= ? AND start_time <= ?`+filter+cohort+`
			AND %s != ''
		GROUP BY k ORDER BY tv DESC LIMIT ?`, dimExpr, agg, dimExpr)

	var topArgs []interface{}
	topArgs = append(topArgs, dimArgs...)
	topArgs = append(topArgs, projectId, profileType, service, from, to)
	topArgs = append(topArgs, filterArgs...)
	topArgs = append(topArgs, cohortArgs...)
	topArgs = append(topArgs, dimArgs...)
	topArgs = append(topArgs, limit)

	rows, err := chdb.Conn.Query(ctx, topQuery, topArgs...)
	if err != nil {
		return nil, err
	}
	var keys []string
	for rows.Next() {
		var k string
		var tv float64
		if err := rows.Scan(&k, &tv); err != nil {
			rows.Close()
			return nil, err
		}
		keys = append(keys, k)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	seriesQuery := fmt.Sprintf(`SELECT toStartOfInterval(start_time, INTERVAL ? MINUTE) AS bucket,
			toFloat64(%s(value)) AS agg_value
		FROM profiling_samples
		WHERE project_id = ? AND type = ? AND service_name = ? AND start_time >= ? AND start_time <= ?`+filter+cohort+`
			AND %s = ?
		GROUP BY bucket ORDER BY bucket ASC`, agg, dimExpr)

	groups := make([]models.ProfileSeriesGroup, 0, len(keys))
	for _, key := range keys {
		var seriesArgs []interface{}
		seriesArgs = append(seriesArgs, intervalMinutes, projectId, profileType, service, from, to)
		seriesArgs = append(seriesArgs, filterArgs...)
		seriesArgs = append(seriesArgs, cohortArgs...)
		seriesArgs = append(seriesArgs, dimArgs...)
		seriesArgs = append(seriesArgs, key)

		srows, err := chdb.Conn.Query(ctx, seriesQuery, seriesArgs...)
		if err != nil {
			return nil, err
		}
		var points []models.TimeSeriesPoint
		for srows.Next() {
			var p models.TimeSeriesPoint
			if err := srows.Scan(&p.Timestamp, &p.Value); err != nil {
				srows.Close()
				return nil, err
			}
			points = append(points, p)
		}
		srows.Close()
		if err := srows.Err(); err != nil {
			return nil, err
		}
		normalizeSeriesPoints(points, normalize, isGauge, intervalMinutes)
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
	rows, err := chdb.Conn.Query(ctx,
		fmt.Sprintf(`SELECT DISTINCT %s AS v
		FROM profiling_samples
		WHERE project_id = ? AND service_name = ? AND type = ? AND start_time >= ? AND start_time <= ? AND %s != ''
		ORDER BY %s ASC LIMIT 100`, column, column, column),
		projectId, service, profileType, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	values := make([]string, 0)
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		values = append(values, v)
	}
	return values, rows.Err()
}

func chCohortFilter(qualifier, appVersion, serverName string) (string, []interface{}) {
	clause := ""
	var args []interface{}
	if appVersion != "" {
		clause += fmt.Sprintf(" AND %sapp_version = ?", qualifier)
		args = append(args, appVersion)
	}
	if serverName != "" {
		clause += fmt.Sprintf(" AND %sserver_name = ?", qualifier)
		args = append(args, serverName)
	}
	return clause, args
}

func chDimExpr(dimension string) (string, []interface{}, bool) {
	switch dimension {
	case "app_version":
		return "app_version", nil, true
	case "server_name":
		return "server_name", nil, true
	}
	key, ok := breakdownLabelKey(dimension)
	if !ok || key == "" {
		return "", nil, false
	}
	return "labels[?]", []interface{}{key}, true
}

func chLabelFilter(qualifier string, filters map[string]string) (string, []interface{}) {
	if len(filters) == 0 {
		return "", nil
	}
	keys := make([]string, 0, len(filters))
	for k := range filters {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	clause := ""
	var args []interface{}
	for _, k := range keys {
		clause += fmt.Sprintf(" AND %slabels[?] = ?", qualifier)
		args = append(args, k, filters[k])
	}
	return clause, args
}

var ProfileRepository = profileRepository{}
