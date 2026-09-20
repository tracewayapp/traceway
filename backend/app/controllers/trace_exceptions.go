package controllers

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry"
)

// The exceptions of an endpoint, a task or an AI trace are the ones recorded on its own span or on a span below it.
func findOccurrenceExceptions(ctx context.Context, projectId uuid.UUID, traceId, spanId string, recordedAt *time.Time, spans []models.Span) ([]models.ExceptionStackTrace, error) {
	if traceId == "" {
		return nil, nil
	}
	exceptions, err := telemetry.ExceptionStackTraceRepository.FindAllByTraceId(ctx, projectId, traceId, recordedAt)
	if err != nil {
		return nil, err
	}
	subtree := map[string]bool{spanId: true}
	for _, span := range spans {
		subtree[span.SpanId] = true
	}
	kept := exceptions[:0]
	for _, exception := range exceptions {
		if subtree[exception.SpanId] {
			kept = append(kept, exception)
		}
	}
	return kept, nil
}
