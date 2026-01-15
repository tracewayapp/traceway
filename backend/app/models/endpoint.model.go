package models

import "time"

type Endpoint struct {
	Id        string `json:"id" ch:"id"`
	ProjectId string `json:"projectId" ch:"project_id"`
	// endpoint is the route from the router/does not contain actual params so it's safe to group on it
	Endpoint   string            `json:"endpoint" ch:"endpoint"`
	Duration   time.Duration     `json:"duration" ch:"duration"`
	RecordedAt time.Time         `json:"recordedAt" ch:"recorded_at"`
	StatusCode int32             `json:"statusCode" ch:"status_code"`
	BodySize   int32             `json:"bodySize" ch:"body_size"`
	ClientIP   string            `json:"clientIP" ch:"client_ip"`
	Scope      map[string]string `json:"scope" ch:"scope"`
	AppVersion string            `json:"appVersion" ch:"app_version"`
	ServerName string            `json:"serverName" ch:"server_name"`
}

type EndpointStats struct {
	Endpoint    string        `json:"endpoint"`
	Count       uint64        `json:"count"`
	P50Duration time.Duration `json:"p50Duration"`
	P95Duration time.Duration `json:"p95Duration"`
	AvgDuration time.Duration `json:"avgDuration"`
	LastSeen    time.Time     `json:"lastSeen"`
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
