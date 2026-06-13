//go:build pgch

package notifications

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/chdb"
	"github.com/tracewayapp/traceway/backend/app/models"
)

type EvalResult struct {
	Fired    bool
	Message  Message
	Messages []Message
}

type RuleEvaluator func(ctx context.Context, rule *models.NotificationRule, projectId uuid.UUID) (*EvalResult, error)

var polledEvaluators = map[string]RuleEvaluator{
	"error_rate_threshold":    evaluateErrorRateThreshold,
	"endpoint_p95_threshold":  evaluateEndpointP95Threshold,
	"endpoint_p99_threshold":  evaluateEndpointP99Threshold,
	"apdex_drop":              evaluateApdexDrop,
	"metric_threshold":        evaluateMetricThreshold,
	"no_data":                 evaluateNoData,
	"error_count_threshold":   evaluateErrorCountThreshold,
	"task_duration_threshold": evaluateTaskDurationThreshold,
	"task_failure_rate":       evaluateTaskFailureRate,
	"throughput_drop":         evaluateThroughputDrop,
	"endpoint_error_rate":     evaluateEndpointErrorRate,
	"impact_score_critical":   evaluateImpactScoreCritical,
	"impact_score_high":       evaluateImpactScoreHigh,
	"impact_score_medium":     evaluateImpactScoreMedium,
}

// --- Error Rate Threshold ---

type errorRateConfig struct {
	ThresholdPercent float64 `json:"thresholdPercent"`
	LookbackMinutes int     `json:"lookbackMinutes"`
	MinRequests     int     `json:"minRequests"`
}

func evaluateErrorRateThreshold(ctx context.Context, rule *models.NotificationRule, projectId uuid.UUID) (*EvalResult, error) {
	var cfg errorRateConfig
	if err := json.Unmarshal(rule.Config, &cfg); err != nil {
		return nil, fmt.Errorf("invalid error_rate_threshold config: %w", err)
	}
	if cfg.LookbackMinutes <= 0 {
		cfg.LookbackMinutes = 5
	}

	now := time.Now().UTC()
	from := now.Add(-time.Duration(cfg.LookbackMinutes) * time.Minute)

	var total, errors uint64
	err := chdb.Conn.QueryRow(ctx,
		"SELECT count() as total, countIf(status_code >= 500) as errors FROM endpoints WHERE project_id = ? AND recorded_at >= ? AND recorded_at <= ?",
		projectId, from, now).Scan(&total, &errors)
	if err != nil {
		return nil, err
	}

	if total < uint64(cfg.MinRequests) {
		return &EvalResult{Fired: false}, nil
	}

	rate := float64(errors) / float64(total) * 100
	if rate < cfg.ThresholdPercent {
		return &EvalResult{Fired: false}, nil
	}

	projectName := getProjectName(projectId)
	msg := buildErrorRateMessage(rate, cfg.ThresholdPercent, cfg.LookbackMinutes, projectName)
	return &EvalResult{Fired: true, Message: msg}, nil
}

// --- Endpoint P95 Threshold ---

type endpointLatencyConfig struct {
	Endpoint        string  `json:"endpoint"`
	ThresholdMs     float64 `json:"thresholdMs"`
	LookbackMinutes int     `json:"lookbackMinutes"`
}

func evaluateEndpointP95Threshold(ctx context.Context, rule *models.NotificationRule, projectId uuid.UUID) (*EvalResult, error) {
	var cfg endpointLatencyConfig
	if err := json.Unmarshal(rule.Config, &cfg); err != nil {
		return nil, fmt.Errorf("invalid endpoint_p95_threshold config: %w", err)
	}
	if cfg.LookbackMinutes <= 0 {
		cfg.LookbackMinutes = 5
	}

	now := time.Now().UTC()
	from := now.Add(-time.Duration(cfg.LookbackMinutes) * time.Minute)

	// duration is stored in nanoseconds, convert to milliseconds
	query := "SELECT quantile(0.95)(duration / 1000000) as p95 FROM endpoints WHERE project_id = ? AND recorded_at >= ? AND recorded_at <= ?"
	args := []interface{}{projectId, from, now}
	if cfg.Endpoint != "" && cfg.Endpoint != "*" {
		query += " AND endpoint = ?"
		args = append(args, cfg.Endpoint)
	}

	var p95 float64
	err := chdb.Conn.QueryRow(ctx, query, args...).Scan(&p95)
	if err != nil {
		return nil, err
	}

	if p95 < cfg.ThresholdMs {
		return &EvalResult{Fired: false}, nil
	}

	endpoint := cfg.Endpoint
	if endpoint == "" || endpoint == "*" {
		endpoint = "all endpoints"
	}
	projectName := getProjectName(projectId)
	msg := buildEndpointLatencyMessage("P95", p95, cfg.ThresholdMs, endpoint, cfg.LookbackMinutes, projectName)
	return &EvalResult{Fired: true, Message: msg}, nil
}

