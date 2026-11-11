package notifications

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry/shared"
)

type metricThresholdConfig struct {
	MetricName      string            `json:"metricName"`
	Operator        string            `json:"operator"`
	ThresholdValue  float64           `json:"thresholdValue"`
	Aggregation     string            `json:"aggregation"`
	LookbackMinutes int               `json:"lookbackMinutes"`
	Tags            map[string]string `json:"tags"`
	GroupBy         []string          `json:"groupBy"`
}

const (
	maxMetricRuleSeriesNotified = 50
	// Series are read in group key order, so a metric with more series than
	// this has the rest left unevaluated rather than unbounded memory.
	maxMetricRuleSeriesScanned = 5000
)

type metricSeries struct {
	Key   string
	Value float64
}

func parseMetricThresholdConfig(raw json.RawMessage) (metricThresholdConfig, error) {
	var cfg metricThresholdConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return cfg, fmt.Errorf("invalid metric_threshold config: %w", err)
	}
	if cfg.LookbackMinutes <= 0 {
		cfg.LookbackMinutes = 5
	}
	return cfg, nil
}

// isPerSeriesRule reports whether a rule keeps its cooldown per series instead
// of per rule, so one breaching series cannot silence another.
func isPerSeriesRule(ruleType string, config json.RawMessage) bool {
	if ruleType != "metric_threshold" {
		return false
	}
	var cfg struct {
		GroupBy []string `json:"groupBy"`
	}
	return json.Unmarshal(config, &cfg) == nil && len(cfg.GroupBy) > 0
}

// metricGroupExpr joins one tagExpr per groupBy key into a single group key
// column. tagExpr takes the tag key as its only placeholder.
func metricGroupExpr(groupBy []string, tagExpr string) (string, []interface{}) {
	if len(groupBy) == 0 {
		return "''", nil
	}
	parts := make([]string, len(groupBy))
	var args []interface{}
	for i, key := range groupBy {
		parts[i] = tagExpr
		if i > 0 {
			args = append(args, shared.GroupKeySeparator)
		}
		args = append(args, key)
	}
	return strings.Join(parts, " || ? || "), args
}

func metricThresholdBreached(value float64, operator string, threshold float64) bool {
	switch operator {
	case "gt":
		return value > threshold
	case "gte":
		return value >= threshold
	case "lt":
		return value < threshold
	case "lte":
		return value <= threshold
	case "eq":
		return value == threshold
	}
	return false
}

func metricSeriesLabel(groupBy []string, key string) string {
	if len(groupBy) == 0 {
		return ""
	}
	values := shared.SplitGroupKey(key)
	pairs := make([]string, len(groupBy))
	for i, k := range groupBy {
		v := ""
		if i < len(values) {
			v = values[i]
		}
		if v == "" {
			v = "(empty)"
		}
		pairs[i] = k + "=" + v
	}
	return "{" + strings.Join(pairs, ", ") + "}"
}

func metricThresholdResult(ctx context.Context, rule *models.NotificationRule, cfg metricThresholdConfig, series []metricSeries, projectId uuid.UUID) *EvalResult {
	var breaching []metricSeries
	for _, s := range series {
		if metricThresholdBreached(s.Value, cfg.Operator, cfg.ThresholdValue) {
			breaching = append(breaching, s)
		}
	}
	if len(breaching) == 0 {
		return &EvalResult{Fired: false}
	}

	if len(cfg.GroupBy) == 0 {
		projectName := getProjectName(ctx, projectId)
		msg := buildMetricThresholdMessage(cfg.MetricName, "", breaching[0].Value, cfg.Operator, cfg.ThresholdValue, cfg.Aggregation, cfg.LookbackMinutes, projectName)
		return &EvalResult{Fired: true, Message: msg}
	}

	cooldown := time.Duration(rule.CooldownMinutes) * time.Minute
	var fresh []metricSeries
	for _, s := range breaching {
		key := metricSeriesDedupKey(rule.Id, s.Key)
		if dedup.isDuplicate(key, cooldown) {
			continue
		}
		// Recorded before dispatch, like check_down: a failing channel retries
		// once per cooldown window, not on every poll.
		dedup.record(key)
		fresh = append(fresh, s)
	}
	if len(fresh) == 0 {
		return &EvalResult{Fired: false}
	}

	slices.SortFunc(fresh, func(a, b metricSeries) int {
		da := math.Abs(a.Value - cfg.ThresholdValue)
		db := math.Abs(b.Value - cfg.ThresholdValue)
		if c := cmp.Compare(db, da); c != 0 {
			return c
		}
		return cmp.Compare(a.Key, b.Key)
	})

	projectName := getProjectName(ctx, projectId)
	notified := fresh[:min(len(fresh), maxMetricRuleSeriesNotified)]
	messages := make([]Message, 0, len(notified)+1)
	for _, s := range notified {
		label := metricSeriesLabel(cfg.GroupBy, s.Key)
		messages = append(messages, buildMetricThresholdMessage(cfg.MetricName, label, s.Value, cfg.Operator, cfg.ThresholdValue, cfg.Aggregation, cfg.LookbackMinutes, projectName))
	}
	if rest := len(fresh) - len(notified); rest > 0 {
		messages = append(messages, buildMetricThresholdOverflowMessage(cfg.MetricName, rest, cfg.Operator, cfg.ThresholdValue, cfg.LookbackMinutes, projectName))
	}
	return &EvalResult{Fired: true, Messages: messages}
}
