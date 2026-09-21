package contract

import (
	"context"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tracewayapp/traceway/backend/app/controllers/clientcontrollers"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry"
)

// Seeded ids, set by seedTelemetry and used to address the by-id detail
// endpoints. Every node belongs to one trace, and its spans hang together the
// way a real request's do, so the distributed-trace endpoint returns a nested
// graph and the exception finds its endpoint through the span above it.
var (
	seedEndpointID  uuid.UUID
	seedTaskID      uuid.UUID
	seedExceptionID uuid.UUID
	seedSessionID   uuid.UUID
	seedAiTraceID   uuid.UUID
	seedLogID       uuid.UUID
	seedTraceID     string // the trace id shared across nodes, 32 hex
	seedHash        string // exception grouping hash (computed, not assigned)

	seedMetricName = "contract.metric"
	seedMetricVal  = 42.5
)

const (
	seedLinkedTraceID  = "4bf92f3577b34da6a3ce929d0e0e4736"
	seedEndpointSpanID = "e1e1e1e1e1e1e1e1"
	seedChildSpanID    = "c2c2c2c2c2c2c2c2"
	seedTaskSpanID     = "a3a3a3a3a3a3a3a3"
	seedTaskChildID    = "b4b4b4b4b4b4b4b4"
	seedAiSpanID       = "d5d5d5d5d5d5d5d5"
)

