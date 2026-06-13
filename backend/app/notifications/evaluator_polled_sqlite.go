//go:build !pgch

package notifications

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories"
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

	var total, errors int64
	err := db.TelemetryDB.QueryRowContext(ctx,
		"SELECT COUNT(*) as total, COALESCE(SUM(CASE WHEN status_code >= 500 THEN 1 ELSE 0 END), 0) as errors FROM endpoints WHERE project_id = ? AND recorded_at >= ? AND recorded_at <= ?",
		projectId.String(), from.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano)).Scan(&total, &errors)
	if err != nil {
		return nil, err
	}

	if total < int64(cfg.MinRequests) {
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

	p95 := queryPercentile(ctx, projectId, cfg.Endpoint, from, now, 0.95)

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

	p99 := queryPercentile(ctx, projectId, cfg.Endpoint, from, now, 0.99)

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

func queryPercentile(ctx context.Context, projectId uuid.UUID, endpoint string, from, to time.Time, pct float64) float64 {
	query := "SELECT duration FROM endpoints WHERE project_id = ? AND recorded_at >= ? AND recorded_at <= ?"
	args := []interface{}{projectId.String(), from.Format(time.RFC3339Nano), to.Format(time.RFC3339Nano)}
	if endpoint != "" && endpoint != "*" {
		query += " AND endpoint = ?"
		args = append(args, endpoint)
	}
	query += " ORDER BY duration ASC"

	rows, err := db.TelemetryDB.QueryContext(ctx, query, args...)
	if err != nil {
		return 0
	}
	defer rows.Close()

	var durations []float64
	for rows.Next() {
		var d int64
		if err := rows.Scan(&d); err != nil {
			continue
		}
		durations = append(durations, float64(d)/1000000) // ns to ms
	}

	return computePercentile(durations, pct)
}

func computePercentile(sorted []float64, p float64) float64 {
	n := len(sorted)
	if n == 0 {
		return 0
	}
	if n == 1 {
		return sorted[0]
	}
	idx := p * float64(n-1)
	lower := int(idx)
	frac := idx - float64(lower)
	if lower+1 >= n {
		return sorted[lower]
	}
	return sorted[lower]*(1-frac) + sorted[lower+1]*frac
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

	var total, satisfied, tolerating int64
	err := db.TelemetryDB.QueryRowContext(ctx,
		`SELECT COUNT(*) as total,
			COALESCE(SUM(CASE WHEN duration <= 750000000 AND status_code < 500 THEN 1 ELSE 0 END), 0) as satisfied,
			COALESCE(SUM(CASE WHEN duration > 750000000 AND duration <= 1500000000 AND status_code < 500 THEN 1 ELSE 0 END), 0) as tolerating
		FROM endpoints WHERE project_id = ? AND recorded_at >= ? AND recorded_at <= ?`,
		projectId.String(), from.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano)).Scan(&total, &satisfied, &tolerating)
	if err != nil {
		return nil, err
	}

	if total < int64(cfg.MinRequests) {
		return &EvalResult{Fired: false}, nil
	}

	apdex := (float64(satisfied) + float64(tolerating)/2.0) / float64(total)
	if apdex >= cfg.ThresholdApdex {
		return &EvalResult{Fired: false}, nil
	}

	projectName := getProjectName(projectId)
	msg := buildApdexDropMessage(apdex, cfg.ThresholdApdex, total, cfg.LookbackMinutes, projectName)
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

	var value float64
	var err error

	switch cfg.Aggregation {
	case "last":
		err = db.TelemetryDB.QueryRowContext(ctx,
			"SELECT value FROM metric_points WHERE project_id = ? AND name = ? AND recorded_at >= ? AND recorded_at <= ? ORDER BY recorded_at DESC LIMIT 1",
			projectId.String(), cfg.MetricName, from.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano)).Scan(&value)
		if err == sql.ErrNoRows {
			return &EvalResult{Fired: false}, nil
		}
		if err != nil {
			return nil, err
		}
	case "p95", "p99":
		pct := 0.95
		if cfg.Aggregation == "p99" {
			pct = 0.99
		}
		rows, qErr := db.TelemetryDB.QueryContext(ctx,
			"SELECT value FROM metric_points WHERE project_id = ? AND name = ? AND recorded_at >= ? AND recorded_at <= ? ORDER BY value ASC",
			projectId.String(), cfg.MetricName, from.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano))
		if qErr != nil {
			return nil, qErr
		}
		defer rows.Close()
		var vals []float64
		for rows.Next() {
			var v float64
			if err := rows.Scan(&v); err != nil {
				continue
			}
			vals = append(vals, v)
		}
		if len(vals) == 0 {
			return &EvalResult{Fired: false}, nil
		}
		value = computePercentile(vals, pct)
	default:
		aggFunc := "avg"
		switch cfg.Aggregation {
		case "max":
			aggFunc = "max"
		case "min":
			aggFunc = "min"
		case "sum":
			aggFunc = "sum"
		}
		query := fmt.Sprintf("SELECT COUNT(value), COALESCE(%s(value), 0) FROM metric_points WHERE project_id = ? AND name = ? AND recorded_at >= ? AND recorded_at <= ?", aggFunc)
		var count int64
		err = db.TelemetryDB.QueryRowContext(ctx, query, projectId.String(), cfg.MetricName, from.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano)).Scan(&count, &value)
		if err != nil {
			return nil, err
		}
		if count == 0 {
			return &EvalResult{Fired: false}, nil
		}
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
	pid := projectId.String()

	if cfg.DataType == "any" {
		tables := []string{"endpoints", "exception_stack_traces", "metric_points", "tasks"}
		for _, t := range tables {
			var maxTs string
			err := db.TelemetryDB.QueryRowContext(ctx,
				fmt.Sprintf("SELECT COALESCE(MAX(recorded_at), '') FROM %s WHERE project_id = ?", t),
				pid).Scan(&maxTs)
			if err == nil && maxTs != "" {
				if parsed, pErr := time.Parse(time.RFC3339Nano, maxTs); pErr == nil && parsed.After(threshold) {
					return &EvalResult{Fired: false}, nil
				}
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

	var maxTs string
	err := db.TelemetryDB.QueryRowContext(ctx,
		fmt.Sprintf("SELECT COALESCE(MAX(recorded_at), '') FROM %s WHERE project_id = ?", table),
		pid).Scan(&maxTs)
	if err != nil {
		return nil, err
	}

	if maxTs != "" {
		if parsed, pErr := time.Parse(time.RFC3339Nano, maxTs); pErr == nil && parsed.After(threshold) {
			return &EvalResult{Fired: false}, nil
		}
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

	var count int64
	err := db.TelemetryDB.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM exception_stack_traces WHERE project_id = ? AND recorded_at >= ? AND recorded_at <= ? AND is_message = 0",
		projectId.String(), from.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano)).Scan(&count)
	if err != nil {
		return nil, err
	}

	if count < cfg.ThresholdCount {
		return &EvalResult{Fired: false}, nil
	}

	projectName := getProjectName(projectId)
	msg := buildErrorCountMessage(count, cfg.ThresholdCount, cfg.LookbackMinutes, projectName)
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

	query := "SELECT duration FROM tasks WHERE project_id = ? AND recorded_at >= ? AND recorded_at <= ?"
	args := []interface{}{projectId.String(), from.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano)}
	if cfg.TaskName != "" && cfg.TaskName != "*" {
		query += " AND task_name = ?"
		args = append(args, cfg.TaskName)
	}
	query += " ORDER BY duration ASC"

	rows, err := db.TelemetryDB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var durations []float64
	for rows.Next() {
		var d int64
		if err := rows.Scan(&d); err != nil {
			continue
		}
		durations = append(durations, float64(d)/1000000) // ns to ms
	}

	p95 := computePercentile(durations, 0.95)

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
	query := "SELECT COUNT(*) FROM tasks WHERE project_id = ? AND recorded_at >= ? AND recorded_at <= ?"
	args := []interface{}{projectId.String(), from.Format(time.RFC3339Nano), to.Format(time.RFC3339Nano)}
	if named {
		query += " AND task_name = ?"
		args = append(args, taskName)
	}

	var total int64
	if err := db.TelemetryDB.QueryRowContext(ctx, query, args...).Scan(&total); err != nil {
		return 0, err
	}
	return total, nil
}

