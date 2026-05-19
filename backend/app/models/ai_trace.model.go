package models

import (
	"time"

	"github.com/google/uuid"
)

type AiTrace struct {
	Id                 uuid.UUID         `json:"id" ch:"id"`
	ProjectId          uuid.UUID         `json:"projectId" ch:"project_id"`
	DistributedTraceId *uuid.UUID        `json:"distributedTraceId,omitempty" ch:"distributed_trace_id"`
	RecordedAt      time.Time         `json:"recordedAt" ch:"recorded_at"`
	Duration        time.Duration     `json:"duration" ch:"duration"`
	StatusCode      uint8             `json:"statusCode" ch:"status_code"`
	Model           string            `json:"model" ch:"model"`
	ResponseModel   string            `json:"responseModel" ch:"response_model"`
	Provider        string            `json:"provider" ch:"provider"`
	Operation       string            `json:"operation" ch:"operation"`
	InputTokens     int64             `json:"inputTokens" ch:"input_tokens"`
	OutputTokens    int64             `json:"outputTokens" ch:"output_tokens"`
	TotalTokens     int64             `json:"totalTokens" ch:"total_tokens"`
	CachedTokens    int64             `json:"cachedTokens" ch:"cached_tokens"`
	ReasoningTokens int64             `json:"reasoningTokens" ch:"reasoning_tokens"`
	InputCost       float64           `json:"inputCost" ch:"input_cost"`
	OutputCost      float64           `json:"outputCost" ch:"output_cost"`
	TotalCost       float64           `json:"totalCost" ch:"total_cost"`
	TraceName       string            `json:"traceName" ch:"trace_name"`
	UserId          string            `json:"userId" ch:"user_id"`
	FinishReason    string            `json:"finishReason" ch:"finish_reason"`
	ServerName      string            `json:"serverName" ch:"server_name"`
	AppVersion      string            `json:"appVersion" ch:"app_version"`
	StorageKey      string            `json:"storageKey" ch:"storage_key"`
	Attributes      map[string]string `json:"attributes" ch:"attributes"`
}

type AiTraceStats struct {
	TraceName       string        `json:"traceName"`
	Count           uint64        `json:"count"`
	P50Duration     time.Duration `json:"p50Duration"`
	P95Duration     time.Duration `json:"p95Duration"`
	AvgDuration     time.Duration `json:"avgDuration"`
	TotalTokens     int64         `json:"totalTokens"`
	TotalCost       float64       `json:"totalCost"`
	AvgInputTokens  float64       `json:"avgInputTokens"`
	AvgOutputTokens float64       `json:"avgOutputTokens"`
	LastSeen        time.Time     `json:"lastSeen"`
}

type AiTraceDetailStats struct {
	Count           int64   `json:"count"`
	AvgDuration     float64 `json:"avgDuration"`
	MedianDuration  float64 `json:"medianDuration"`
	P95Duration     float64 `json:"p95Duration"`
	TotalTokens     int64   `json:"totalTokens"`
	TotalCost       float64 `json:"totalCost"`
	AvgInputTokens  float64 `json:"avgInputTokens"`
	AvgOutputTokens float64 `json:"avgOutputTokens"`
	Throughput      float64 `json:"throughput"`
}