// seedTelemetry inserts exactly one row of every telemetry type the CLI reads,
// directly via the backend repositories (which, in the default build, write to
// the SQLite telemetry DB synchronously). Everything is timestamped at `at` and
// linked by a shared trace id.
func seedTelemetry(ctx context.Context, projectIDStr string, at time.Time) error {
	pid, err := uuid.Parse(projectIDStr)
	if err != nil {
		return fmt.Errorf("parse project id: %w", err)
	}

	seedEndpointID = uuid.New()
	seedTaskID = uuid.New()
	seedExceptionID = uuid.New()
	seedSessionID = uuid.New()
	seedAiTraceID = uuid.New()
	seedLogID = uuid.New()
	traceUUID := uuid.New()
	seedTraceID = hex.EncodeToString(traceUUID[:])

	stack := "ContractError: seeded failure\n\tat contract.go:42"
	seedHash = clientcontrollers.ComputeExceptionHash(stack, false)

	endpoint := models.Endpoint{
		Id:         seedEndpointID,
		ProjectId:  pid,
		Endpoint:   "GET /api/contract",
		Duration:   123 * time.Millisecond,
		RecordedAt: at,
		StatusCode: 200,
		BodySize:   1024,
		ClientIP:   "127.0.0.1",
		Attributes: map[string]string{},
		AppVersion: "1.0.0",
		ServerName: "contract-host",
		TraceId:    seedTraceID,
		SpanId:     seedEndpointSpanID,
		IsRoot:     true,
	}
	if err := telemetry.EndpointRepository.InsertAsync(ctx, []models.Endpoint{endpoint}); err != nil {
		return fmt.Errorf("endpoint: %w", err)
	}

	task := models.Task{
		Id:           seedTaskID,
		ProjectId:    pid,
		TaskName:     "contract-task",
		Duration:     250 * time.Millisecond,
		RecordedAt:   at,
		ClientIP:     "127.0.0.1",
		Attributes:   map[string]string{},
		AppVersion:   "1.0.0",
		ServerName:   "contract-host",
		TraceId:      seedTraceID,
		SpanId:       seedTaskSpanID,
		ParentSpanId: seedChildSpanID,
	}
	if err := telemetry.TaskRepository.InsertAsync(ctx, []models.Task{task}); err != nil {
		return fmt.Errorf("task: %w", err)
	}

	// Recorded on the span under the endpoint, with no trace type: the way an
	// exception arrives from a span that is not itself an endpoint or a task.
	exception := models.ExceptionStackTrace{
		Id:            seedExceptionID,
		ProjectId:     pid,
		TraceId:       seedTraceID,
		SpanId:        seedChildSpanID,
		ExceptionHash: seedHash,
		StackTrace:    stack,
		RecordedAt:    at,
		Attributes:    map[string]string{},
		AppVersion:    "1.0.0",
		ServerName:    "contract-host",
		IsMessage:     false,
		SessionId:     &seedSessionID,
	}
	if err := telemetry.ExceptionStackTraceRepository.InsertAsync(ctx, []models.ExceptionStackTrace{exception}); err != nil {
		return fmt.Errorf("exception: %w", err)
	}

	span := func(id, parent, name string, kind int32) models.Span {
		return models.Span{
			ProjectId: pid, TraceId: seedTraceID, SpanId: id, ParentSpanId: parent,
			Name: name, StartTime: at, Duration: 100 * time.Millisecond, RecordedAt: at,
			SpanKind: kind, ServiceName: "contract-svc", Attributes: map[string]string{"contract.key": "value"},
		}
	}
	spans := []models.Span{
		span(seedEndpointSpanID, "", "GET /api/contract", 2),
		span(seedChildSpanID, seedEndpointSpanID, "contract-span", 1),
		span(seedTaskSpanID, seedChildSpanID, "contract-task", 5),
		span(seedTaskChildID, seedTaskSpanID, "contract-task-span", 1),
		span(seedAiSpanID, seedEndpointSpanID, "chat gpt-4", 3),
	}
	if err := telemetry.SpanRepository.InsertAsync(ctx, spans); err != nil {
		return fmt.Errorf("span: %w", err)
	}

	metric := models.MetricPoint{
		ProjectId:  pid,
		Name:       seedMetricName,
		Value:      seedMetricVal,
		Tags:       map[string]string{},
		RecordedAt: at,
	}
	if err := telemetry.MetricPointRepository.InsertAsync(ctx, []models.MetricPoint{metric}); err != nil {
		return fmt.Errorf("metric: %w", err)
	}

	session := models.Session{
		Id:         seedSessionID,
		ProjectId:  pid,
		StartedAt:  at,
		Duration:   60000,
		ClientIP:   "127.0.0.1",
		Attributes: map[string]string{},
		AppVersion: "1.0.0",
		ServerName: "contract-host",
		TraceId:    seedTraceID,
	}
	if err := telemetry.SessionRepository.Upsert(ctx, []models.Session{session}); err != nil {
		return fmt.Errorf("session: %w", err)
	}

	aiTrace := models.AiTrace{
		Id:             seedAiTraceID,
		ProjectId:      pid,
		RecordedAt:     at,
		Duration:       500 * time.Millisecond,
		StatusCode:     200,
		Model:          "gpt-4",
		ResponseModel:  "gpt-4",
		Provider:       "openai",
		Operation:      "chat",
		InputTokens:    10,
		OutputTokens:   20,
		TotalTokens:    30,
		TotalCost:      0.0012,
		TraceName:      "contract-ai",
		Attributes:     map[string]string{},
		TraceId:        seedTraceID,
		SpanId:         seedAiSpanID,
		ParentSpanId:   seedEndpointSpanID,
		ConversationId: "contract-conversation",
		ToolCallCount:  1,
		ToolNames:      []string{"contract_tool"},
		Flagged:        true,
		FlaggedTerms:   []string{"contract-term"},
	}
	if err := telemetry.AiTraceRepository.InsertAsync(ctx, []models.AiTrace{aiTrace}); err != nil {
		return fmt.Errorf("ai trace: %w", err)
	}

	logRecord := models.LogRecord{
		Id:                 seedLogID,
		ProjectId:          pid,
		Timestamp:          at,
		SeverityText:       "INFO",
		SeverityNumber:     9,
		ServiceName:        "contract-svc",
		Body:               "contract log line",
		ResourceAttributes: map[string]string{},
		ScopeAttributes:    map[string]string{},
		LogAttributes:      map[string]string{},
	}
	if err := telemetry.LogRecordRepository.InsertAsync(ctx, []models.LogRecord{logRecord}); err != nil {
		return fmt.Errorf("log: %w", err)
	}

	return nil
}
