package traceway

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// ParentContext returns a context whose active span is the one ctx belongs
// to. Handlers pass their *gin.Context as the context, and gin only looks
// into the request context for non-string keys when ContextWithFallback is
// on, which would also let a client disconnect cancel every query. So the
// span is read from the request gin always exposes under ContextRequestKey.
func ParentContext(ctx context.Context) (context.Context, bool) {
	if ctx == nil {
		return nil, false
	}
	if trace.SpanContextFromContext(ctx).IsValid() {
		return ctx, true
	}
	if req, ok := ctx.Value(gin.ContextRequestKey).(*http.Request); ok && req != nil {
		if span := trace.SpanFromContext(req.Context()); span.SpanContext().IsValid() {
			return trace.ContextWithSpan(ctx, span), true
		}
	}
	return nil, false
}

type Span struct {
	span trace.Span
}

func (s *Span) End() {
	if s != nil {
		s.span.End()
	}
}

func StartSpan(ctx context.Context, name string) *Span {
	parent, ok := ParentContext(ctx)
	if !ok {
		return nil
	}
	_, span := current.Load().tracer.Start(parent, name)
	return &Span{span: span}
}

func SetAttribute(ctx context.Context, key, value string) {
	if span, ok := recordingSpan(ctx); ok {
		span.SetAttributes(attribute.String(key, value))
	}
}

func RunTask(ctx context.Context, name string, fn func(ctx context.Context)) {
	s := current.Load()
	if !s.enabled {
		fn(ctx)
		return
	}
	ctx, span := s.tracer.Start(ctx, name,
		trace.WithNewRoot(),
		trace.WithSpanKind(trace.SpanKindConsumer),
	)
	defer func() {
		if r := recover(); r != nil {
			span.SetStatus(codes.Error, "panic")
			span.End()
			panic(r)
		}
		span.End()
	}()
	fn(ctx)
}
