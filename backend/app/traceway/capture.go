package traceway

import (
	"context"
	"errors"
	"runtime"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

func CaptureException(err error) {
	s := current.Load()
	if !s.enabled || err == nil {
		return
	}
	s.recordStandalone(exceptionAttributes(err, captureStack(1)), err.Error())
}

func CaptureExceptionContext(ctx context.Context, err error) {
	s := current.Load()
	if !s.enabled || err == nil {
		return
	}
	attrs := exceptionAttributes(err, captureStack(1))
	if span, ok := recordingSpan(ctx); ok {
		addException(span, attrs, err.Error())
		return
	}
	s.recordStandalone(attrs, err.Error())
}

func CaptureMessage(msg string) {
	s := current.Load()
	if !s.enabled || msg == "" {
		return
	}
	s.recordStandalone([]attribute.KeyValue{attribute.String("exception.message", msg)}, msg)
}

func Recover() {
	r := recover()
	if r == nil {
		return
	}
	s := current.Load()
	if !s.enabled {
		return
	}
	err := panicError(r)
	s.recordStandalone(exceptionAttributes(err, panicStack()), err.Error())
}

// RecordPanic must be called from the deferred function that recovered.
func RecordPanic(span trace.Span, r any) string {
	err := panicError(r)
	frames := panicStack()
	addException(span, exceptionAttributes(err, frames), err.Error())
	return FormatWithStack(err, frames)
}

func RecordError(span trace.Span, err error) {
	if err == nil || !span.IsRecording() {
		return
	}
	var frames []runtime.Frame
	var stackErr *StackTraceError
	if errors.As(err, &stackErr) {
		frames = stackErr.Frames
	}
	addException(span, exceptionAttributes(err, frames), err.Error())
}

func (s *state) recordStandalone(attrs []attribute.KeyValue, message string) {
	_, span := s.tracer.Start(context.Background(), "exception",
		trace.WithNewRoot(),
		trace.WithSpanKind(trace.SpanKindInternal),
	)
	addException(span, attrs, message)
	span.End()
}

func addException(span trace.Span, attrs []attribute.KeyValue, message string) {
	span.AddEvent("exception", trace.WithAttributes(attrs...))
	span.SetStatus(codes.Error, message)
}

// The stack trace is the frames alone: Traceway prepends "type: message"
// itself, which rebuilds exactly the string the native SDK sent.
func exceptionAttributes(err error, frames []runtime.Frame) []attribute.KeyValue {
	attrs := []attribute.KeyValue{
		attribute.String("exception.type", errorType(err)),
		attribute.String("exception.message", err.Error()),
	}
	if len(frames) > 0 {
		attrs = append(attrs, attribute.String("exception.stacktrace", formatFrames(frames)))
	}
	return attrs
}

func recordingSpan(ctx context.Context) (trace.Span, bool) {
	parent, ok := ParentContext(ctx)
	if !ok {
		return nil, false
	}
	span := trace.SpanFromContext(parent)
	return span, span.IsRecording()
}
