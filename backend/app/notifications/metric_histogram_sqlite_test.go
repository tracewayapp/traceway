//go:build !telemetry_ch && !telemetry_duckdb

package notifications

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/v2/app/models"
	"github.com/tracewayapp/traceway/backend/v2/app/repositories/telemetry"
	"github.com/tracewayapp/traceway/backend/v2/app/repositories/telemetry/shared"
)

func insertObservations(t *testing.T, projectId uuid.UUID, name string, at time.Time, dims map[string]string, value float64, n int) {
	t.Helper()
	h := shared.NewHistogram()
	for i := 0; i < n; i++ {
		h.Observe(value, 1)
	}
	if err := telemetry.MetricHistogramRepository.InsertCells(context.Background(), []shared.MetricHistogramCell{
		{ProjectId: projectId, Name: name, RecordedAt: at.UTC().Truncate(time.Minute), Dims: dims, Hist: h},
	}); err != nil {
		t.Fatal(err)
	}
}

func TestMetricThresholdReadsDistributions(t *testing.T) {
	setupNotificationsTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	at := time.Now().Add(-5 * time.Minute)
	insertObservations(t, projectId, "browser.web_vital.lcp", at, map[string]string{"url.path": "/checkout"}, 1500, 80)
	insertObservations(t, projectId, "browser.web_vital.lcp", at, map[string]string{"url.path": "/checkout"}, 6000, 20)
	insertObservations(t, projectId, "browser.web_vital.lcp", at, map[string]string{"url.path": "/rare"}, 9000, 3)
	// Still filling: the last two minutes are not judged yet.
	insertObservations(t, projectId, "browser.web_vital.lcp", time.Now(), map[string]string{"url.path": "/checkout"}, 60000, 500)

	evaluate := func(config string) *EvalResult {
		t.Helper()
		result, err := evaluateMetricThreshold(ctx, &models.NotificationRule{Id: 7000, CooldownMinutes: 0, Config: json.RawMessage(config)}, projectId)
		if err != nil {
			t.Fatal(err)
		}
		return result
	}
	cases := []struct {
		name   string
		config string
		fired  bool
	}{
		{"p75 under the threshold", `{"metricName":"browser.web_vital.lcp","aggregation":"p75","operator":"gt","thresholdValue":4000,"lookbackMinutes":15}`, false},
		{"p95 over the threshold", `{"metricName":"browser.web_vital.lcp","aggregation":"p95","operator":"gt","thresholdValue":4000,"lookbackMinutes":15}`, true},
		{"good ratio as a percentage", `{"metricName":"browser.web_vital.lcp","aggregation":"good_ratio","operator":"lt","thresholdValue":85,"lookbackMinutes":15}`, true},
		{"good ratio with its own limit", `{"metricName":"browser.web_vital.lcp","aggregation":"good_ratio","goodThreshold":7000,"operator":"lt","thresholdValue":75,"lookbackMinutes":15,"tags":{"url.path":"/checkout"}}`, false},
		{"count of observations", `{"metricName":"browser.web_vital.lcp","aggregation":"count","operator":"gte","thresholdValue":103,"lookbackMinutes":15}`, true},
		{"a quiet series is not judged", `{"metricName":"browser.web_vital.lcp","aggregation":"p75","operator":"gt","thresholdValue":4000,"lookbackMinutes":15,"tags":{"url.path":"/rare"}}`, false},
		{"unless the rule lowers the minimum", `{"metricName":"browser.web_vital.lcp","aggregation":"p75","minCount":1,"operator":"gt","thresholdValue":4000,"lookbackMinutes":15,"tags":{"url.path":"/rare"}}`, true},
		{"a distribution with no traffic", `{"metricName":"browser.web_vital.inp","aggregation":"p75","operator":"gt","thresholdValue":1,"lookbackMinutes":15}`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := evaluate(tc.config); got.Fired != tc.fired {
				t.Errorf("fired = %v, want %v", got.Fired, tc.fired)
			}
		})
	}

	// An extreme is an observation, not an estimate: one frozen page on a quiet path is enough.
	if single := evaluate(`{"metricName":"browser.web_vital.lcp","aggregation":"max","operator":"gte","thresholdValue":9000,"lookbackMinutes":15,"tags":{"url.path":"/rare"}}`); !single.Fired {
		t.Error("max over a series with three observations did not fire")
	}
}