// --- Endpoint P99 Threshold ---

func evaluateEndpointP99Threshold(ctx context.Context, rule *models.NotificationRule, projectId uuid.UUID) (*EvalResult, error) {
	var cfg endpointLatencyConfig
	if err := json.Unmarshal(rule.Config, &cfg); err != nil {
		return nil, fmt.Errorf("invalid endpoint_p99_threshold config: %w", err)
	}
	if cfg.LookbackMinutes <= 0 {
		cfg.LookbackMinutes = 5
	}

	now := time.Now().UTC()
	from := now.Add(-time.Duration(cfg.LookbackMinutes) * time.Minute)

	query := "SELECT quantile(0.99)(duration / 1000000) as p99 FROM endpoints WHERE project_id = ? AND recorded_at >= ? AND recorded_at <= ?"
	args := []interface{}{projectId, from, now}
	if cfg.Endpoint != "" && cfg.Endpoint != "*" {
		query += " AND endpoint = ?"
		args = append(args, cfg.Endpoint)
	}

	var p99 float64
	err := chdb.Conn.QueryRow(ctx, query, args...).Scan(&p99)
	if err != nil {
		return nil, err
	}

	if p99 < cfg.ThresholdMs {
		return &EvalResult{Fired: false}, nil
	}

	endpoint := cfg.Endpoint
	if endpoint == "" || endpoint == "*" {
		endpoint = "all endpoints"
	}
	projectName := getProjectName(projectId)
	msg := buildEndpointLatencyMessage("P99", p99, cfg.ThresholdMs, endpoint, cfg.LookbackMinutes, projectName)
	return &EvalResult{Fired: true, Message: msg}, nil
}

// --- Apdex Drop ---

type apdexConfig struct {
	ThresholdApdex  float64 `json:"thresholdApdex"`
	LookbackMinutes int     `json:"lookbackMinutes"`
	MinRequests     int     `json:"minRequests"`
}

func evaluateApdexDrop(ctx context.Context, rule *models.NotificationRule, projectId uuid.UUID) (*EvalResult, error) {
	var cfg apdexConfig
	if err := json.Unmarshal(rule.Config, &cfg); err != nil {
		return nil, fmt.Errorf("invalid apdex_drop config: %w", err)
	}
	if cfg.LookbackMinutes <= 0 {
		cfg.LookbackMinutes = 15
	}

	now := time.Now().UTC()
	from := now.Add(-time.Duration(cfg.LookbackMinutes) * time.Minute)

	// Apdex thresholds: Good <= 750ms (750000000ns), Tolerable <= 1500ms (1500000000ns)
	var total, satisfied, tolerating uint64
	err := chdb.Conn.QueryRow(ctx,
		`SELECT count() as total,
			countIf(duration <= 750000000 AND status_code < 500) as satisfied,
			countIf(duration > 750000000 AND duration <= 1500000000 AND status_code < 500) as tolerating
		FROM endpoints WHERE project_id = ? AND recorded_at >= ? AND recorded_at <= ?`,
		projectId, from, now).Scan(&total, &satisfied, &tolerating)
	if err != nil {
		return nil, err
	}

	if total < uint64(cfg.MinRequests) {
		return &EvalResult{Fired: false}, nil
	}

	apdex := (float64(satisfied) + float64(tolerating)/2.0) / float64(total)
	if apdex >= cfg.ThresholdApdex {
		return &EvalResult{Fired: false}, nil
	}

	projectName := getProjectName(projectId)
	msg := buildApdexDropMessage(apdex, cfg.ThresholdApdex, int64(total), cfg.LookbackMinutes, projectName)
	return &EvalResult{Fired: true, Message: msg}, nil
}

// --- Metric Threshold ---

type metricThresholdConfig struct {
	MetricName      string  `json:"metricName"`
	Operator        string  `json:"operator"`
	ThresholdValue  float64 `json:"thresholdValue"`
	Aggregation     string  `json:"aggregation"`
	LookbackMinutes int     `json:"lookbackMinutes"`
}

