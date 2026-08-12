package models

import (
	"time"

	"github.com/google/uuid"
)

type AiTrace struct {
	Id                 uuid.UUID         `json:"id" ch:"id"`
	ProjectId          uuid.UUID         `json:"projectId" ch:"project_id"`
	RecordedAt         time.Time         `json:"recordedAt" ch:"recorded_at"`
	Duration           time.Duration     `json:"duration" ch:"duration"`
	StatusCode         uint8             `json:"statusCode" ch:"status_code"`
	Model              string            `json:"model" ch:"model"`
	ResponseModel      string            `json:"responseModel" ch:"response_model"`
	Provider           string            `json:"provider" ch:"provider"`
	Operation          string            `json:"operation" ch:"operation"`
	InputTokens        int64             `json:"inputTokens" ch:"input_tokens"`
	OutputTokens       int64             `json:"outputTokens" ch:"output_tokens"`
	TotalTokens        int64             `json:"totalTokens" ch:"total_tokens"`
	CachedTokens       int64             `json:"cachedTokens" ch:"cached_tokens"`
	ReasoningTokens    int64             `json:"reasoningTokens" ch:"reasoning_tokens"`
	InputCost          float64           `json:"inputCost" ch:"input_cost"`
	OutputCost         float64           `json:"outputCost" ch:"output_cost"`
	TotalCost          float64           `json:"totalCost" ch:"total_cost"`
	TraceName          string            `json:"traceName" ch:"trace_name"`
	UserId             string            `json:"userId" ch:"user_id"`
	FinishReason       string            `json:"finishReason" ch:"finish_reason"`
	ServerName         string            `json:"serverName" ch:"server_name"`
	AppVersion         string            `json:"appVersion" ch:"app_version"`
	StorageKey         string            `json:"storageKey" ch:"storage_key"`
	Attributes         map[string]string `json:"attributes" ch:"attributes"`
	DistributedTraceId *uuid.UUID        `json:"distributedTraceId,omitempty" ch:"distributed_trace_id"`
	IsRoot             bool              `json:"isRoot" ch:"is_root"`
	ConversationId     string            `json:"conversationId" ch:"conversation_id"`
	ToolCallCount      int64             `json:"toolCallCount" ch:"tool_call_count"`
	ToolNames          []string          `json:"toolNames" ch:"tool_names"`
	Flagged            bool              `json:"flagged" ch:"flagged"`
	FlaggedTerms       []string          `json:"flaggedTerms" ch:"flagged_terms"`
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
	HasRoot         bool          `json:"hasRoot"`
	HasNonRoot      bool          `json:"hasNonRoot"`
}

type AiConversationStats struct {
	ConversationId string    `json:"conversationId"`
	UserId         string    `json:"userId"`
	Turns          int64     `json:"turns"`
	TotalTokens    int64     `json:"totalTokens"`
	TotalCost      float64   `json:"totalCost"`
	ToolCallCount  int64     `json:"toolCallCount"`
	ToolNames      []string  `json:"toolNames"`
	Models         []string  `json:"models"`
	Flagged        bool      `json:"flagged"`
	FlaggedTerms   []string  `json:"flaggedTerms"`
	FirstSeen      time.Time `json:"firstSeen"`
	LastSeen       time.Time `json:"lastSeen"`
}

// AiConversationThresholds carries range-wide outlier cutoffs for the
// conversations list so the frontend can highlight rows beyond the P95.
type AiConversationThresholds struct {
	P95Cost  float64 `json:"p95Cost"`
	P95Turns float64 `json:"p95Turns"`
}

type AiConversationDetailStats struct {
	Turns         int64     `json:"turns"`
	TotalTokens   int64     `json:"totalTokens"`
	TotalCost     float64   `json:"totalCost"`
	ToolCallCount int64     `json:"toolCallCount"`
	AvgDuration   float64   `json:"avgDuration"`
	Models        []string  `json:"models"`
	Flagged       bool      `json:"flagged"`
	FlaggedTerms  []string  `json:"flaggedTerms"`
	UserId        string    `json:"userId"`
	FirstSeen     time.Time `json:"firstSeen"`
	LastSeen      time.Time `json:"lastSeen"`
}

type AiUserStats struct {
	UserId                   string    `json:"userId"`
	ConversationCount        int64     `json:"conversationCount"`
	TotalCalls               int64     `json:"totalCalls"`
	AvgTurns                 float64   `json:"avgTurns"`
	MinTurns                 int64     `json:"minTurns"`
	MedianTurns              float64   `json:"medianTurns"`
	AvgCostPerConversation   float64   `json:"avgCostPerConversation"`
	TotalCost                float64   `json:"totalCost"`
	FlaggedConversationCount int64     `json:"flaggedConversationCount"`
	TotalTokens              int64     `json:"totalTokens"`
	LastSeen                 time.Time `json:"lastSeen"`
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
