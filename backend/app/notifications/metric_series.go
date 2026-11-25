package notifications

import (
	"cmp"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/v2/app/db"
	"github.com/tracewayapp/traceway/backend/v2/app/models"
	"github.com/tracewayapp/traceway/backend/v2/app/outbox"
	"github.com/tracewayapp/traceway/backend/v2/app/repositories/telemetry/shared"
	"github.com/tracewayapp/traceway/backend/v2/app/repositories/transactional"
)

type metricThresholdConfig struct {
	MetricName      string            `json:"metricName"`
	Operator        string            `json:"operator"`
	ThresholdValue  float64           `json:"thresholdValue"`
	Aggregation     string            `json:"aggregation"`
	LookbackMinutes int               `json:"lookbackMinutes"`
	Tags            map[string]string `json:"tags"`
	GroupBy         []string          `json:"groupBy"`
	// MinCount and GoodThreshold apply to distribution metrics: the observations a
	// series needs before it is judged, and the limit good_ratio counts under.
	MinCount      int     `json:"minCount"`
	GoodThreshold float64 `json:"goodThreshold"`
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
	Insufficient bool // The histogram reported, but it cannot yet be judged.
	Key          string
	Value        float64
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
	// A release comparison fires once per release and segment, and keeps that itself.
	if ruleType == "metric_release_regression" {
		return true
	}
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
	return !s.Insufficient && metricThresholdBreached(s.Value, cfg.Operator, cfg.ThresholdValue)
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

func metricThresholdResult(ctx context.Context, rule *models.NotificationRule, cfg metricThresholdConfig, series []metricSeries, projectId uuid.UUID, truncated bool) (*EvalResult, error) {
	if len(cfg.GroupBy) == 0 {
		i := slices.IndexFunc(series, cfg.breaches)
		if i < 0 {
			return &EvalResult{Fired: false}, nil
		}
		projectName := getProjectName(ctx, projectId)
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
	// An edit, a failed enqueue, or a restart must never separate a page or
	// outbox delivery from the state that will eventually close it.
	var delivery metricSeriesDelivery
	var due []metricSeries
	result, err := db.ExecuteTransactionContext(ctx, func(tx *sql.Tx) (*EvalResult, error) {
		current, err := transactional.NotificationRuleRepository.FindByIdForUpdate(tx, rule.Id)
		if err != nil {
			return nil, err
		}
		if current == nil || !current.Enabled || !current.UpdatedAt.Equal(rule.UpdatedAt) || (current.SnoozedUntil != nil && current.SnoozedUntil.After(time.Now())) {
			return nil, nil
		}
		channel, err := transactional.NotificationChannelRepository.FindById(tx, current.ChannelId)
		if err != nil {
			return nil, err
		}
		if channel == nil || !channel.Enabled {
			return nil, nil
		}
		t, err := transitionMetricSeries(tx, rule.Id, cfg, series, fresh, cooldown, truncated, time.Now().UTC())
		if err != nil {
			return nil, err
		}
		if len(t.alerted) == 0 && !t.overflowAlerted && len(t.recovered) == 0 && len(t.silenced) == 0 && !t.allClear {
			return &EvalResult{}, nil
		}
		project, err := transactional.ProjectRepository.FindById(tx, projectId)
		if err != nil {
			return nil, err
		}
		if project == nil {
			return nil, fmt.Errorf("project %s for rule %d is missing", projectId, rule.Id)
		}
		due = t.due
		result := t.messages(cfg, project.Name)
		delivery.rule = &models.NotificationRuleWithChannel{
			Id: current.Id, ProjectId: projectId, Name: current.Name,
			RuleType: current.RuleType, Severity: current.Severity,
			ChannelId: channel.Id, ChannelName: channel.Name, ChannelType: channel.ChannelType,
		}
		for _, batch := range [][]Message{result.Messages, result.Recoveries} {
			for i := range batch {
				batch[i] = prepareRuleMessage(delivery.rule, batch[i])
				if err := delivery.persist(tx, channel, batch[i]); err != nil {
					delivery.failed = &batch[i]
					return nil, err
				}
			}
		}
		return result, nil
	})
	if err != nil {
		if delivery.failed != nil {
			writeFiredNotification(delivery.rule, *delivery.failed, "failed", err.Error())
		}
		return nil, err
	}
	if result == nil {
		return &EvalResult{}, nil
	}
	for _, s := range due {
		dedup.record(metricSeriesDedupKey(rule.Id, s.Key))
	}
	result.seriesCommitted = true
	delivery.afterCommit()
	return result, nil
}

type metricSeriesDelivery struct {
	rule   *models.NotificationRuleWithChannel
	pages  []metricPageOutcome
	failed *Message
}

type metricPageOutcome struct {
	message Message
	status  string
}

func (d *metricSeriesDelivery) persist(tx *sql.Tx, channel *models.NotificationChannel, msg Message) error {
	if channel.ChannelType != "escalation" {
		_, err := enqueueRuleMessage(tx, d.rule, channel, msg)
		return err
	}
	if metricSeriesPager == nil {
		return fmt.Errorf("escalation pager not initialized for rule %d", d.rule.Id)
	}
	opened, err := metricSeriesPager(tx, channel.Config, d.rule, msg)
	if err == nil && !msg.Recovered {
		status := "deduped"
		if opened {
			status = "sent"
		}
		d.pages = append(d.pages, metricPageOutcome{msg, status})
	}
	return err
}

func (d *metricSeriesDelivery) afterCommit() {
	if d.rule == nil {
		return
	}
	if d.rule.ChannelType != "escalation" {
		outbox.Wake()
		return
	}
	if wakeMetricPager != nil {
		wakeMetricPager()
	}
	for _, page := range d.pages {
		writeFiredNotification(d.rule, page.message, page.status, "")
	}
}

func (t *metricSeriesTransitions) messages(cfg metricThresholdConfig, projectName string) *EvalResult {
	var messages, recoveries []Message
	for _, s := range t.alerted {
		messages = append(messages, buildMetricThresholdMessage(s.Name, s.Value, cfg.Operator, cfg.ThresholdValue, cfg.Aggregation, cfg.LookbackMinutes, projectName))
	}
	if t.overflowAlerted {
		messages = append(messages, buildMetricThresholdOverflowMessage(cfg.MetricName, t.overflow, cfg.Operator, cfg.ThresholdValue, cfg.LookbackMinutes, projectName))
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
	return &EvalResult{Fired: len(messages) > 0, Messages: messages, Recoveries: recoveries}
}

type metricSeriesTransitions struct {
	// due are the breaching series whose cooldown has passed; overflow counts
	// those past the per-evaluation notification limit.
	due             []metricSeries
	overflow        int
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
// metricSeriesSilenceLimit. An open alert is sent again only once the cooldown
// since its last notice has passed; the row keeps that time, so neither a
// restart nor a second instance repeats it early. fresh are the breaching
// series, most severe first. Callers ensure some series reported.
func transitionMetricSeries(tx *sql.Tx, ruleId int, cfg metricThresholdConfig, series, fresh []metricSeries, cooldown time.Duration, truncated bool, now time.Time) (metricSeriesTransitions, error) {
	var t metricSeriesTransitions
	rows, err := transactional.NotificationRuleFiringRepository.FindByRule(tx, ruleId)
	if err != nil {
		return t, err
	}
	firing := make(map[string]*models.NotificationRuleFiring, len(rows))
	for _, row := range rows {
		firing[row.Token] = row
	}
	cooling := func(token string) bool {
		row := firing[token]
		return row != nil && now.Sub(row.NotifiedAt()) < cooldown
	}
	open := func(token string) (bool, error) {
		if firing[token] != nil {
			return true, transactional.NotificationRuleFiringRepository.MarkNotified(tx, ruleId, token, now)
		}
		opened, err := transactional.NotificationRuleFiringRepository.Insert(tx, ruleId, token, now)
		if opened {
			firing[token] = &models.NotificationRuleFiring{RuleId: ruleId, Token: token, Since: now, LastNotifiedAt: &now}
		}
		return opened, err
	}

	for _, s := range fresh {
		if !cooling(s.Name) {
			t.due = append(t.due, s)
		}
	}
	notified := t.due[:min(len(t.due), maxMetricRuleSeriesNotified)]
	t.overflow = len(t.due) - len(notified)
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
	if t.overflow > 0 && !cooling(overflowToken) {
		if t.overflowAlerted, err = open(overflowToken); err != nil {
			return t, err
		}
	}
	// The "N more series" notice names no series, so it clears only once none
	// breaches.
	unresolved := func(s metricSeries) bool { return s.Insufficient || cfg.breaches(s) }
	if firing[overflowToken] != nil && !truncated && !slices.ContainsFunc(series, unresolved) {
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
		if prev, ok := reported[s.Name]; !ok || !unresolved(prev) {
			reported[s.Name] = s
		}
	}
	for _, row := range rows {
		if row.Token == overflowToken {
			continue
		}
		s, reporting := reported[row.Token]
		switch {
		case reporting && (unresolved(s) || len(t.recovered)+len(t.silenced) == maxMetricRuleSeriesNotified):
			if row.SilentSince != nil {
				if err := transactional.NotificationRuleFiringRepository.SetSilentSince(tx, ruleId, row.Token, nil); err != nil {
					return t, err
				}
			}
			continue
		case !reporting && truncated:
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