func evaluateMetricThreshold(ctx context.Context, rule *models.NotificationRule, projectId uuid.UUID) (*EvalResult, error) {
	var cfg metricThresholdConfig
	if err := json.Unmarshal(rule.Config, &cfg); err != nil {
		return nil, fmt.Errorf("invalid metric_threshold config: %w", err)
	}
	if cfg.LookbackMinutes <= 0 {
		cfg.LookbackMinutes = 5
	}

	now := time.Now().UTC()
	from := now.Add(-time.Duration(cfg.LookbackMinutes) * time.Minute)

	selectExpr := "avg(value)"
	switch cfg.Aggregation {
	case "max":
		selectExpr = "max(value)"
	case "min":
		selectExpr = "min(value)"
	case "sum":
		selectExpr = "sum(value)"
	case "p95":
		selectExpr = "quantile(0.95)(value)"
	case "p99":
		selectExpr = "quantile(0.99)(value)"
	case "last":
		selectExpr = "argMax(value, recorded_at)"
	}

	query := fmt.Sprintf("SELECT count(), %s FROM metric_points WHERE project_id = ? AND name = ? AND recorded_at >= ? AND recorded_at <= ?", selectExpr)
	args := []interface{}{projectId, cfg.MetricName, from, now}

	var count uint64
	var value float64
	err := chdb.Conn.QueryRow(ctx, query, args...).Scan(&count, &value)
	if err != nil {
		return nil, err
	}
	if count == 0 {
		return &EvalResult{Fired: false}, nil
	}

	fired := false
	switch cfg.Operator {
	case "gt":
		fired = value > cfg.ThresholdValue
	case "gte":
		fired = value >= cfg.ThresholdValue
	case "lt":
		fired = value < cfg.ThresholdValue
	case "lte":
		fired = value <= cfg.ThresholdValue
	case "eq":
		fired = value == cfg.ThresholdValue
	}

	if !fired {
		return &EvalResult{Fired: false}, nil
	}

	projectName := getProjectName(projectId)
	msg := buildMetricThresholdMessage(cfg.MetricName, value, cfg.Operator, cfg.ThresholdValue, cfg.Aggregation, cfg.LookbackMinutes, projectName)
	return &EvalResult{Fired: true, Message: msg}, nil
}

// --- No Data ---

type noDataConfig struct {
	DataType       string `json:"dataType"`
	SilenceMinutes int    `json:"silenceMinutes"`
}

func evaluateNoData(ctx context.Context, rule *models.NotificationRule, projectId uuid.UUID) (*EvalResult, error) {
	var cfg noDataConfig
	if err := json.Unmarshal(rule.Config, &cfg); err != nil {
		return nil, fmt.Errorf("invalid no_data config: %w", err)
	}
	if cfg.SilenceMinutes <= 0 {
		cfg.SilenceMinutes = 10
	}

	threshold := time.Now().UTC().Add(-time.Duration(cfg.SilenceMinutes) * time.Minute)

	if cfg.DataType == "any" {
		tables := []string{"endpoints", "exception_stack_traces", "metric_points", "tasks"}
		for _, t := range tables {
			var maxTs time.Time
			err := chdb.Conn.QueryRow(ctx,
				fmt.Sprintf("SELECT max(recorded_at) FROM %s WHERE project_id = ?", t),
				projectId).Scan(&maxTs)
			if err == nil && maxTs.After(threshold) {
				return &EvalResult{Fired: false}, nil
			}
		}
		projectName := getProjectName(projectId)
		msg := buildNoDataMessage("any", cfg.SilenceMinutes, projectName)
		return &EvalResult{Fired: true, Message: msg}, nil
	}

	table := ""
	switch cfg.DataType {
	case "endpoints":
		table = "endpoints"
	case "exceptions":
		table = "exception_stack_traces"
	case "metrics":
		table = "metric_points"
	case "tasks":
		table = "tasks"
	default:
		return nil, fmt.Errorf("unknown data type: %s", cfg.DataType)
	}

	var maxTs time.Time
	err := chdb.Conn.QueryRow(ctx,
		fmt.Sprintf("SELECT max(recorded_at) FROM %s WHERE project_id = ?", table),
		projectId).Scan(&maxTs)
	if err != nil {
		return nil, err
	}

	if maxTs.After(threshold) {
		return &EvalResult{Fired: false}, nil
	}

	projectName := getProjectName(projectId)
	msg := buildNoDataMessage(cfg.DataType, cfg.SilenceMinutes, projectName)
	return &EvalResult{Fired: true, Message: msg}, nil
}

