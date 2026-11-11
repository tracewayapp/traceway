package traceway

import (
	"context"
	"fmt"
	"net/http"
	"sync/atomic"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/attribute"
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

type statementSpansKey struct{}

// WithoutStatementSpans keeps the database statements run under ctx out of
// the trace, while the request itself is still traced.
func WithoutStatementSpans(ctx context.Context) context.Context {
	return context.WithValue(ctx, statementSpansKey{}, true)
}

// StatementParent is ParentContext for a database statement. It reports no
// parent when the request opted out with WithoutStatementSpans.
func StatementParent(ctx context.Context) (context.Context, bool) {
	parent, ok := ParentContext(ctx)
	if !ok || ctx.Value(statementSpansKey{}) != nil {
		return nil, false
	}
	if req, ok := ctx.Value(gin.ContextRequestKey).(*http.Request); ok && req != nil && req.Context().Value(statementSpansKey{}) != nil {
		return nil, false
	}
	return parent, true
}

type Span struct {
	span trace.Span
}

func (s *Span) End() {
	if s != nil {
		s.span.End()
	}
}

// StartSpan starts a child of ctx's span. Work that belongs under it, such as
// its database statements, must run with the returned context.
func StartSpan(ctx context.Context, name string) (context.Context, *Span) {
	parent, ok := ParentContext(ctx)
	if !ok {
		return ctx, nil
	}
	spanCtx, span := current.Load().tracer.Start(parent, name)
	return spanCtx, &Span{span: span}
}

func SetAttribute(ctx context.Context, key, value string) {
	if span, ok := recordingSpan(ctx); ok {
		span.SetAttributes(attribute.String(key, value))
	}
}

type taskKey struct{}

type taskState struct {
	idle atomic.Bool
}

// SkipTask marks the running task as a tick that found nothing to do. The
// task is then not reported, and neither are its statements. Outside a task
// it does nothing.
func SkipTask(ctx context.Context) {
	if task, ok := ctx.Value(taskKey{}).(*taskState); ok {
		task.idle.Store(true)
	}
}

// recordedPanic carries a panic RunTask already recorded on its task, so a
// Recover further up the goroutine does not report it a second time.
type recordedPanic struct {
	value any
}

func (p recordedPanic) Error() string {
	return fmt.Sprint(p.value)
}

func RunTask(ctx context.Context, name string, fn func(ctx context.Context)) {
	s := current.Load()
	if !s.enabled {
		fn(ctx)
		return
	}
	task := &taskState{}
	ctx, span := s.tracer.Start(context.WithValue(ctx, taskKey{}, task), name,
		trace.WithNewRoot(),
		trace.WithSpanKind(trace.SpanKindConsumer),
	)
	defer func() {
		if r := recover(); r != nil {
			if _, recorded := r.(recordedPanic); !recorded {
				RecordPanic(span, r)
				r = recordedPanic{value: r}
			}
			span.End()
			panic(r)
		}
		if task.idle.Load() {
			span.SetAttributes(attribute.Bool(idleTaskAttribute, true))
		}
		span.End()
	}()
	fn(ctx)
}
