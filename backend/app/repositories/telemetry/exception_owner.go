package telemetry

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/models"
)

// ExceptionOwner is the endpoint, task or AI trace an exception happened in.
type ExceptionOwner struct {
	TraceType  string        `json:"traceType"`
	Id         uuid.UUID     `json:"id"`
	Name       string        `json:"name"`
	StatusCode int16         `json:"statusCode"`
	Duration   time.Duration `json:"duration"`
	RecordedAt time.Time     `json:"recordedAt"`
	TraceId    string        `json:"traceId"`
}

const maxExceptionOwnerDepth = 10000

// FindExceptionOwner finds the endpoint, task or AI trace of the exception's project whose span the exception was
// recorded on, or the nearest one above that span. Ingest stores the exception's trace and span and nothing more, so
// the owner is worked out here, when somebody looks. nil means the exception stands alone.
func FindExceptionOwner(ctx context.Context, exception models.ExceptionStackTrace) (*ExceptionOwner, error) {
	if exception.TraceId == "" || exception.SpanId == "" {
		return nil, nil
	}
	traceIds := []string{exception.TraceId}
	projectIds := []uuid.UUID{exception.ProjectId}
	at := exception.RecordedAt

	owners := map[string]*ExceptionOwner{}
	endpoints, err := EndpointRepository.FindByTraceIds(ctx, traceIds, projectIds, &at)
	if err != nil {
		return nil, err
	}
	tasks, err := TaskRepository.FindByTraceIds(ctx, traceIds, projectIds, &at)
	if err != nil {
		return nil, err
	}
	aiTraces, err := AiTraceRepository.FindByTraceIds(ctx, traceIds, projectIds, &at)
	if err != nil {
		return nil, err
	}
	for _, ai := range aiTraces {
		if ai.TraceId == exception.TraceId {
			owners[ai.SpanId] = &ExceptionOwner{TraceType: "ai_trace", Id: ai.Id, Name: ai.TraceName, Duration: ai.Duration, RecordedAt: ai.RecordedAt, TraceId: ai.TraceId}
		}
	}
	for _, task := range tasks {
		if task.TraceId == exception.TraceId {
			owners[task.SpanId] = &ExceptionOwner{TraceType: "task", Id: task.Id, Name: task.TaskName, Duration: task.Duration, RecordedAt: task.RecordedAt, TraceId: task.TraceId}
		}
	}
	for _, endpoint := range endpoints {
		if endpoint.TraceId == exception.TraceId {
			owners[endpoint.SpanId] = &ExceptionOwner{TraceType: "endpoint", Id: endpoint.Id, Name: endpoint.Endpoint, StatusCode: endpoint.StatusCode, Duration: endpoint.Duration, RecordedAt: endpoint.RecordedAt, TraceId: endpoint.TraceId}
		}
	}
	if len(owners) == 0 {
		return nil, nil
	}
	if owner := owners[exception.SpanId]; owner != nil {
		return owner, nil
	}

	parents, err := SpanRepository.FindTraceParents(ctx, projectIds, exception.TraceId, at)
	if err != nil {
		return nil, err
	}
	visited := map[string]bool{exception.SpanId: true}
	for current, depth := parents[exception.SpanId], 0; current != "" && !visited[current] && depth < maxExceptionOwnerDepth; current, depth = parents[current], depth+1 {
		if owner := owners[current]; owner != nil {
			return owner, nil
		}
		visited[current] = true
	}
	return nil, nil
}
