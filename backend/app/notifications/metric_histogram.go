package notifications

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/v2/app/clientmetrics"
	"github.com/tracewayapp/traceway/backend/v2/app/models"
	"github.com/tracewayapp/traceway/backend/v2/app/repositories/telemetry"
	"github.com/tracewayapp/traceway/backend/v2/app/repositories/telemetry/shared"
	"github.com/tracewayapp/traceway/backend/v2/app/sdkconfig"
)

const (
	// A distribution is read up to this far behind now: a client uploads when its
	// page is hidden or every few minutes, so the last minutes are still filling.
	histogramRuleLag = 2 * time.Minute
	// defaultHistogramRuleMinCount keeps a quiet hour from paging anyone: the p95
	// of three page views is not a p95.
	defaultHistogramRuleMinCount = 10

	releaseDimension = "service.version"
)

func histogramRuleWindow(now time.Time, lookbackMinutes int) (time.Time, time.Time) {
	to := now.Add(-histogramRuleLag)
	return to.Add(-time.Duration(lookbackMinutes) * time.Minute), to
}

// mergedHistograms reads a distribution metric over a window and merges every time
// bucket of a group into one histogram.
func mergedHistograms(ctx context.Context, projectId uuid.UUID, name string, from, to time.Time, tags map[string]string, groupBy []string) (map[string]*shared.Histogram, error) {
	minutes := int(to.Sub(from).Minutes()) + 1
	series, _, err := telemetry.MetricHistogramRepository.Query(ctx, projectId, name, from, to, minutes, tags, groupBy, maxMetricRuleSeriesScanned)
	if err != nil {
		return nil, err
	}
	merged := make(map[string]*shared.Histogram, len(series))
	for key, points := range series {
		h := shared.NewHistogram()
		for _, point := range points {
			h.Merge(point.Hist)
		}
		if h.Count > 0 {
			merged[key] = h
		}
	}
	return merged, nil
}

func isDistributionRuleMetric(name string) bool {
	entry, known := clientmetrics.Lookup(name)
	return known && entry.Shape == clientmetrics.Distribution
}

// histogramRuleValue reduces a histogram to the number a rule compares. good_ratio
// is a percentage here (75 means three quarters of the observations were good),
// because that is how a threshold is typed into the rule form.
func histogramRuleValue(h *shared.Histogram, cfg metricThresholdConfig) (float64, bool) {
	switch cfg.Aggregation {
	case "count", "sum":
		return h.Aggregate(cfg.Aggregation, 0), true
	}
	minCount := cfg.MinCount
	if minCount <= 0 {
		minCount = defaultHistogramRuleMinCount
	}
	if h.Count < float64(minCount) {
		return 0, false
	}
	if cfg.Aggregation == shared.AggregationGoodRatio {
		threshold := cfg.GoodThreshold
		if threshold <= 0 {
			if entry, known := clientmetrics.Lookup(cfg.MetricName); known {
				threshold = entry.Good
			}
		}
		if threshold <= 0 {
			return 0, false
		}
		return 100 * h.Aggregate(shared.AggregationGoodRatio, threshold), true
	}
	return h.Aggregate(cfg.Aggregation, 0), true
}

