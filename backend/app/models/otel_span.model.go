package models

import (
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
)

type OtelSpan struct {
	Span
	OTLP    *tracepb.Span          `json:"-"`
	Context *tracepb.ResourceSpans `json:"-"`
}
