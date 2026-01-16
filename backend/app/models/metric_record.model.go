package models

import (
	"time"

	"github.com/google/uuid"
)

type MetricRecord struct {
	ProjectId  uuid.UUID `json:"projectId" ch:"project_id"`
	Name       string    `json:"name" ch:"name"`
	Value      float64   `json:"value" ch:"value"`
	RecordedAt time.Time `json:"recordedAt" ch:"recorded_at"`
	ServerName string    `json:"serverName" ch:"server_name"`
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
