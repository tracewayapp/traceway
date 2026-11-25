package traceway

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/attribute"
	otellog "go.opentelemetry.io/otel/log"
)

func EmitLog(ctx context.Context, severity otellog.Severity, msg string) {
	EmitLogFields(ctx, severity, msg, nil)
}

// EmitLogFields adds fields as attributes of the record, so the line can be filtered by them.
func EmitLogFields(ctx context.Context, severity otellog.Severity, msg string, fields map[string]string) {
	s := current.Load()
	if !s.enabled {
		return
	}
	if parent, ok := ParentContext(ctx); ok {
		ctx = parent
	} else {
		ctx = context.Background()
	}
	var record otellog.Record
	now := time.Now()
	record.SetTimestamp(now)
	record.SetObservedTimestamp(now)
	record.SetSeverity(severity)
	record.SetSeverityText(severity.String())
	record.SetBody(attribute.StringValue(msg))
	for key, value := range fields {
		record.AddAttributes(attribute.String(key, value))
	}
	s.logger.Emit(ctx, record)
}