// histogramMetricSeries evaluates a metric_threshold rule against the histogram
// table. It reports false when the metric is not stored as a distribution, in which
// case the caller reads metric points as before.
func histogramMetricSeries(ctx context.Context, cfg metricThresholdConfig, projectId uuid.UUID, now time.Time) ([]metricSeries, bool, error) {
	if cfg.Aggregation == "last" {
		return nil, false, nil
	}
	from, to := histogramRuleWindow(now, cfg.LookbackMinutes)
	histograms, err := mergedHistograms(ctx, projectId, cfg.MetricName, from, to, cfg.Tags, cfg.GroupBy)
	if err != nil {
		return nil, false, err
	}
	// No rows: a distribution with no traffic, or a point metric. Only the first is
	// ours, and it has nothing to evaluate.
	if len(histograms) == 0 {
		return nil, shared.IsHistogramOnlyAggregation(cfg.Aggregation) && cfg.Aggregation != "p95" && cfg.Aggregation != "p99" || isDistributionRuleMetric(cfg.MetricName), nil
	}
	keys := make([]string, 0, len(histograms))
	for key := range histograms {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	series := make([]metricSeries, 0, len(keys))
	for _, key := range keys {
		value, ok := histogramRuleValue(histograms[key], cfg)
		if !ok {
			continue
		}
		if len(cfg.GroupBy) == 0 {
			key = ""
		}
		series = append(series, metricSeries{Key: key, Value: value})
	}
	return series, true, nil
}

type releaseRegressionConfig struct {
	MetricName string `json:"metricName"`
	// Aggregation is a percentile or avg; p75 by default, the web vitals convention.
	Aggregation string `json:"aggregation"`
	// ThresholdPercent is how much worse the new release may be before the rule fires.
	ThresholdPercent float64           `json:"thresholdPercent"`
	MinCount         int               `json:"minCount"`
	LookbackMinutes  int               `json:"lookbackMinutes"`
	Tags             map[string]string `json:"tags"`
	// GroupBy compares the two releases inside each segment (a form factor, an OS)
	// and fires per segment. Without it the rule compares the releases as a whole,
	// after giving the old release the device mix of the new one.
	GroupBy []string `json:"groupBy"`
}

func parseReleaseRegressionConfig(raw json.RawMessage) (releaseRegressionConfig, error) {
	var cfg releaseRegressionConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return cfg, fmt.Errorf("invalid metric_release_regression config: %w", err)
	}
	if cfg.Aggregation == "" {
		cfg.Aggregation = "p75"
	}
	if cfg.ThresholdPercent <= 0 {
		cfg.ThresholdPercent = 20
	}
	if cfg.MinCount <= 0 {
		cfg.MinCount = 100
	}
	if cfg.LookbackMinutes <= 0 {
		cfg.LookbackMinutes = 180
	}
	return cfg, nil
}

const (
	releaseHistoryWindow = 7 * 24 * time.Hour
	// A mix adjustment that covers less than this share of the new release's
	// traffic says little about it, so the releases are then compared as they are.
	minMixCoverage = 0.5
)

// releaseMixStrata is what a rollout usually differs in from the release before it:
// a staged rollout reaches one platform or one kind of device first, and those are
// faster or slower whatever the code does.
var releaseMixStrata = []string{"device.form_factor", "os.name"}

type releasePair struct {
	newest, previous *shared.Histogram
}

type releaseComparison struct {
	newest, previous string
	// segments holds the recent observations of both releases per segment (GroupBy)
	// or per mix stratum.
	segments map[string]*releasePair
}