func TestReleaseRegressionComparesTheNewestRelease(t *testing.T) {
	setupNotificationsTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	now := time.Now()
	release := func(version string, firstSeenAgo time.Duration, value float64, recent int) {
		insertObservations(t, projectId, "browser.web_vital.lcp", now.Add(-firstSeenAgo), map[string]string{"service.version": version}, value, 5)
		if recent > 0 {
			insertObservations(t, projectId, "browser.web_vital.lcp", now.Add(-30*time.Minute), map[string]string{"service.version": version}, value, recent)
		}
	}
	release("4.0.0", 6*24*time.Hour, 1700, 40)
	release("4.1.0", 3*24*time.Hour, 1800, 400)
	release("4.2.0", 2*time.Hour, 2600, 150)

	rule := func(id int, extra string) *models.NotificationRule {
		return &models.NotificationRule{Id: id, CooldownMinutes: 60, Config: json.RawMessage(fmt.Sprintf(`{"metricName":"browser.web_vital.lcp"%s}`, extra))}
	}
	result, err := evaluateMetricReleaseRegression(ctx, rule(7100, ""), projectId)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Fired || len(result.Messages) != 1 {
		t.Fatalf("a 44%% slower release did not fire: %+v", result)
	}
	if subject := result.Messages[0].Subject; !containsAll(subject, "4.2.0", "browser.web_vital.lcp") {
		t.Errorf("subject %q", subject)
	}
	if body := result.Messages[0].Body; !containsAll(body, "4.2.0", "4.1.0") {
		t.Errorf("compared against the wrong release: %q", body)
	}

	again, err := evaluateMetricReleaseRegression(ctx, rule(7100, ""), projectId)
	if err != nil || again.Fired {
		t.Errorf("the same release fired twice (err %v)", err)
	}
	// Once per release, whatever the cooldown: nothing the rule remembers lives in the process.
	noCooldown := rule(7100, "")
	noCooldown.CooldownMinutes = 0
	if repeated, err := evaluateMetricReleaseRegression(ctx, noCooldown, projectId); err != nil || repeated.Fired {
		t.Errorf("the same release fired again with no cooldown (err %v)", err)
	}
	tolerant, err := evaluateMetricReleaseRegression(ctx, rule(7101, `,"thresholdPercent":60`), projectId)
	if err != nil || tolerant.Fired {
		t.Errorf("fired under its threshold (err %v)", err)
	}
	early, err := evaluateMetricReleaseRegression(ctx, rule(7102, `,"minCount":1000`), projectId)
	if err != nil || early.Fired {
		t.Errorf("judged a release before it had enough observations (err %v)", err)
	}

	// The next release is a new report, and the rule then remembers that one.
	release("4.3.0", time.Hour, 4000, 150)
	if next, err := evaluateMetricReleaseRegression(ctx, noCooldown, projectId); err != nil || !next.Fired || !containsAll(next.Messages[0].Subject, "4.3.0") {
		t.Errorf("a newer, slower release did not fire: %+v (err %v)", next, err)
	}
	if repeated, err := evaluateMetricReleaseRegression(ctx, noCooldown, projectId); err != nil || repeated.Fired {
		t.Errorf("the newer release fired twice (err %v)", err)
	}
}

// A hotfix ships within the hour of the release it fixes. First appearances are read
// by the hour, so the version numbers decide which of the two is the new one.
func TestReleaseRegressionTellsApartReleasesOfTheSameHour(t *testing.T) {
	setupNotificationsTestDB(t)
	projectId := uuid.New()
	at := time.Now().Add(-10 * time.Minute).Truncate(time.Hour).Add(-time.Hour)
	insertObservations(t, projectId, "browser.web_vital.inp", at.Add(5*time.Minute), map[string]string{"service.version": "4.9.0"}, 60, 200)
	insertObservations(t, projectId, "browser.web_vital.inp", at.Add(20*time.Minute), map[string]string{"service.version": "4.10.0"}, 700, 200)

	rule := &models.NotificationRule{Id: 7150, CooldownMinutes: 60, Config: json.RawMessage(`{"metricName":"browser.web_vital.inp","lookbackMinutes":180}`)}
	result, err := evaluateMetricReleaseRegression(context.Background(), rule, projectId)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Fired || len(result.Messages) != 1 {
		t.Fatalf("4.10.0 is the newer release and far slower, but nothing fired: %+v", result)
	}
	if subject := result.Messages[0].Subject; !containsAll(subject, "4.10.0") {
		t.Errorf("subject %q", subject)
	}
}