// --- Error Count Threshold ---

type errorCountConfig struct {
	ThresholdCount  int64 `json:"thresholdCount"`
	LookbackMinutes int   `json:"lookbackMinutes"`
}

func evaluateErrorCountThreshold(ctx context.Context, rule *models.NotificationRule, projectId uuid.UUID) (*EvalResult, error) {
	var cfg errorCountConfig
	if err := json.Unmarshal(rule.Config, &cfg); err != nil {
		return nil, fmt.Errorf("invalid error_count_threshold config: %w", err)
	}
	if cfg.LookbackMinutes <= 0 {
		cfg.LookbackMinutes = 60
	}

	now := time.Now().UTC()
	from := now.Add(-time.Duration(cfg.LookbackMinutes) * time.Minute)

	var count uint64
	err := chdb.Conn.QueryRow(ctx,
		"SELECT count() FROM exception_stack_traces WHERE project_id = ? AND recorded_at >= ? AND recorded_at <= ? AND is_message = 0",
		projectId, from, now).Scan(&count)
	if err != nil {
		return nil, err
	}

	if int64(count) < cfg.ThresholdCount {
		return &EvalResult{Fired: false}, nil
	}

	projectName := getProjectName(projectId)
	msg := buildErrorCountMessage(int64(count), cfg.ThresholdCount, cfg.LookbackMinutes, projectName)
	return &EvalResult{Fired: true, Message: msg}, nil
}

// --- Task Duration Threshold ---

type taskDurationConfig struct {
	TaskName        string  `json:"taskName"`
	ThresholdMs     float64 `json:"thresholdMs"`
	LookbackMinutes int     `json:"lookbackMinutes"`
}

func evaluateTaskDurationThreshold(ctx context.Context, rule *models.NotificationRule, projectId uuid.UUID) (*EvalResult, error) {
	var cfg taskDurationConfig
	if err := json.Unmarshal(rule.Config, &cfg); err != nil {
		return nil, fmt.Errorf("invalid task_duration_threshold config: %w", err)
	}
	if cfg.LookbackMinutes <= 0 {
		cfg.LookbackMinutes = 30
	}

	now := time.Now().UTC()
	from := now.Add(-time.Duration(cfg.LookbackMinutes) * time.Minute)

	// duration is stored in nanoseconds, convert to milliseconds
	query := "SELECT quantile(0.95)(duration / 1000000) as p95 FROM tasks WHERE project_id = ? AND recorded_at >= ? AND recorded_at <= ?"
	args := []interface{}{projectId, from, now}
	if cfg.TaskName != "" && cfg.TaskName != "*" {
		query += " AND task_name = ?"
		args = append(args, cfg.TaskName)
	}

	var p95 float64
	err := chdb.Conn.QueryRow(ctx, query, args...).Scan(&p95)
	if err != nil {
		return nil, err
	}

	if p95 < cfg.ThresholdMs {
		return &EvalResult{Fired: false}, nil
	}

	taskName := cfg.TaskName
	if taskName == "" || taskName == "*" {
		taskName = "all tasks"
	}
	projectName := getProjectName(projectId)
	msg := buildTaskDurationMessage(taskName, p95, cfg.ThresholdMs, cfg.LookbackMinutes, projectName)
	return &EvalResult{Fired: true, Message: msg}, nil
}

// --- Task Failure Rate ---

func countTaskExecutions(ctx context.Context, projectId uuid.UUID, taskName string, named bool, from, to time.Time) (int64, error) {
	query := "SELECT count() FROM tasks WHERE project_id = ? AND recorded_at >= ? AND recorded_at <= ?"
	args := []interface{}{projectId, from, to}
	if named {
		query += " AND task_name = ?"
		args = append(args, taskName)
	}

	var total uint64
	if err := chdb.Conn.QueryRow(ctx, query, args...).Scan(&total); err != nil {
		return 0, err
	}
	return int64(total), nil
}