func countFailedTaskExecutions(ctx context.Context, projectId uuid.UUID, taskName string, named bool, from, to time.Time) (int64, error) {
	pid := projectId.String()
	fromStr := from.Format(time.RFC3339Nano)
	toStr := to.Format(time.RFC3339Nano)

	query := "SELECT COUNT(DISTINCT trace_id) FROM exception_stack_traces WHERE project_id = ? AND trace_type = 'task' AND recorded_at >= ? AND recorded_at <= ?" +
		" AND trace_id IN (SELECT id FROM tasks WHERE project_id = ? AND recorded_at >= ? AND recorded_at <= ?"
	args := []interface{}{pid, fromStr, toStr, pid, fromStr, toStr}
	if named {
		query += " AND task_name = ?"
		args = append(args, taskName)
	}
	query += ")"

	var failed int64
	if err := db.TelemetryDB.QueryRowContext(ctx, query, args...).Scan(&failed); err != nil {
		return 0, err
	}
	return failed, nil
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
	pid := projectId.String()

	var currentCount int64
	err := db.TelemetryDB.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM endpoints WHERE project_id = ? AND recorded_at >= ? AND recorded_at <= ?",
		pid, lookbackFrom.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano)).Scan(&currentCount)
	if err != nil {
		return nil, err
	}

	var baselineCount int64
	err = db.TelemetryDB.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM endpoints WHERE project_id = ? AND recorded_at >= ? AND recorded_at <= ?",
		pid, baselineFrom.Format(time.RFC3339Nano), lookbackFrom.Format(time.RFC3339Nano)).Scan(&baselineCount)
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
	msg := buildThroughputDropMessage(dropPercent, currentCount, normalizedBaseline, cfg.LookbackMinutes, projectName)
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
	pid := projectId.String()

	var total, errors int64
	err := db.TelemetryDB.QueryRowContext(ctx,
		"SELECT COUNT(*) as total, COALESCE(SUM(CASE WHEN status_code >= 500 THEN 1 ELSE 0 END), 0) as errors FROM endpoints WHERE project_id = ? AND endpoint = ? AND recorded_at >= ? AND recorded_at <= ?",
		pid, cfg.Endpoint, from.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano)).Scan(&total, &errors)
	if err != nil {
		return nil, err
	}

	if total < int64(cfg.MinRequests) {
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
	pid := projectId.String()
	fromStr := from.Format(time.RFC3339Nano)
	nowStr := now.Format(time.RFC3339Nano)

	rows, err := db.TelemetryDB.QueryContext(ctx, `SELECT
		e.endpoint,
		COUNT(*) as total_count,
		COALESCE(s.offset_ms, 0) as offset_ms,
		COALESCE(SUM(CASE WHEN e.duration <= (750000000 + COALESCE(s.offset_ms, 0) * 1000000) AND e.status_code < 500 THEN 1 ELSE 0 END), 0) as satisfied_count,
		COALESCE(SUM(CASE WHEN e.duration > (750000000 + COALESCE(s.offset_ms, 0) * 1000000) AND e.duration <= (1500000000 + COALESCE(s.offset_ms, 0) * 1000000) AND e.status_code < 500 THEN 1 ELSE 0 END), 0) as tolerating_count,
		COALESCE(SUM(CASE WHEN e.duration > (1500000000 + COALESCE(s.offset_ms, 0) * 1000000) OR e.status_code >= 500 THEN 1 ELSE 0 END), 0) as bad_count,
		COALESCE(SUM(CASE WHEN e.status_code >= 400 AND e.status_code < 500 THEN 1 ELSE 0 END), 0) as client_error_count
	FROM endpoints e
	LEFT JOIN slow_endpoints s ON e.endpoint = s.endpoint AND e.project_id = s.project_id
	WHERE e.project_id = ? AND e.recorded_at >= ? AND e.recorded_at <= ? AND e.is_stream = 0
	GROUP BY e.endpoint, COALESCE(s.offset_ms, 0)`,
		pid, fromStr, nowStr)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var candidates []impactEndpointData
	for rows.Next() {
		var ep string
		var totalCount, satisfied, tolerating, bad, clientErrors int64
		var offsetMs int64
		if err := rows.Scan(&ep, &totalCount, &offsetMs, &satisfied, &tolerating, &bad, &clientErrors); err != nil {
			return nil, err
		}

		if totalCount < int64(minRequests) {
			continue
		}

		candidates = append(candidates, impactEndpointData{
			endpoint:     ep,
			totalCount:   uint64(totalCount),
			offsetMs:     uint32(offsetMs),
			satisfied:    uint64(satisfied),
			tolerating:   uint64(tolerating),
			bad:          uint64(bad),
			clientErrors: uint64(clientErrors),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if len(candidates) == 0 {
		return nil, nil
	}

	p99s := make(map[string]float64, len(candidates))
	pRows, err := db.TelemetryDB.QueryContext(ctx, `SELECT endpoint, duration FROM (
		SELECT endpoint, duration,
			ROW_NUMBER() OVER (PARTITION BY endpoint ORDER BY duration) AS rn,
			COUNT(*) OVER (PARTITION BY endpoint) AS cnt
		FROM endpoints WHERE project_id = ? AND recorded_at >= ? AND recorded_at <= ? AND is_stream = 0
	) WHERE rn = CAST(0.99 * (cnt - 1) AS INTEGER) + 1`,
		pid, fromStr, nowStr)
	if err != nil {
		return nil, err
	}
	defer pRows.Close()

	for pRows.Next() {
		var ep string
		var d int64
		if err := pRows.Scan(&ep, &d); err != nil {
			return nil, err
		}
		p99s[ep] = float64(d)
	}
	if err := pRows.Err(); err != nil {
		return nil, err
	}

	var result []impactEndpointData
	for _, c := range candidates {
		c.p99 = p99s[c.endpoint]

		impact := repositories.ComputeImpactScore(c.endpoint, c.totalCount, c.satisfied, c.tolerating, c.bad, c.clientErrors, c.p99, c.offsetMs)
		if impact >= minImpactThreshold {
			c.impact = impact
			result = append(result, c)
		}
	}

	return result, nil
}