// releaseRegression finds the two releases to compare and their recent
// observations. "Newest" is the release first seen last within a week: version
// strings do not sort, first appearances do. A release is judged only once it has
// minCount observations in the window, so the first hundred users of a rollout
// decide, not the first three. The earlier release is the busiest one with enough
// observations in the same window.
func releaseRegression(ctx context.Context, cfg releaseRegressionConfig, projectId uuid.UUID, now time.Time) (*releaseComparison, error) {
	_, to := histogramRuleWindow(now, cfg.LookbackMinutes)
	recentFrom := to.Add(-time.Duration(cfg.LookbackMinutes) * time.Minute).Truncate(time.Hour)
	segmentKeys := cfg.GroupBy
	if len(segmentKeys) == 0 {
		segmentKeys = releaseMixStrata
	}
	groupBy := append([]string{releaseDimension}, segmentKeys...)
	series, _, err := telemetry.MetricHistogramRepository.Query(ctx, projectId, cfg.MetricName, to.Add(-releaseHistoryWindow), to, 60, cfg.Tags, groupBy, maxMetricRuleSeriesScanned)
	if err != nil {
		return nil, err
	}
	type release struct {
		name      string
		firstSeen time.Time
		count     float64
		segments  map[string]*shared.Histogram
	}
	releases := map[string]*release{}
	for key, points := range series {
		parts := shared.SplitGroupKey(key)
		name := parts[0]
		if len(points) == 0 || len(parts) != len(groupBy) || name == "" || name == clientmetrics.Other {
			continue
		}
		r := releases[name]
		if r == nil {
			r = &release{name: name, firstSeen: points[0].Timestamp, segments: map[string]*shared.Histogram{}}
			releases[name] = r
		}
		if points[0].Timestamp.Before(r.firstSeen) {
			r.firstSeen = points[0].Timestamp
		}
		segment := strings.Join(parts[1:], shared.GroupKeySeparator)
		for _, point := range points {
			if point.Timestamp.Before(recentFrom) {
				continue
			}
			if r.segments[segment] == nil {
				r.segments[segment] = shared.NewHistogram()
			}
			r.segments[segment].Merge(point.Hist)
			r.count += point.Hist.Count
		}
	}
	ordered := make([]*release, 0, len(releases))
	for _, r := range releases {
		ordered = append(ordered, r)
	}
	slices.SortFunc(ordered, func(a, b *release) int {
		if c := b.firstSeen.Compare(a.firstSeen); c != 0 {
			return c
		}
		// First appearances are read by the hour. Two releases of the same hour, a
		// hotfix right after its release, are told apart by their version numbers.
		if c, ok := sdkconfig.CompareVersions(b.name, a.name); ok && c != 0 {
			return c
		}
		return strings.Compare(a.name, b.name)
	})
	enough := func(r *release) bool { return r.count >= float64(cfg.MinCount) }
	if len(ordered) < 2 || !enough(ordered[0]) {
		return nil, nil
	}
	newest := ordered[0]
	var previous *release
	for _, candidate := range ordered[1:] {
		if enough(candidate) && (previous == nil || candidate.count > previous.count) {
			previous = candidate
		}
	}
	if previous == nil {
		return nil, nil
	}
	comparison := &releaseComparison{newest: newest.name, previous: previous.name, segments: map[string]*releasePair{}}
	for segment, h := range newest.segments {
		comparison.segments[segment] = &releasePair{newest: h, previous: previous.segments[segment]}
	}
	return comparison, nil
}

// mixAdjusted gives the earlier release the device mix of the new one: each stratum
// of the old release is weighted to the share that stratum has in the new release,
// so a rollout that reached phones first is compared with the old release on phones.
// It reports false when the strata both releases share cover too little of the new
// release, and the caller then compares the releases as they are.
func (c *releaseComparison) mixAdjusted() (newest, previous *shared.Histogram, adjusted bool) {
	newest, previous = shared.NewHistogram(), shared.NewHistogram()
	rawNewest, rawPrevious := shared.NewHistogram(), shared.NewHistogram()
	var total float64
	for _, pair := range c.segments {
		total += pair.newest.Count
		rawNewest.Merge(pair.newest)
		if pair.previous == nil || pair.previous.Count <= 0 {
			continue
		}
		newest.Merge(pair.newest)
		previous.Merge(pair.previous.Scaled(pair.newest.Count / pair.previous.Count))
	}
	if total > 0 && newest.Count >= minMixCoverage*total && len(c.segments) > 1 {
		// The scaled histogram keeps the old release's real sample size for the
		// confidence interval.
		var sample float64
		for _, pair := range c.segments {
			if pair.previous != nil {
				sample += pair.previous.Count
			}
		}
		return newest, previous.Scaled(sample / previous.Count), true
	}
	for _, pair := range c.segments {
		if pair.previous != nil {
			rawPrevious.Merge(pair.previous)
		}
	}
	return rawNewest, rawPrevious, false
}

// regressed compares one pair of distributions. A percentile must be worse by more
// than the allowed percentage and by more than chance explains: the low end of the
// new release's confidence interval has to lie above the high end of the old one's.
func regressed(cfg releaseRegressionConfig, newest, previous *shared.Histogram) (current, baseline float64, fired bool) {
	if newest == nil || previous == nil || newest.Count < float64(cfg.MinCount) || previous.Count < float64(cfg.MinCount) {
		return 0, 0, false
	}
	current, baseline = newest.Aggregate(cfg.Aggregation, 0), previous.Aggregate(cfg.Aggregation, 0)
	if baseline <= 0 || current <= baseline*(1+cfg.ThresholdPercent/100) {
		return current, baseline, false
	}
	if q, percentile := shared.HistogramAggregations[cfg.Aggregation]; percentile {
		low, _ := newest.QuantileInterval(q)
		_, high := previous.QuantileInterval(q)
		if low <= high {
			return current, baseline, false
		}
	}
	return current, baseline, true
}