func TestReleaseRegressionIsNotFooledByADifferentDeviceMix(t *testing.T) {
	setupNotificationsTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	now := time.Now()
	observe := func(version, formFactor string, ago time.Duration, value float64, n int) {
		insertObservations(t, projectId, "browser.web_vital.lcp", now.Add(-ago), map[string]string{"service.version": version, "device.form_factor": formFactor, "os.name": "Android"}, value, n)
	}
	// The old release is mostly desktops, which are fast. Phones are slow on both.
	observe("5.0.0", "desktop", 4*24*time.Hour, 1200, 5)
	observe("5.0.0", "desktop", 30*time.Minute, 1200, 900)
	observe("5.0.0", "phone", 30*time.Minute, 3000, 300)
	// The new release has reached phones first, and is no slower on them.
	observe("5.1.0", "phone", 90*time.Minute, 3000, 5)
	observe("5.1.0", "phone", 30*time.Minute, 3000, 380)
	observe("5.1.0", "desktop", 30*time.Minute, 1200, 20)

	rule := func(id int, extra string) *models.NotificationRule {
		return &models.NotificationRule{Id: id, CooldownMinutes: 60, Config: json.RawMessage(fmt.Sprintf(`{"metricName":"browser.web_vital.lcp"%s}`, extra))}
	}
	result, err := evaluateMetricReleaseRegression(ctx, rule(7200, ""), projectId)
	if err != nil {
		t.Fatal(err)
	}
	if result.Fired {
		t.Fatalf("a rollout to slower devices was reported as a regression: %+v", result.Messages)
	}

	// Now the new release really is slower on phones.
	observe("5.1.0", "phone", 20*time.Minute, 4400, 3000)
	whole, err := evaluateMetricReleaseRegression(ctx, rule(7201, ""), projectId)
	if err != nil || !whole.Fired || len(whole.Messages) != 1 {
		t.Fatalf("a real regression did not fire: %+v (err %v)", whole, err)
	}
	if body := whole.Messages[0].Body; !containsAll(body, "5.1.0", "5.0.0", "same mix") {
		t.Errorf("body %q", body)
	}
	bySegment, err := evaluateMetricReleaseRegression(ctx, rule(7202, `,"groupBy":["device.form_factor"]`), projectId)
	if err != nil || !bySegment.Fired || len(bySegment.Messages) != 1 {
		t.Fatalf("by form factor: %+v (err %v)", bySegment, err)
	}
	if subject := bySegment.Messages[0].Subject; !containsAll(subject, "5.1.0", "device.form_factor=phone") {
		t.Errorf("subject %q", subject)
	}
	again, err := evaluateMetricReleaseRegression(ctx, rule(7202, `,"groupBy":["device.form_factor"]`), projectId)
	if err != nil || again.Fired {
		t.Errorf("the same segment fired twice inside the cooldown (err %v)", err)
	}
}

func TestReleaseRegressionNeedsMoreThanChance(t *testing.T) {
	setupNotificationsTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	now := time.Now()
	// Both releases have the same wide distribution; the new one happens to have a
	// few more slow page views in a small sample.
	spread := func(version string, ago time.Duration, slow int) {
		for i, value := range []float64{800, 1200, 1800, 2600, 3800, 5600} {
			n := 20
			if i >= 4 {
				n = slow
			}
			insertObservations(t, projectId, "browser.web_vital.lcp", now.Add(-ago), map[string]string{"service.version": version}, value, n)
		}
	}
	insertObservations(t, projectId, "browser.web_vital.lcp", now.Add(-3*24*time.Hour), map[string]string{"service.version": "6.0.0"}, 1800, 5)
	spread("6.0.0", 40*time.Minute, 12)
	spread("6.1.0", 30*time.Minute, 20)

	config := `{"metricName":"browser.web_vital.lcp","thresholdPercent":10}`
	newest, previous := shared.NewHistogram(), shared.NewHistogram()
	comparison, err := releaseRegression(ctx, releaseRegressionConfig{MetricName: "browser.web_vital.lcp", Aggregation: "p75", MinCount: 100, LookbackMinutes: 180}, projectId, now.UTC())
	if err != nil || comparison == nil {
		t.Fatalf("comparison %v (err %v)", comparison, err)
	}
	for _, pair := range comparison.segments {
		newest.Merge(pair.newest)
		previous.Merge(pair.previous)
	}
	if current, baseline := newest.Quantile(0.75), previous.Quantile(0.75); current <= baseline*1.1 {
		t.Fatalf("the test needs a p75 that looks regressed: %v against %v", current, baseline)
	}
	result, err := evaluateMetricReleaseRegression(ctx, &models.NotificationRule{Id: 7300, CooldownMinutes: 60, Config: json.RawMessage(config)}, projectId)
	if err != nil {
		t.Fatal(err)
	}
	if result.Fired {
		t.Errorf("a difference inside the confidence interval fired: %+v", result.Messages)
	}
}

func containsAll(s string, parts ...string) bool {
	for _, part := range parts {
		if !contains(s, part) {
			return false
		}
	}
	return true
}

func contains(s, part string) bool {
	for i := 0; i+len(part) <= len(s); i++ {
		if s[i:i+len(part)] == part {
			return true
		}
	}
	return false
}

func TestSamplingDoesNotBypassHistogramMinimumCount(t *testing.T) {
	h := shared.NewHistogram()
	h.Observe(5000, 1000)
	if _, enough := histogramRuleValue(h, metricThresholdConfig{MetricName: "browser.web_vital.lcp", Aggregation: "p75", MinCount: 10}); enough {
		t.Fatal("one sampled visit passed the ten-observation gate")
	}
	if _, _, fired := regressed(releaseRegressionConfig{Aggregation: "p75", MinCount: 10, ThresholdPercent: 20}, h, h); fired {
		t.Fatal("undersampled release fired")
	}
}
