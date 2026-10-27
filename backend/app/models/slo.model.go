package models

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

const (
	SLIKindEndpoint = "endpoint"
	SLIKindSpan     = "span"
	SLIKindTask     = "task"
	SLIKindMonitor  = "monitor"
	SLIKindMetric   = "metric"
	SLIKindSession  = "session"

	SLOStatusOk       = "ok"
	SLOStatusBreached = "breached"
	SLOStatusNoData   = "no_data"

	SLOGoodAvailability = "availability"
	SLOGoodLatency      = "latency"

	SLOMaxSpanAttributeFilters = 10
	SLOMaxMonitors             = 50
	SLOMaxMetricTagFilters     = 20
)

var (
	SLOWindowDays         = []int{7, 30, 90}
	SLOMetricSliceMinutes = []int{1, 5, 15, 60}
	SLOMetricAggregations = map[string]bool{"avg": true, "min": true, "max": true, "sum": true, "last": true}
	SLOMetricOperators    = map[string]bool{"gt": true, "gte": true, "lt": true, "lte": true}
)

type SLO struct {
	Id              int        `json:"id" lit:"id"`
	ProjectId       uuid.UUID  `json:"projectId" lit:"project_id"`
	Name            string     `json:"name" lit:"name"`
	Description     string     `json:"description" lit:"description"`
	SliKind         string     `json:"sliKind" lit:"sli_kind"`
	SliConfig       JSONText   `json:"sliConfig" lit:"sli_config"`
	TargetPercent   float64    `json:"targetPercent" lit:"target_percent"`
	WindowDays      int        `json:"windowDays" lit:"window_days"`
	BucketMinutes   int        `json:"bucketMinutes" lit:"bucket_minutes"`
	Enabled         bool       `json:"enabled" lit:"enabled"`
	Status          string     `json:"status" lit:"status"`
	CurrentSli      *float64   `json:"currentSli" lit:"current_sli"`
	BudgetRemaining *float64   `json:"budgetRemaining" lit:"budget_remaining"`
	BurnRate1h      *float64   `json:"burnRate1h" lit:"burn_rate_1h"`
	BurnRate6h      *float64   `json:"burnRate6h" lit:"burn_rate_6h"`
	ComputedAt      *time.Time `json:"computedAt" lit:"computed_at"`
	HistoryFrom     *time.Time `json:"historyFrom" lit:"history_from"`
	LastBucketAt    *time.Time `json:"lastBucketAt" lit:"last_bucket_at"`
	SliSince        *time.Time `json:"sliSince" lit:"sli_since"`
	GapFrom         *time.Time `json:"-" lit:"gap_from"`
	GapTo           *time.Time `json:"-" lit:"gap_to"`
	Revision        int        `json:"revision" lit:"revision"`
	CreatedBy       *int       `json:"createdBy" lit:"created_by"`
	CreatedAt       time.Time  `json:"createdAt" lit:"created_at"`
	UpdatedAt       time.Time  `json:"updatedAt" lit:"updated_at"`
}

func (s *SLO) Target() float64 {
	return s.TargetPercent / 100
}

func (s *SLO) BucketDuration() time.Duration {
	if s.BucketMinutes < 1 {
		return time.Minute
	}
	return time.Duration(s.BucketMinutes) * time.Minute
}

func (s *SLO) MissingCountsFrom() time.Time {
	if s.SliSince != nil {
		return s.SliSince.UTC()
	}
	return s.CreatedAt.UTC()
}

func (s *SLO) Window() time.Duration {
	return time.Duration(s.WindowDays) * 24 * time.Hour
}

type SLOSnapshot struct {
	Status          string
	CurrentSli      *float64
	BudgetRemaining *float64
	BurnRate1h      *float64
	BurnRate6h      *float64
	ComputedAt      time.Time
	HistoryFrom     *time.Time
	LastBucketAt    *time.Time
	GapFrom         *time.Time
	GapTo           *time.Time
}

