package notifications

import (
	"cmp"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry/shared"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
	traceway "go.tracewayapp.com"
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
	// A firing series silent this long, while the metric's other series still
	// report, is taken to be gone (a replaced pod, a removed host).
	metricSeriesSilenceLimit = time.Hour
)

type metricSeries struct {
	Key   string
	Value float64
	// Name is the series as notifications show it, which is also its dedup token.
	Name string
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

func (cfg metricThresholdConfig) breaches(s metricSeries) bool {
	return metricThresholdBreached(s.Value, cfg.Operator, cfg.ThresholdValue)
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

func metricOverflowToken(metricName string) string {
	return metricName + ":overflow"
}

func metricThresholdResult(rule *models.NotificationRule, cfg metricThresholdConfig, series []metricSeries, projectId uuid.UUID) (*EvalResult, error) {
	if len(cfg.GroupBy) == 0 {
		i := slices.IndexFunc(series, cfg.breaches)
		if i < 0 {
			return &EvalResult{Fired: false}, nil
		}
		projectName := getProjectName(projectId)
		msg := buildMetricThresholdMessage(cfg.MetricName, series[i].Value, cfg.Operator, cfg.ThresholdValue, cfg.Aggregation, cfg.LookbackMinutes, projectName)
		return &EvalResult{Fired: true, Message: msg}, nil
	}
	// No series reporting is an outage of the metric, not a recovery of its
	// series, so nothing can change.
	if len(series) == 0 {
		return &EvalResult{Fired: false}, nil
	}

	var breaching []metricSeries
	for i := range series {
		series[i].Name = cfg.MetricName + metricSeriesLabel(cfg.GroupBy, series[i].Key)
		if cfg.breaches(series[i]) {
			breaching = append(breaching, series[i])
		}
	}

	cooldown := time.Duration(rule.CooldownMinutes) * time.Minute
	var fresh []metricSeries
	for _, s := range breaching {
		if !dedup.isDuplicate(metricSeriesDedupKey(rule.Id, s.Key), cooldown) {
			fresh = append(fresh, s)
		}
	}

	slices.SortFunc(fresh, func(a, b metricSeries) int {
		da := math.Abs(a.Value - cfg.ThresholdValue)
		db := math.Abs(b.Value - cfg.ThresholdValue)
		if c := cmp.Compare(db, da); c != 0 {
			return c
		}
		return cmp.Compare(a.Key, b.Key)
	})
	notified := fresh[:min(len(fresh), maxMetricRuleSeriesNotified)]
	overflow := len(fresh) - len(notified)

	t, err := db.ExecuteTransaction(func(tx *sql.Tx) (metricSeriesTransitions, error) {
		return transitionMetricSeries(tx, rule.Id, cfg, series, notified, overflow > 0, time.Now().UTC())
	})
	if err != nil {
		return nil, err
	}
	// Recorded once the transaction committed, so a failed one is retried on the
	// next poll, and before dispatch, like check_down: a failing channel retries
	// once per cooldown window, not on every poll.
	for _, s := range fresh {
		dedup.record(metricSeriesDedupKey(rule.Id, s.Key))
	}
	if len(t.alerted) == 0 && !t.overflowAlerted && len(t.recovered) == 0 && len(t.silenced) == 0 && !t.allClear {
		return &EvalResult{Fired: false}, nil
	}

	projectName := getProjectName(projectId)
	var messages, recoveries []Message
	for _, s := range t.alerted {
		messages = append(messages, buildMetricThresholdMessage(s.Name, s.Value, cfg.Operator, cfg.ThresholdValue, cfg.Aggregation, cfg.LookbackMinutes, projectName))
	}
	if t.overflowAlerted {
		messages = append(messages, buildMetricThresholdOverflowMessage(cfg.MetricName, overflow, cfg.Operator, cfg.ThresholdValue, cfg.LookbackMinutes, projectName))
	}
	for _, s := range t.recovered {
		recoveries = append(recoveries, buildMetricSeriesRecoveredMessage(s.Name, s.Value, cfg.Operator, cfg.ThresholdValue, cfg.Aggregation, cfg.LookbackMinutes, projectName))
	}
	for _, name := range t.silenced {
		recoveries = append(recoveries, buildMetricSeriesSilencedMessage(name, projectName))
	}
	if t.allClear {
		recoveries = append(recoveries, buildMetricThresholdOverflowRecoveredMessage(cfg.MetricName, cfg.Operator, cfg.ThresholdValue, cfg.LookbackMinutes, projectName))
	}
	return &EvalResult{Fired: len(messages) > 0, Messages: messages, Recoveries: recoveries}, nil
}

type metricSeriesTransitions struct {
	alerted         []metricSeries
	overflowAlerted bool
	recovered       []metricSeries
	silenced        []string
	allClear        bool
}

// transitionMetricSeries keeps the alerts a grouped rule has open in
// notification_rule_firing, keyed by series name, so each one is closed after
// a restart too: by a recovery once its series is back within the threshold,
// or by a silence notice once it has reported nothing for
// metricSeriesSilenceLimit. Callers ensure some series reported.
func transitionMetricSeries(tx *sql.Tx, ruleId int, cfg metricThresholdConfig, series, notified []metricSeries, overflow bool, now time.Time) (metricSeriesTransitions, error) {
	var t metricSeriesTransitions
	rows, err := transactional.NotificationRuleFiringRepository.FindByRule(tx, ruleId)
	if err != nil {
		return t, err
	}
	firing := make(map[string]bool, len(rows))
	for _, row := range rows {
		firing[row.Token] = true
	}
	open := func(token string) (bool, error) {
		if firing[token] {
			return true, nil
		}
		opened, err := transactional.NotificationRuleFiringRepository.Insert(tx, ruleId, token, now)
		firing[token] = opened
		return opened, err
	}

	for _, s := range notified {
		opened, err := open(s.Name)
		if err != nil {
			return t, err
		}
		if opened {
			t.alerted = append(t.alerted, s)
		}
	}
	overflowToken := metricOverflowToken(cfg.MetricName)
	if overflow {
		if t.overflowAlerted, err = open(overflowToken); err != nil {
			return t, err
		}
	}
	// The "N more series" notice names no series, so it clears only once none
	// breaches.
	if firing[overflowToken] && !slices.ContainsFunc(series, cfg.breaches) {
		if t.allClear, err = transactional.NotificationRuleFiringRepository.Delete(tx, ruleId, overflowToken); err != nil {
			return t, err
		}
	}
	if len(rows) == 0 {
		return t, nil
	}

	// Two series can render the same name; the breaching one decides.
	reported := make(map[string]metricSeries, len(series))
	for _, s := range series {
		if prev, ok := reported[s.Name]; !ok || !cfg.breaches(prev) {
			reported[s.Name] = s
		}
	}
	// Past the scan limit a missing series may just not have been read.
	scannedAll := len(series) < maxMetricRuleSeriesScanned
	for _, row := range rows {
		if row.Token == overflowToken {
			continue
		}
		s, reporting := reported[row.Token]
		switch {
		case reporting && cfg.breaches(s):
			if row.SilentSince != nil {
				if err := transactional.NotificationRuleFiringRepository.SetSilentSince(tx, ruleId, row.Token, nil); err != nil {
					return t, err
				}
			}
			continue
		case !reporting && !scannedAll:
			continue
		case !reporting && row.SilentSince == nil:
			if err := transactional.NotificationRuleFiringRepository.SetSilentSince(tx, ruleId, row.Token, &now); err != nil {
				return t, err
			}
			continue
		case !reporting && now.Sub(*row.SilentSince) < metricSeriesSilenceLimit:
			continue
		case len(t.recovered)+len(t.silenced) == maxMetricRuleSeriesNotified:
			// Closed on the next evaluation.
			continue
		}
		closed, err := transactional.NotificationRuleFiringRepository.Delete(tx, ruleId, row.Token)
		if err != nil {
			return t, err
		}
		if closed && reporting {
			t.recovered = append(t.recovered, s)
		} else if closed {
			t.silenced = append(t.silenced, row.Token)
		}
	}
	return t, nil
}

// reopenFiring puts back an alert whose closing notice could not be sent, so
// the next evaluation closes it again.
func reopenFiring(ruleId int, token string) {
	_, err := db.ExecuteTransaction(func(tx *sql.Tx) (bool, error) {
		return transactional.NotificationRuleFiringRepository.Insert(tx, ruleId, token, time.Now())
	})
	if err != nil {
		traceway.CaptureException(fmt.Errorf("failed to reopen alert %q of rule %d after a failed recovery: %w", token, ruleId, err))
	}
}