type releaseFinding struct {
	segment           string
	current, baseline float64
}

func evaluateMetricReleaseRegression(ctx context.Context, rule *models.NotificationRule, projectId uuid.UUID) (*EvalResult, error) {
	cfg, err := parseReleaseRegressionConfig(rule.Config)
	if err != nil {
		return nil, err
	}
	comparison, err := releaseRegression(ctx, cfg, projectId, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	if comparison == nil {
		return &EvalResult{Fired: false}, nil
	}
	var findings []releaseFinding
	adjusted := false
	if len(cfg.GroupBy) == 0 {
		newest, previous, mix := comparison.mixAdjusted()
		adjusted = mix
		if current, baseline, fired := regressed(cfg, newest, previous); fired {
			findings = append(findings, releaseFinding{current: current, baseline: baseline})
		}
	} else {
		for segment, pair := range comparison.segments {
			if current, baseline, fired := regressed(cfg, pair.newest, pair.previous); fired {
				findings = append(findings, releaseFinding{segment: segment, current: current, baseline: baseline})
			}
		}
	}
	// Once per release and segment: the next evaluation finds the same two releases.
	cooldown := time.Duration(rule.CooldownMinutes) * time.Minute
	fresh := findings[:0]
	for _, finding := range findings {
		key := metricSeriesDedupKey(rule.Id, "release:"+comparison.newest+":"+finding.segment)
		if dedup.isDuplicate(key, cooldown) {
			continue
		}
		dedup.record(key)
		fresh = append(fresh, finding)
	}
	if len(fresh) == 0 {
		return &EvalResult{Fired: false}, nil
	}
	slices.SortFunc(fresh, func(a, b releaseFinding) int {
		if c := cmp.Compare(b.current/b.baseline, a.current/a.baseline); c != 0 {
			return c
		}
		return strings.Compare(a.segment, b.segment)
	})
	projectName := getProjectName(ctx, projectId)
	messages := make([]Message, 0, min(len(fresh), maxMetricRuleSeriesNotified))
	for _, finding := range fresh[:min(len(fresh), maxMetricRuleSeriesNotified)] {
		label := ""
		if len(cfg.GroupBy) > 0 {
			label = metricSeriesLabel(cfg.GroupBy, finding.segment)
		}
		messages = append(messages, buildReleaseRegressionMessage(cfg, comparison.newest, comparison.previous, label, finding.current, finding.baseline, adjusted, projectName))
	}
	return &EvalResult{Fired: true, Messages: messages}, nil
}

func buildReleaseRegressionMessage(cfg releaseRegressionConfig, newest, previous, segment string, current, baseline float64, adjusted bool, projectName string) Message {
	change := 100 * (current - baseline) / baseline
	where := ""
	if segment != "" {
		where = " for " + segment
	}
	headline := fmt.Sprintf("Release %s has a %s %s of %.2f%s, %.0f%% worse than release %s (%.2f) over the last %d minutes.",
		newest, cfg.MetricName, cfg.Aggregation, current, where, change, previous, baseline, cfg.LookbackMinutes)
	if adjusted {
		headline += " The older release is weighted to the same mix of devices and operating systems."
	}
	severity := SeverityWarning
	if change > 2*cfg.ThresholdPercent {
		severity = SeverityCritical
	}
	scope := newest
	if segment != "" {
		scope += " " + segment
	}
	return Message{
		Subject:    fmt.Sprintf("[%s] Release %s regressed %s%s by %.0f%%", projectName, newest, cfg.MetricName, where, change),
		Body:       headline,
		Severity:   severity,
		URL:        "/dashboards?preset=24h",
		DedupToken: cfg.MetricName + ":release:" + newest + ":" + segment,
		Email: alertEmail(models.EmailAlert{
			ProjectName: projectName,
			Headline:    headline,
			ScopeLabel:  "Release",
			Scope:       scope,
			Observed:    fmt.Sprintf("%s %.2f", cfg.Aggregation, current),
			Threshold:   fmt.Sprintf("at most %.0f%% above %.2f (release %s)", cfg.ThresholdPercent, baseline, previous),
			WindowMins:  cfg.LookbackMinutes,
		}),
	}
}
