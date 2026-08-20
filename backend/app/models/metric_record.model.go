package models

import (
	"time"

	"github.com/google/uuid"
)

type MetricPoint struct {
	ProjectId  uuid.UUID         `json:"projectId" ch:"project_id"`
	Name       string            `json:"name" ch:"name"`
	Value      float64           `json:"value" ch:"value"`
	Tags       map[string]string `json:"tags" ch:"tags"`
	RecordedAt time.Time         `json:"recordedAt" ch:"recorded_at"`
}

type ServerLatestPoint struct {
	ServerName     string    `json:"serverName"`
	Value          float64   `json:"value"`
	LastReportedAt time.Time `json:"lastReportedAt"`
}

type DiscoveredMetric struct {
	Name       string   `json:"name"`
	TagKeys    []string `json:"tagKeys"`
	MetricType string   `json:"metricType,omitempty"`
	Unit       string   `json:"unit,omitempty"`
}

const (
	MetricNameMemoryUsage  = "mem.used"
	MetricNameMemoryTotal  = "mem.total"
	MetricNameCpuUsage     = "cpu.used_pcnt"
	MetricNameGoRoutines   = "go.go_routines"
	MetricNameHeapObjects  = "go.heap_objects"
	MetricNameNumGC        = "go.num_gc"
	MetricNameGCPauseTotal = "go.gc_pause"
	// other metric names are custom and added by the clients
)
