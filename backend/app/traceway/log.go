package traceway

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/attribute"
	otellog "go.opentelemetry.io/otel/log"
)

func EmitLog(ctx context.Context, severity otellog.Severity, msg string) {
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
	s.logger.Emit(ctx, record)
}