func countFailedTaskExecutions(ctx context.Context, projectId uuid.UUID, taskName string, named bool, from, to time.Time) (int64, error) {
	query := "SELECT countDistinct(trace_id) FROM exception_stack_traces WHERE project_id = ? AND trace_type = 'task' AND recorded_at >= ? AND recorded_at <= ?" +
		" AND trace_id IN (SELECT id FROM tasks WHERE project_id = ? AND recorded_at >= ? AND recorded_at <= ?"
	args := []interface{}{projectId, from, to, projectId, from, to}
	if named {
		query += " AND task_name = ?"
		args = append(args, taskName)
	}
	query += ")"

	var failed uint64
	if err := chdb.Conn.QueryRow(ctx, query, args...).Scan(&failed); err != nil {
		return 0, err
	}
	return int64(failed), nil
}

// --- Throughput Drop ---

type throughputDropConfig struct {
	DropPercent           float64 `json:"dropPercent"`
	LookbackMinutes       int     `json:"lookbackMinutes"`
	BaselineWindowMinutes int     `json:"baselineWindowMinutes"`
}

func evaluateThroughputDrop(ctx context.Context, rule *models.NotificationRule, projectId uuid.UUID) (*EvalResult, error) {
	var cfg throughputDropConfig
	if err := json.Unmarshal(rule.Config, &cfg); err != nil {
		return nil, fmt.Errorf("invalid throughput_drop config: %w", err)
	}
	if cfg.LookbackMinutes <= 0 {
		cfg.LookbackMinutes = 15
	}
	if cfg.BaselineWindowMinutes <= 0 {
		cfg.BaselineWindowMinutes = 60
	}

	now := time.Now().UTC()
	lookbackFrom := now.Add(-time.Duration(cfg.LookbackMinutes) * time.Minute)
	baselineFrom := lookbackFrom.Add(-time.Duration(cfg.BaselineWindowMinutes) * time.Minute)

	var currentCount uint64
	err := chdb.Conn.QueryRow(ctx,
		"SELECT count() FROM endpoints WHERE project_id = ? AND recorded_at >= ? AND recorded_at <= ?",
		projectId, lookbackFrom, now).Scan(&currentCount)
	if err != nil {
		return nil, err
	}

	var baselineCount uint64
	err = chdb.Conn.QueryRow(ctx,
		"SELECT count() FROM endpoints WHERE project_id = ? AND recorded_at >= ? AND recorded_at <= ?",
		projectId, baselineFrom, lookbackFrom).Scan(&baselineCount)
	if err != nil {
		return nil, err
	}

	if baselineCount == 0 {
		return &EvalResult{Fired: false}, nil
	}

	normalizedBaseline := float64(baselineCount) * float64(cfg.LookbackMinutes) / float64(cfg.BaselineWindowMinutes)
	if normalizedBaseline == 0 {
		return &EvalResult{Fired: false}, nil
	}

	dropPercent := (1 - float64(currentCount)/normalizedBaseline) * 100
	if dropPercent < cfg.DropPercent {
		return &EvalResult{Fired: false}, nil
	}

	projectName := getProjectName(projectId)
	msg := buildThroughputDropMessage(dropPercent, int64(currentCount), normalizedBaseline, cfg.LookbackMinutes, projectName)
	return &EvalResult{Fired: true, Message: msg}, nil
}

// --- Endpoint Error Rate ---

type endpointErrorRateConfig struct {
	Endpoint         string  `json:"endpoint"`
	ThresholdPercent float64 `json:"thresholdPercent"`
	LookbackMinutes  int     `json:"lookbackMinutes"`
	MinRequests      int     `json:"minRequests"`
}

func evaluateEndpointErrorRate(ctx context.Context, rule *models.NotificationRule, projectId uuid.UUID) (*EvalResult, error) {
	var cfg endpointErrorRateConfig
	if err := json.Unmarshal(rule.Config, &cfg); err != nil {
		return nil, fmt.Errorf("invalid endpoint_error_rate config: %w", err)
	}
	if cfg.LookbackMinutes <= 0 {
		cfg.LookbackMinutes = 10
	}

	now := time.Now().UTC()
	from := now.Add(-time.Duration(cfg.LookbackMinutes) * time.Minute)

	var total, errors uint64
	err := chdb.Conn.QueryRow(ctx,
		"SELECT count() as total, countIf(status_code >= 500) as errors FROM endpoints WHERE project_id = ? AND endpoint = ? AND recorded_at >= ? AND recorded_at <= ?",
		projectId, cfg.Endpoint, from, now).Scan(&total, &errors)
	if err != nil {
		return nil, err
	}

	if total < uint64(cfg.MinRequests) {
		return &EvalResult{Fired: false}, nil
	}

	rate := float64(errors) / float64(total) * 100
	if rate < cfg.ThresholdPercent {
		return &EvalResult{Fired: false}, nil
	}

	projectName := getProjectName(projectId)
	msg := buildEndpointErrorRateMessage(cfg.Endpoint, rate, cfg.ThresholdPercent, projectName)
	return &EvalResult{Fired: true, Message: msg}, nil
}

