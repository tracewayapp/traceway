//go:build !transactional_pg && !telemetry_ch && !telemetry_duckdb

package notifications

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/dbtest"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
)

const freshnessRuleConfig = `{"metricName":"radius.freshness","aggregation":"last","operator":"gt","thresholdValue":900,"lookbackMinutes":5,"groupBy":["site"]}`

type metricSeriesFixture struct {
	ProjectId uuid.UUID
	ChannelId int
	Rule      *models.NotificationRule
}

func setupMetricSeries(t *testing.T) *metricSeriesFixture {
	t.Helper()
	return setupMetricSeriesChannel(t, "slack", `{"webhookUrl":"https://hooks.example.com/x"}`)
}

// setupMetricSeriesChannel seeds a project, a channel and one grouped rule
// (f.Rule) on radius.freshness.
func setupMetricSeriesChannel(t *testing.T, channelType, channelConfig string) *metricSeriesFixture {
	t.Helper()
	dbtest.SetupSQLite(t)
	f := &metricSeriesFixture{}
	_, err := db.ExecuteTransaction(func(tx *sql.Tx) (struct{}, error) {
		project, err := transactional.ProjectRepository.Create(tx, "HS API", "gin")
		if err != nil {
			return struct{}{}, err
		}
		f.ProjectId = project.Id
		now := time.Now().UTC()
		channel := &models.NotificationChannel{ProjectId: project.Id, Name: "Ops", ChannelType: channelType, Config: []byte(channelConfig), Enabled: true, CreatedAt: now, UpdatedAt: now}
		f.ChannelId, err = transactional.NotificationChannelRepository.Create(tx, channel)
		return struct{}{}, err
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	f.Rule = f.newRule(t, freshnessRuleConfig)
	return f
}

// newRule creates a metric_threshold rule with a 30 minute cooldown. Rule ids
// restart in every test database, so the evaluator's in-memory state of the
// id is cleared on both ends.
func (f *metricSeriesFixture) newRule(t *testing.T, config string) *models.NotificationRule {
	t.Helper()
	now := time.Now().UTC()
	rule := &models.NotificationRule{ProjectId: f.ProjectId, ChannelId: f.ChannelId, Name: "freshness", RuleType: "metric_threshold",
		Config: []byte(config), Enabled: true, CooldownMinutes: 30, CreatedAt: now, UpdatedAt: now}
	_, err := db.ExecuteTransaction(func(tx *sql.Tx) (struct{}, error) {
		var err error
		rule.Id, err = transactional.NotificationRuleRepository.Create(tx, rule)
		return struct{}{}, err
	})
	if err != nil {
		t.Fatalf("create rule: %v", err)
	}
	ClearRuleState(rule.Id)
	t.Cleanup(func() { ClearRuleState(rule.Id) })
	return rule
}

func (f *metricSeriesFixture) insert(t *testing.T, value float64, tags string, ago time.Duration) {
	t.Helper()
	if _, err := db.TelemetryDB.Exec("INSERT INTO metric_points (project_id, name, value, tags, recorded_at) VALUES (?, ?, ?, ?, ?)",
		f.ProjectId.String(), "radius.freshness", value, tags, time.Now().UTC().Add(-ago).Format(time.RFC3339Nano)); err != nil {
		t.Fatalf("failed to insert metric point: %v", err)
	}
}

// report records a point of the site's series; with the "last" aggregation the
// newest point is the series' value.
func (f *metricSeriesFixture) report(t *testing.T, site string, value float64, ago time.Duration) {
	t.Helper()
	f.insert(t, value, fmt.Sprintf(`{"site":%q}`, site), ago)
}

func (f *metricSeriesFixture) stopReporting(t *testing.T, site string) {
	t.Helper()
	if _, err := db.TelemetryDB.Exec("DELETE FROM metric_points WHERE tags = ?", fmt.Sprintf(`{"site":%q}`, site)); err != nil {
		t.Fatal(err)
	}
}

func (f *metricSeriesFixture) evaluate(t *testing.T) *EvalResult {
	t.Helper()
	result, err := evaluateMetricThreshold(context.Background(), f.Rule, f.ProjectId)
	if err != nil {
		t.Fatalf("evaluateMetricThreshold failed: %v", err)
	}
	return result
}

// firing returns the rule's open alert for the token, nil when there is none.
func (f *metricSeriesFixture) firing(t *testing.T, token string) *models.NotificationRuleFiring {
	t.Helper()
	rows, err := db.ExecuteTransaction(func(tx *sql.Tx) ([]*models.NotificationRuleFiring, error) {
		return transactional.NotificationRuleFiringRepository.FindByRule(tx, f.Rule.Id)
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if row.Token == token {
			return row
		}
	}
	return nil
}

func (f *metricSeriesFixture) silentFor(t *testing.T, token string, d time.Duration) {
	t.Helper()
	if _, err := db.DB.Exec("UPDATE notification_rule_firing SET silent_since = ? WHERE rule_id = ? AND token = ?", time.Now().UTC().Add(-d), f.Rule.Id, token); err != nil {
		t.Fatal(err)
	}
}

func subjectsOf(messages []Message) []string {
	out := make([]string, 0, len(messages))
	for _, m := range messages {
		out = append(out, m.Subject)
	}
	return out
}

const ruinsSeries = "radius.freshness{site=RUINS}"

func TestMetricSeriesSendsOneRecoveryPerAlertedSeries(t *testing.T) {
	f := setupMetricSeries(t)
	f.report(t, "RUINS", 22166, 2*time.Minute)
	f.report(t, "HILL", 30, 2*time.Minute)

	alert := f.evaluate(t)
	if len(alert.Messages) != 1 || len(alert.Recoveries) != 0 {
		t.Fatalf("first poll: alerts %v, recoveries %v", subjectsOf(alert.Messages), subjectsOf(alert.Recoveries))
	}
	if again := f.evaluate(t); again.Fired || len(again.Recoveries) != 0 {
		t.Fatalf("a series still breaching must stay quiet: %+v", again)
	}

	f.report(t, "RUINS", 100, time.Minute)
	recovery := f.evaluate(t)
	if recovery.Fired || len(recovery.Recoveries) != 1 {
		t.Fatalf("want one recovery and no alert, got alerts %v, recoveries %v", subjectsOf(recovery.Messages), subjectsOf(recovery.Recoveries))
	}
	msg := recovery.Recoveries[0]
	if want := "[HS API] Metric radius.freshness{site=RUINS} recovered: 100.00 (threshold: gt 900.00)"; msg.Subject != want {
		t.Errorf("subject = %q, want %q", msg.Subject, want)
	}
	if !msg.Recovered || msg.Severity != SeverityInfo {
		t.Errorf("recovery must be an Info notice marked Recovered: %+v", msg)
	}
	if msg.DedupToken != alert.Messages[0].DedupToken {
		t.Errorf("recovery token %q must match the alert's %q so it resolves the same page", msg.DedupToken, alert.Messages[0].DedupToken)
	}

	if again := f.evaluate(t); len(again.Recoveries) != 0 {
		t.Fatalf("the recovery must be sent once: %v", subjectsOf(again.Recoveries))
	}
}

func TestMetricSeriesRecoverySurvivesARestart(t *testing.T) {
	f := setupMetricSeries(t)
	f.report(t, "RUINS", 22166, 2*time.Minute)
	if alert := f.evaluate(t); len(alert.Messages) != 1 {
		t.Fatalf("alerts = %v", subjectsOf(alert.Messages))
	}

	ClearRuleState(f.Rule.Id)
	f.report(t, "RUINS", 100, time.Minute)
	if recovery := f.evaluate(t); len(recovery.Recoveries) != 1 {
		t.Fatalf("the open alert must be remembered across a restart, recoveries = %v", subjectsOf(recovery.Recoveries))
	}
}

func TestMetricSeriesSilentForAnHourIsClosed(t *testing.T) {
	f := setupMetricSeries(t)
	f.report(t, "RUINS", 22166, 2*time.Minute)
	f.report(t, "HILL", 30, 2*time.Minute)
	if alert := f.evaluate(t); len(alert.Messages) != 1 {
		t.Fatalf("alerts = %v", subjectsOf(alert.Messages))
	}

	f.stopReporting(t, "RUINS")
	for range 2 {
		if silent := f.evaluate(t); len(silent.Recoveries) != 0 {
			t.Fatalf("a series silent for minutes has not recovered: %v", subjectsOf(silent.Recoveries))
		}
	}
	if row := f.firing(t, ruinsSeries); row == nil || row.SilentSince == nil {
		t.Fatalf("the silent series must stay open with its silence recorded: %+v", row)
	}

	f.silentFor(t, ruinsSeries, metricSeriesSilenceLimit+time.Minute)
	closed := f.evaluate(t)
	if len(closed.Recoveries) != 1 {
		t.Fatalf("recoveries = %v, want the silence notice", subjectsOf(closed.Recoveries))
	}
	msg := closed.Recoveries[0]
	if want := "[HS API] Metric radius.freshness{site=RUINS} stopped reporting, alert closed"; msg.Subject != want {
		t.Errorf("subject = %q, want %q", msg.Subject, want)
	}
	if !msg.Recovered || msg.DedupToken != ruinsSeries {
		t.Errorf("the silence notice must close the series' alert and page: %+v", msg)
	}
	if row := f.firing(t, ruinsSeries); row != nil {
		t.Fatalf("the alert must be closed: %+v", row)
	}
}

func TestMetricSeriesSilentSeriesThatReturnsWithinTheThresholdRecovers(t *testing.T) {
	f := setupMetricSeries(t)
	f.report(t, "RUINS", 22166, 2*time.Minute)
	f.report(t, "HILL", 30, 2*time.Minute)
	f.evaluate(t)
	f.stopReporting(t, "RUINS")
	f.evaluate(t)

	f.report(t, "RUINS", 100, time.Minute)
	recovery := f.evaluate(t)
	if len(recovery.Recoveries) != 1 || !strings.Contains(recovery.Recoveries[0].Subject, "{site=RUINS} recovered") {
		t.Fatalf("recoveries = %v, want RUINS' recovery", subjectsOf(recovery.Recoveries))
	}
}

func TestMetricSeriesSilenceEndsWhenTheSeriesBreachesAgain(t *testing.T) {
	f := setupMetricSeries(t)
	f.report(t, "RUINS", 22166, 2*time.Minute)
	f.report(t, "HILL", 30, 2*time.Minute)
	f.evaluate(t)
	f.stopReporting(t, "RUINS")
	f.evaluate(t)

	f.report(t, "RUINS", 30000, time.Minute)
	if result := f.evaluate(t); result.Fired || len(result.Recoveries) != 0 {
		t.Fatalf("a series breaching within its cooldown stays quiet: %+v", result)
	}
	if row := f.firing(t, ruinsSeries); row == nil || row.SilentSince != nil {
		t.Fatalf("the series reported again, so its silence must be cleared: %+v", row)
	}
}

func TestMetricSeriesStayFiringWhileTheWholeMetricIsSilent(t *testing.T) {
	f := setupMetricSeries(t)
	f.report(t, "RUINS", 22166, 2*time.Minute)
	f.report(t, "HILL", 30, 2*time.Minute)
	f.evaluate(t)
	f.silentFor(t, ruinsSeries, 2*metricSeriesSilenceLimit)

	f.stopReporting(t, "RUINS")
	f.stopReporting(t, "HILL")
	if result := f.evaluate(t); len(result.Recoveries) != 0 {
		t.Fatalf("no series reporting is an outage, not a recovery: %v", subjectsOf(result.Recoveries))
	}
	if f.firing(t, ruinsSeries) == nil {
		t.Fatal("the alert must stay open")
	}
}

func TestMetricSeriesBeyondTheScanLimitIsNotSilenced(t *testing.T) {
	f := setupMetricSeries(t)
	cfg, err := parseMetricThresholdConfig(f.Rule.Config)
	if err != nil {
		t.Fatal(err)
	}
	const unread = "radius.freshness{site=zzz}"
	if _, err := db.ExecuteTransaction(func(tx *sql.Tx) (bool, error) {
		return transactional.NotificationRuleFiringRepository.Insert(tx, f.Rule.Id, unread, time.Now())
	}); err != nil {
		t.Fatal(err)
	}
	f.silentFor(t, unread, 2*metricSeriesSilenceLimit)
	evaluate := func(count int) *EvalResult {
		t.Helper()
		series := make([]metricSeries, count)
		for i := range series {
			series[i] = metricSeries{Key: fmt.Sprintf("s%04d", i), Value: 1}
		}
		result, err := metricThresholdResult(f.Rule, cfg, series, f.ProjectId)
		if err != nil {
			t.Fatal(err)
		}
		return result
	}

	if result := evaluate(maxMetricRuleSeriesScanned); len(result.Recoveries) != 0 {
		t.Fatalf("a truncated scan cannot show a series is gone: %v", subjectsOf(result.Recoveries))
	}
	if result := evaluate(maxMetricRuleSeriesScanned - 1); len(result.Recoveries) != 1 {
		t.Fatalf("recoveries = %v, want the silence notice once every series was read", subjectsOf(result.Recoveries))
	}
}

func TestMetricSeriesRecoveriesAreCappedAndTheOverflowClearsLast(t *testing.T) {
	f := setupMetricSeries(t)
	cfg, err := parseMetricThresholdConfig(f.Rule.Config)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ThresholdValue = 1
	const total = 2*maxMetricRuleSeriesNotified + 7
	values := make([]float64, total)
	evaluate := func() *EvalResult {
		t.Helper()
		series := make([]metricSeries, total)
		for i := range series {
			series[i] = metricSeries{Key: fmt.Sprintf("h%03d", i), Value: values[i]}
		}
		result, err := metricThresholdResult(f.Rule, cfg, series, f.ProjectId)
		if err != nil {
			t.Fatal(err)
		}
		return result
	}

	// The first poll names the 50 worst series and summarizes the other 7.
	first := maxMetricRuleSeriesNotified + 7
	for i := range first {
		values[i] = float64(i + 2)
	}
	if alert := evaluate(); len(alert.Messages) != maxMetricRuleSeriesNotified+1 {
		t.Fatalf("alerts = %d, want 50 plus the overflow notice", len(alert.Messages))
	}
	// 50 more start breaching while the first ones are still in their cooldown.
	for i := first; i < total; i++ {
		values[i] = 1000
	}
	if alert := evaluate(); len(alert.Messages) != maxMetricRuleSeriesNotified {
		t.Fatalf("alerts = %d, want the 50 new series", len(alert.Messages))
	}

	clear(values)
	recovery := evaluate()
	got := subjectsOf(recovery.Recoveries)
	if len(got) != maxMetricRuleSeriesNotified+1 {
		t.Fatalf("recoveries = %d, want 50 series plus the overflow notice", len(got))
	}
	for _, s := range got[:maxMetricRuleSeriesNotified] {
		for i := range 7 {
			if strings.Contains(s, fmt.Sprintf("site=h%03d}", i)) {
				t.Errorf("series h%03d was only summarized, so it has no recovery of its own: %q", i, s)
			}
		}
	}
	if last := got[maxMetricRuleSeriesNotified]; !strings.Contains(last, "no series breaches the threshold") {
		t.Errorf("last recovery = %q, want the overflow notice's", last)
	}

	rest := evaluate()
	if len(rest.Recoveries) != maxMetricRuleSeriesNotified {
		t.Fatalf("recoveries = %d, want the 50 series held back by the cap", len(rest.Recoveries))
	}
	if done := evaluate(); len(done.Recoveries) != 0 {
		t.Fatalf("recoveries = %v, want none left", subjectsOf(done.Recoveries))
	}
}

func TestMetricSeriesRecoveryResolvesTheSeriesPage(t *testing.T) {
	f := setupMetricSeriesChannel(t, "escalation", `{"policyId":1}`)
	var opened, resolved []string
	previousOpener, previousResolver := pageOpener, pageResolver
	pageOpener = func(_ json.RawMessage, _ *models.NotificationRuleWithChannel, msg Message) (bool, error) {
		opened = append(opened, msg.DedupToken)
		return true, nil
	}
	pageResolver = func(ruleId int, dedupToken string) error {
		if ruleId != f.Rule.Id {
			t.Errorf("resolved a page of rule %d, want %d", ruleId, f.Rule.Id)
		}
		resolved = append(resolved, dedupToken)
		return nil
	}
	t.Cleanup(func() { pageOpener, pageResolver = previousOpener, previousResolver })

	f.report(t, "RUINS", 22166, 2*time.Minute)
	evaluatePolledRules(context.Background())
	if len(opened) != 1 || opened[0] != ruinsSeries {
		t.Fatalf("opened pages = %v", opened)
	}

	f.report(t, "RUINS", 100, time.Minute)
	evaluatePolledRules(context.Background())
	if len(resolved) != 1 || resolved[0] != opened[0] {
		t.Fatalf("resolved pages = %v, want the page the alert opened", resolved)
	}
	if len(opened) != 1 {
		t.Fatalf("a recovery must not open a page: %v", opened)
	}
}

func TestMetricSeriesRecoveryIsEnqueuedForChatChannels(t *testing.T) {
	f := setupMetricSeries(t)
	f.report(t, "RUINS", 22166, 2*time.Minute)
	evaluatePolledRules(context.Background())
	f.report(t, "RUINS", 100, time.Minute)
	evaluatePolledRules(context.Background())

	rows := outboxRows(t)
	if len(rows) != 2 {
		t.Fatalf("outbox rows = %d, want the alert and the recovery", len(rows))
	}
	var recovery Message
	if err := json.Unmarshal(rows[1].Message, &recovery); err != nil {
		t.Fatal(err)
	}
	if !recovery.Recovered || !strings.Contains(recovery.Subject, "{site=RUINS} recovered") {
		t.Fatalf("second delivery = %+v, want the recovery", recovery)
	}
}

func TestRuleFiringTransitionsOnce(t *testing.T) {
	f := setupMetricSeries(t)
	run := func(op func(tx *sql.Tx) (bool, error)) bool {
		t.Helper()
		changed, err := db.ExecuteTransaction(op)
		if err != nil {
			t.Fatal(err)
		}
		return changed
	}
	insert := func(tx *sql.Tx) (bool, error) {
		return transactional.NotificationRuleFiringRepository.Insert(tx, f.Rule.Id, "m{site=A}", time.Now())
	}
	remove := func(tx *sql.Tx) (bool, error) {
		return transactional.NotificationRuleFiringRepository.Delete(tx, f.Rule.Id, "m{site=A}")
	}
	if !run(insert) || run(insert) {
		t.Fatal("only the first of two instances may open the alert")
	}
	if !run(remove) || run(remove) {
		t.Fatal("only the first of two instances may close it")
	}
}

func TestEvaluateMetricThresholdGroupBy(t *testing.T) {
	f := setupMetricSeries(t)
	ctx := context.Background()
	projectId := f.ProjectId
	insert := func(value float64, tags string) {
		t.Helper()
		f.insert(t, value, tags, time.Minute)
	}
	insert(22166, `{"site":"RUINS","nas":"10.0.0.1"}`)
	// Older, so "last" reads RUINS as 22166.
	f.insert(t, 10, `{"site":"RUINS","nas":"10.0.0.2"}`, 2*time.Minute)
	insert(30, `{"site":"HILL","nas":"10.0.0.3"}`)
	insert(950, `{"nas":"10.0.0.4"}`)

	newRule := func(agg, groupBy string) *models.NotificationRule {
		return f.newRule(t, fmt.Sprintf(`{"metricName":"radius.freshness","aggregation":%q,"operator":"gt","thresholdValue":900,"lookbackMinutes":5,"groupBy":%s}`, agg, groupBy))
	}

	for _, agg := range []string{"max", "last", "p95"} {
		t.Run("one message per breaching series ("+agg+")", func(t *testing.T) {
			rule := newRule(agg, `["site"]`)
			result, err := evaluateMetricThreshold(ctx, rule, projectId)
			if err != nil {
				t.Fatalf("evaluateMetricThreshold failed: %v", err)
			}
			got := subjectsOf(result.Messages)
			if !result.Fired || len(got) != 2 {
				t.Fatalf("messages = %v, want two", got)
			}
			if !strings.Contains(got[0], "radius.freshness{site=RUINS}") || !strings.Contains(got[1], "radius.freshness{site=(empty)}") {
				t.Errorf("subjects = %v, want RUINS first, then the untagged series", got)
			}
			if result.Messages[0].DedupToken != "radius.freshness{site=RUINS}" {
				t.Errorf("dedup token = %q", result.Messages[0].DedupToken)
			}
		})
	}

	t.Run("several group keys name every value", func(t *testing.T) {
		result, err := evaluateMetricThreshold(ctx, newRule("max", `["site","nas"]`), projectId)
		if err != nil {
			t.Fatalf("evaluateMetricThreshold failed: %v", err)
		}
		got := subjectsOf(result.Messages)
		if len(got) != 2 || !strings.Contains(got[0], "{site=RUINS, nas=10.0.0.1}") {
			t.Errorf("subjects = %v", got)
		}
	})

	t.Run("a breaching series stays quiet for the cooldown, a new one still fires", func(t *testing.T) {
		rule := newRule("max", `["site"]`)
		if _, err := evaluateMetricThreshold(ctx, rule, projectId); err != nil {
			t.Fatalf("evaluateMetricThreshold failed: %v", err)
		}
		insert(5000, `{"site":"HILL","nas":"10.0.0.3"}`)
		result, err := evaluateMetricThreshold(ctx, rule, projectId)
		if err != nil {
			t.Fatalf("evaluateMetricThreshold failed: %v", err)
		}
		got := subjectsOf(result.Messages)
		if len(got) != 1 || !strings.Contains(got[0], "{site=HILL}") {
			t.Errorf("subjects = %v, want only HILL", got)
		}
		if !isPerSeriesRule(rule.RuleType, rule.Config) {
			t.Error("a grouped rule must bypass the rule-level cooldown")
		}
	})

	t.Run("without groupBy the rule reduces every series to one value", func(t *testing.T) {
		rule := newRule("max", `[]`)
		result, err := evaluateMetricThreshold(ctx, rule, projectId)
		if err != nil {
			t.Fatalf("evaluateMetricThreshold failed: %v", err)
		}
		if !result.Fired || len(result.Messages) != 0 || !strings.Contains(result.Message.Subject, "Metric radius.freshness is 22166.00") {
			t.Errorf("result = %+v", result)
		}
		if isPerSeriesRule(rule.RuleType, rule.Config) {
			t.Error("an ungrouped rule keeps the rule-level cooldown")
		}
	})
}

func TestMetricThresholdResultCapsNotifiedSeries(t *testing.T) {
	f := setupMetricSeries(t)

	cfg := metricThresholdConfig{MetricName: "m", Operator: "gt", ThresholdValue: 1, LookbackMinutes: 5, GroupBy: []string{"host"}}
	var series []metricSeries
	for i := range maxMetricRuleSeriesNotified + 7 {
		series = append(series, metricSeries{Key: fmt.Sprintf("h%03d", i), Value: float64(i + 2)})
	}
	rule := f.newRule(t, `{"metricName":"m","groupBy":["host"]}`)
	result, err := metricThresholdResult(rule, cfg, series, f.ProjectId)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Messages) != maxMetricRuleSeriesNotified+1 {
		t.Fatalf("messages = %d, want %d", len(result.Messages), maxMetricRuleSeriesNotified+1)
	}
	if !strings.Contains(result.Messages[0].Subject, "m{host=h056}") {
		t.Errorf("first subject = %q, want the worst series first", result.Messages[0].Subject)
	}
	if last := result.Messages[maxMetricRuleSeriesNotified].Subject; !strings.Contains(last, "7 more series") {
		t.Errorf("overflow subject = %q", last)
	}
	again, err := metricThresholdResult(rule, cfg, series, f.ProjectId)
	if err != nil {
		t.Fatal(err)
	}
	if again.Fired {
		t.Errorf("series summarized in the overflow notice must also wait out the cooldown, got %d messages", len(again.Messages))
	}
}

func TestMetricSeriesFailedRecoveryIsRetried(t *testing.T) {
	f := setupMetricSeriesChannel(t, "escalation", `{"policyId":1}`)
	resolveErr := fmt.Errorf("database is locked")
	var resolved []string
	previousOpener, previousResolver := pageOpener, pageResolver
	pageOpener = func(json.RawMessage, *models.NotificationRuleWithChannel, Message) (bool, error) { return true, nil }
	pageResolver = func(_ int, dedupToken string) error {
		resolved = append(resolved, dedupToken)
		err := resolveErr
		resolveErr = nil
		return err
	}
	t.Cleanup(func() { pageOpener, pageResolver = previousOpener, previousResolver })

	f.report(t, "RUINS", 22166, 2*time.Minute)
	evaluatePolledRules(context.Background())
	f.report(t, "RUINS", 100, time.Minute)
	evaluatePolledRules(context.Background())
	if len(resolved) != 1 || f.firing(t, ruinsSeries) == nil {
		t.Fatalf("a recovery whose page could not be resolved must keep the alert open: resolved %v", resolved)
	}

	evaluatePolledRules(context.Background())
	if len(resolved) != 2 || f.firing(t, ruinsSeries) != nil {
		t.Fatalf("the next poll must resolve the page again: resolved %v", resolved)
	}
}

func TestMetricSeriesAlertIsRetriedAfterAFailedTransaction(t *testing.T) {
	f := setupMetricSeries(t)
	f.report(t, "RUINS", 22166, 2*time.Minute)
	rename := func(from, to string) {
		t.Helper()
		if _, err := db.DB.Exec(fmt.Sprintf("ALTER TABLE %s RENAME TO %s", from, to)); err != nil {
			t.Fatal(err)
		}
	}

	rename("notification_rule_firing", "notification_rule_firing_moved")
	if _, err := evaluateMetricThreshold(context.Background(), f.Rule, f.ProjectId); err == nil {
		t.Fatal("the evaluation must fail while its state cannot be read")
	}
	rename("notification_rule_firing_moved", "notification_rule_firing")

	if alert := f.evaluate(t); len(alert.Messages) != 1 {
		t.Fatalf("alerts = %v, want the alert the failed evaluation could not send", subjectsOf(alert.Messages))
	}
}
