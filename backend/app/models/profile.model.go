package models

import (
	"time"

	"github.com/google/uuid"
)

type ProfileStack struct {
	ProjectId   uuid.UUID `json:"projectId" ch:"project_id"`
	ServiceName string    `json:"serviceName" ch:"service_name"`
	StackHash   uint64    `json:"stackHash" ch:"stack_hash"`
	Stack       []string  `json:"stack" ch:"stack"`
	LastSeen    time.Time `json:"lastSeen" ch:"last_seen"`
}

type ProfileSample struct {
	ProjectId   uuid.UUID         `json:"projectId" ch:"project_id"`
	ProfileId   uuid.UUID         `json:"profileId" ch:"profile_id"`
	ServiceName string            `json:"serviceName" ch:"service_name"`
	Type        string            `json:"type" ch:"type"`
	Unit        string            `json:"unit" ch:"unit"`
	IsGauge     bool              `json:"isGauge" ch:"is_gauge"`
	Start       time.Time         `json:"start" ch:"start_time"`
	End         time.Time         `json:"end" ch:"end_time"`
	StackHash   uint64            `json:"stackHash" ch:"stack_hash"`
	Value       int64             `json:"value" ch:"value"`
	Labels      map[string]string `json:"labels" ch:"labels"`
	ServerName  string            `json:"serverName" ch:"server_name"`
	AppVersion  string            `json:"appVersion" ch:"app_version"`
	TraceId     string            `json:"traceId" ch:"trace_id"`
	SpanId      string            `json:"spanId" ch:"span_id"`
}

type ProfileGroup struct {
	ServiceName  string    `json:"serviceName" ch:"service_name"`
	Type         string    `json:"type" ch:"profile_type"`
	Unit         string    `json:"unit" ch:"unit"`
	IsGauge      bool      `json:"isGauge" ch:"is_gauge"`
	ProfileCount int64     `json:"profileCount" ch:"profile_count"`
	SampleCount  int64     `json:"sampleCount" ch:"sample_count"`
	TotalValue   int64     `json:"totalValue" ch:"total_value"`
	LastSeen     time.Time `json:"lastSeen" ch:"last_seen"`
	Sparkline    []float64 `json:"sparkline"`
}

type ProfileTypeInfo struct {
	Type    string `json:"type" ch:"type"`
	Unit    string `json:"unit" ch:"unit"`
	IsGauge bool   `json:"isGauge" ch:"is_gauge"`
}

type ProfileStackValue struct {
	Stack []string `json:"stack"`
	Value int64    `json:"value"`
}

type ProfileSeriesGroup struct {
	Key    string            `json:"key"`
	Points []TimeSeriesPoint `json:"points"`
}

type Profile struct {
	Id                 uuid.UUID         `json:"id" ch:"id"`
	ProjectId          uuid.UUID         `json:"projectId" ch:"project_id"`
	RecordedAt         time.Time         `json:"recordedAt" ch:"recorded_at"`
	Duration           time.Duration     `json:"duration" ch:"duration"`
	ServiceName        string            `json:"serviceName" ch:"service_name"`
	ProfileType        string            `json:"profileType" ch:"profile_type"`
	Unit               string            `json:"unit" ch:"unit"`
	IsGauge            bool              `json:"isGauge" ch:"is_gauge"`
	SampleCount        uint64            `json:"sampleCount" ch:"sample_count"`
	TotalValue         int64             `json:"totalValue" ch:"total_value"`
	ServerName         string            `json:"serverName" ch:"server_name"`
	AppVersion         string            `json:"appVersion" ch:"app_version"`
	Attributes         map[string]string `json:"attributes" ch:"attributes"`
	StorageKey         string            `json:"storageKey" ch:"storage_key"`
	TraceId            string            `json:"traceId" ch:"trace_id"`
	SpanId             string            `json:"spanId" ch:"span_id"`
	DistributedTraceId *uuid.UUID        `json:"distributedTraceId,omitempty" ch:"distributed_trace_id"`
}