// --- Impact Score ---

func computeImpactEndpoints(ctx context.Context, projectId uuid.UUID, minRequests int) ([]impactEndpointData, error) {
	now := time.Now().UTC()
	from := now.Add(-24 * time.Hour)

	query := `SELECT
		endpoint, total_count, p99_duration, offset_ms,
		satisfied_count, tolerating_count, bad_count, client_error_count,
		greatest(
			if(total_count > 0,
				1.0 - ((satisfied_count + tolerating_count * 0.5) / total_count), 0.0),
			multiIf(
				bad_count / total_count > 0.33, 0.75,
				bad_count / total_count > 0.20, 0.50,
				bad_count / total_count > 0.10, 0.25, 0.0),
			multiIf(
				toFloat64(p99_duration) - toFloat64(offset_ms) * 1000000 > 8000000000, 0.75,
				toFloat64(p99_duration) - toFloat64(offset_ms) * 1000000 > 6000000000, 0.50,
				toFloat64(p99_duration) - toFloat64(offset_ms) * 1000000 > 3000000000, 0.25, 0.0),
			if(endpoint != 'UNMATCHED' AND total_count > 10,
				multiIf(
					client_error_count / total_count > 0.50, 0.75,
					client_error_count / total_count > 0.25, 0.50, 0.0),
				0.0),
			multiIf(
				bad_count / total_count > 0.10 AND bad_count >= 500, 0.75,
				bad_count / total_count > 0.10 AND bad_count >= 50, 0.50,
				bad_count / total_count > 0.05 AND bad_count >= 2000, 0.75,
				bad_count / total_count > 0.05 AND bad_count >= 500, 0.50,
				bad_count / total_count > 0.05 AND bad_count >= 50, 0.25,
				bad_count / total_count > 0.01 AND bad_count >= 10000, 0.75,
				bad_count / total_count > 0.01 AND bad_count >= 2000, 0.50,
				bad_count / total_count > 0.01 AND bad_count >= 500, 0.25,
				0.0)
		) as impact
	FROM (
		SELECT
			endpoint,
			offset_ms,
			count() as total_count,
			quantile(0.99)(duration) as p99_duration,
			countIf(duration <= (750000000 + toInt64(offset_ms) * 1000000)
				AND status_code < 500) as satisfied_count,
			countIf(duration > (750000000 + toInt64(offset_ms) * 1000000)
				AND duration <= (1500000000 + toInt64(offset_ms) * 1000000)
				AND status_code < 500) as tolerating_count,
			countIf(duration > (1500000000 + toInt64(offset_ms) * 1000000)
				OR status_code >= 500) as bad_count,
			countIf(status_code >= 400 AND status_code < 500) as client_error_count
		FROM (
			SELECT e.endpoint, e.duration, e.status_code, e.recorded_at,
				   s.offset_ms as offset_ms
			FROM endpoints e
			LEFT JOIN (SELECT * FROM slow_endpoints FINAL) AS s
				ON e.endpoint = s.endpoint AND e.project_id = s.project_id
			WHERE e.project_id = ? AND e.recorded_at >= ? AND e.recorded_at <= ? AND e.is_stream = 0
		)
		GROUP BY endpoint, offset_ms
	)
	WHERE impact >= ? AND total_count >= ?`

	rows, err := chdb.Conn.Query(ctx, query, projectId, from, now, minImpactThreshold, uint64(minRequests))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []impactEndpointData
	for rows.Next() {
		var ep string
		var totalCount, satisfied, tolerating, bad, clientErrors uint64
		var p99 float64
		var offsetMs uint32
		var impact float64
		if err := rows.Scan(&ep, &totalCount, &p99, &offsetMs,
			&satisfied, &tolerating, &bad, &clientErrors, &impact); err != nil {
			return nil, err
		}
		result = append(result, impactEndpointData{
			endpoint:     ep,
			impact:       impact,
			totalCount:   totalCount,
			p99:          p99,
			offsetMs:     offsetMs,
			satisfied:    satisfied,
			tolerating:   tolerating,
			bad:          bad,
			clientErrors: clientErrors,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}
