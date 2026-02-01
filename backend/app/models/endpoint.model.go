package models

import (
	"time"

	"github.com/google/uuid"
)

type Endpoint struct {
	Id        uuid.UUID `json:"id" ch:"id"`
	ProjectId uuid.UUID `json:"projectId" ch:"project_id"`
	// endpoint is the route from the router/does not contain actual params so it's safe to group on it
	Endpoint   string            `json:"endpoint" ch:"endpoint"`
	Duration   time.Duration     `json:"duration" ch:"duration"`
	RecordedAt time.Time         `json:"recordedAt" ch:"recorded_at"`
	StatusCode int16             `json:"statusCode" ch:"status_code"`
	BodySize   int32             `json:"bodySize" ch:"body_size"`
	ClientIP   string            `json:"clientIP" ch:"client_ip"`
	Attributes map[string]string `json:"attributes" ch:"attributes"`
	AppVersion string            `json:"appVersion" ch:"app_version"`
	ServerName string            `json:"serverName" ch:"server_name"`
}

type EndpointStats struct {
	Endpoint    string        `json:"endpoint"`
	Count       uint64        `json:"count"`
	P50Duration time.Duration `json:"p50Duration"`
	P95Duration time.Duration `json:"p95Duration"`
	P99Duration time.Duration `json:"p99Duration"`
	AvgDuration time.Duration `json:"avgDuration"`
	LastSeen    time.Time     `json:"lastSeen"`
	Impact      float64       `json:"impact"` // 0-1 Apdex-based impact score
}

// EndpointDetailStats contains detailed statistics for a specific endpoint
type EndpointDetailStats struct {
	Count          int64   `json:"count"`
	AvgDuration    float64 `json:"avgDuration"`    // in ms
	MedianDuration float64 `json:"medianDuration"` // in ms
	P95Duration    float64 `json:"p95Duration"`    // in ms
	P99Duration    float64 `json:"p99Duration"`    // in ms
	Apdex          float64 `json:"apdex"`          // 0-1 score
	ErrorRate      float64 `json:"errorRate"`      // percentage
	Throughput     float64 `json:"throughput"`     // requests per minute
}

// EndpointTimeSeriesPoint represents a single data point in a time series for endpoint charts
type EndpointTimeSeriesPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Endpoint  string    `json:"endpoint"`
	Value     float64   `json:"value"`
}

// EndpointStackedChartResponse contains the data for rendering a stacked area chart
type EndpointStackedChartResponse struct {
	Endpoints []string                  `json:"endpoints"` // Top 5 + "Other"
	Series    []EndpointTimeSeriesPoint `json:"series"`
}