type SLOFreshness struct {
	Id           int        `lit:"id"`
	WindowDays   int        `lit:"window_days"`
	ComputedAt   *time.Time `lit:"computed_at"`
	HistoryFrom  *time.Time `lit:"history_from"`
	LastBucketAt *time.Time `lit:"last_bucket_at"`
	GapFrom      *time.Time `lit:"gap_from"`
	CreatedAt    time.Time  `lit:"created_at"`
}

type NotificationRuleState struct {
	Id        int        `json:"id" lit:"id"`
	RuleId    int        `json:"ruleId" lit:"rule_id"`
	Firing    bool       `json:"firing" lit:"firing"`
	Since     *time.Time `json:"since" lit:"since"`
	UpdatedAt time.Time  `json:"updatedAt" lit:"updated_at"`
}

type EndpointSLIConfig struct {
	Endpoint           string  `json:"endpoint"`
	Good               string  `json:"good"`
	LatencyThresholdMs float64 `json:"latencyThresholdMs"`
}

type SpanSLIAttribute struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type SpanSLIConfig struct {
	ServiceName        string             `json:"serviceName"`
	Name               string             `json:"name"`
	Kind               *int32             `json:"kind"`
	Attributes         []SpanSLIAttribute `json:"attributes"`
	Good               string             `json:"good"`
	LatencyThresholdMs float64            `json:"latencyThresholdMs"`
}

type TaskSLIConfig struct {
	TaskName string `json:"taskName"`
}

type MonitorSLIConfig struct {
	CheckIds []int `json:"checkIds"`
}

type SessionSLIConfig struct {
	AppVersion      string `json:"appVersion"`
	IncludeMessages bool   `json:"includeMessages"`
}

type MetricSLIConfig struct {
	MetricName   string            `json:"metricName"`
	Tags         map[string]string `json:"tags"`
	Aggregation  string            `json:"aggregation"`
	Operator     string            `json:"operator"`
	Threshold    float64           `json:"threshold"`
	MissingIsBad bool              `json:"missingIsBad"`
}

type SLIConfig struct {
	Endpoint *EndpointSLIConfig
	Span     *SpanSLIConfig
	Task     *TaskSLIConfig
	Monitor  *MonitorSLIConfig
	Metric   *MetricSLIConfig
	Session  *SessionSLIConfig
}

func ParseSLIConfig(kind string, raw []byte) (*SLIConfig, error) {
	if len(raw) == 0 {
		raw = []byte("{}")
	}
	cfg := &SLIConfig{}
	var err error
	switch kind {
	case SLIKindEndpoint:
		cfg.Endpoint = &EndpointSLIConfig{}
		err = json.Unmarshal(raw, cfg.Endpoint)
	case SLIKindSpan:
		cfg.Span = &SpanSLIConfig{}
		err = json.Unmarshal(raw, cfg.Span)
	case SLIKindTask:
		cfg.Task = &TaskSLIConfig{}
		err = json.Unmarshal(raw, cfg.Task)
	case SLIKindMonitor:
		cfg.Monitor = &MonitorSLIConfig{}
		err = json.Unmarshal(raw, cfg.Monitor)
	case SLIKindMetric:
		cfg.Metric = &MetricSLIConfig{}
		err = json.Unmarshal(raw, cfg.Metric)
	case SLIKindSession:
		cfg.Session = &SessionSLIConfig{}
		err = json.Unmarshal(raw, cfg.Session)
	default:
		return nil, fmt.Errorf("unknown SLI kind %q", kind)
	}
	if err != nil {
		return nil, fmt.Errorf("invalid %s SLI config: %w", kind, err)
	}
	return cfg, nil
}

func ValidSLIKind(kind string) bool {
	switch kind {
	case SLIKindEndpoint, SLIKindSpan, SLIKindTask, SLIKindMonitor, SLIKindMetric, SLIKindSession:
		return true
	}
	return false
}
