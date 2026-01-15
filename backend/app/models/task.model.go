package models

import "time"

type Task struct {
	Id         string            `json:"id" ch:"id"`
	ProjectId  string            `json:"projectId" ch:"project_id"`
	TaskName   string            `json:"taskName" ch:"task_name"`
	Duration   time.Duration     `json:"duration" ch:"duration"`
	RecordedAt time.Time         `json:"recordedAt" ch:"recorded_at"`
	ClientIP   string            `json:"clientIP" ch:"client_ip"`
	Scope      map[string]string `json:"scope" ch:"scope"`
	AppVersion string            `json:"appVersion" ch:"app_version"`
	ServerName string            `json:"serverName" ch:"server_name"`
}

type TaskStats struct {
	TaskName    string        `json:"taskName"`
	Count       uint64        `json:"count"`
	P50Duration time.Duration `json:"p50Duration"`
	P95Duration time.Duration `json:"p95Duration"`
	AvgDuration time.Duration `json:"avgDuration"`
	LastSeen    time.Time     `json:"lastSeen"`
}

// TaskDetailStats contains detailed statistics for a specific task
type TaskDetailStats struct {
	Count          int64   `json:"count"`
	AvgDuration    float64 `json:"avgDuration"`    // in ms
	MedianDuration float64 `json:"medianDuration"` // in ms
	P95Duration    float64 `json:"p95Duration"`    // in ms
	P99Duration    float64 `json:"p99Duration"`    // in ms
	Throughput     float64 `json:"throughput"`     // tasks per minute
}
