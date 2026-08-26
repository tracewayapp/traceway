package telemetry

import (
	"math"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/models"
)

func assertApproxEqual(t *testing.T, name string, got, want, tolerance float64) {
	t.Helper()
	if math.Abs(got-want) > tolerance {
		t.Errorf("%s: got %v, want %v (tolerance %v)", name, got, want, tolerance)
	}
}

func makeEndpoint(projectId uuid.UUID, endpoint string, duration time.Duration, statusCode int16, recordedAt time.Time) models.Endpoint {
	return models.Endpoint{
		Id:         uuid.New(),
		ProjectId:  projectId,
		Endpoint:   endpoint,
		Duration:   duration,
		RecordedAt: recordedAt,
		StatusCode: statusCode,
		BodySize:   100,
		ClientIP:   "127.0.0.1",
		AppVersion: "1.0.0",
		ServerName: "test-server",
	}
}

func makeTask(projectId uuid.UUID, taskName string, duration time.Duration, recordedAt time.Time) models.Task {
	return models.Task{
		Id:         uuid.New(),
		ProjectId:  projectId,
		TaskName:   taskName,
		Duration:   duration,
		RecordedAt: recordedAt,
		ClientIP:   "127.0.0.1",
		AppVersion: "1.0.0",
		ServerName: "test-server",
	}
}

func makeTaskWithRoot(projectId uuid.UUID, taskName string, duration time.Duration, recordedAt time.Time, isRoot bool) models.Task {
	t := makeTask(projectId, taskName, duration, recordedAt)
	t.IsRoot = isRoot
	return t
}

func makeAiTrace(projectId uuid.UUID, traceName string, duration time.Duration, totalTokens int64, totalCost float64, recordedAt time.Time) models.AiTrace {
	return models.AiTrace{
		Id:           uuid.New(),
		ProjectId:    projectId,
		RecordedAt:   recordedAt,
		Duration:     duration,
		StatusCode:   200,
		Model:        "test-model",
		Provider:     "test-provider",
		Operation:    "chat",
		InputTokens:  totalTokens / 2,
		OutputTokens: totalTokens - totalTokens/2,
		TotalTokens:  totalTokens,
		TotalCost:    totalCost,
		TraceName:    traceName,
		ServerName:   "test-server",
		AppVersion:   "1.0.0",
		IsRoot:       true,
	}
}

func makeAiTraceWithRoot(projectId uuid.UUID, traceName string, duration time.Duration, totalTokens int64, totalCost float64, recordedAt time.Time, isRoot bool) models.AiTrace {
	tr := makeAiTrace(projectId, traceName, duration, totalTokens, totalCost, recordedAt)
	tr.IsRoot = isRoot
	return tr
}

func makeException(projectId uuid.UUID, hash, stackTrace string, recordedAt time.Time) models.ExceptionStackTrace {
	return models.ExceptionStackTrace{
		Id:            uuid.New(),
		ProjectId:     projectId,
		TraceType:     "endpoint",
		ExceptionHash: hash,
		StackTrace:    stackTrace,
		RecordedAt:    recordedAt,
		AppVersion:    "1.0.0",
		ServerName:    "test-server",
	}
}

func makeSpan(projectId, traceId uuid.UUID, name string, startTime time.Time, duration time.Duration) models.Span {
	return models.Span{
		Id:         uuid.New(),
		TraceId:    traceId,
		ProjectId:  projectId,
		Name:       name,
		StartTime:  startTime,
		Duration:   duration,
		RecordedAt: startTime,
	}
}

func makeSessionRecording(projectId, exceptionId uuid.UUID, filePath string, recordedAt time.Time) models.SessionRecording {
	return models.SessionRecording{
		Id:          uuid.New(),
		ProjectId:   projectId,
		ExceptionId: exceptionId,
		FilePath:    filePath,
		RecordedAt:  recordedAt,
	}
}

func makeMetricPoint(projectId uuid.UUID, name string, value float64, tags map[string]string, recordedAt time.Time) models.MetricPoint {
	if tags == nil {
		tags = map[string]string{}
	}
	return models.MetricPoint{
		ProjectId:  projectId,
		Name:       name,
		Value:      value,
		Tags:       tags,
		RecordedAt: recordedAt,
	}
}

func makeFiredNotification(projectId uuid.UUID, ruleName, status string, firedAt time.Time) FiredNotification {
	return FiredNotification{
		ProjectId:   projectId,
		RuleId:      1,
		RuleType:    "event",
		RuleName:    ruleName,
		ChannelType: "slack",
		ChannelName: "alerts",
		Severity:    "warning",
		Subject:     "Test Alert",
		Body:        "Test alert body",
		Status:      status,
		Endpoint:    "GET /api/test",
		FiredAt:     firedAt,
	}
}

func truncateMs(t time.Time) time.Time {
	return t.Truncate(time.Millisecond)
}
