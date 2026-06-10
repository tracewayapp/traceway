package otelprocessor

import (
	"context"

	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func (p *symbolicatorProcessor) processTraces(ctx context.Context, td ptrace.Traces) (ptrace.Traces, error) {
	resourceSpans := td.ResourceSpans()
	for i := 0; i < resourceSpans.Len(); i++ {
		rs := resourceSpans.At(i)
		resourceAttrs := rs.Resource().Attributes()
		scopeSpans := rs.ScopeSpans()
		for j := 0; j < scopeSpans.Len(); j++ {
			spans := scopeSpans.At(j).Spans()
			for k := 0; k < spans.Len(); k++ {
				span := spans.At(k)
				p.processRecord(ctx, span.Attributes(), resourceAttrs)
				events := span.Events()
				for l := 0; l < events.Len(); l++ {
					p.processRecord(ctx, events.At(l).Attributes(), resourceAttrs)
				}
			}
		}
	}
	return td, nil
}

func (p *symbolicatorProcessor) processLogs(ctx context.Context, ld plog.Logs) (plog.Logs, error) {
	resourceLogs := ld.ResourceLogs()
	for i := 0; i < resourceLogs.Len(); i++ {
		rl := resourceLogs.At(i)
		resourceAttrs := rl.Resource().Attributes()
		scopeLogs := rl.ScopeLogs()
		for j := 0; j < scopeLogs.Len(); j++ {
			records := scopeLogs.At(j).LogRecords()
			for k := 0; k < records.Len(); k++ {
				p.processRecord(ctx, records.At(k).Attributes(), resourceAttrs)
			}
		}
	}
	return ld, nil
}
