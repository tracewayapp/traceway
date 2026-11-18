package traceway

import (
	"context"
	"sync"

	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

const (
	idleTaskAttribute = "traceway.task.idle"

	// An idle poll tick ends after a handful of statements. A task past this
	// many spans is doing work, so its spans are released as they end
	// instead of piling up in memory until the task finishes.
	heldSpanLimit = 64
)

// taskFilter holds back the spans of a running task until the task ends, so
// a tick that found nothing to do (SkipTask) is dropped together with the
// statements it ran.
type taskFilter struct {
	sdktrace.SpanProcessor
	mu   sync.Mutex
	held map[trace.TraceID]*heldTask
}

type heldTask struct {
	spans    []sdktrace.ReadOnlySpan
	released bool
}

func newTaskFilter(next sdktrace.SpanProcessor) *taskFilter {
	return &taskFilter{SpanProcessor: next, held: map[trace.TraceID]*heldTask{}}
}

func (f *taskFilter) OnStart(parent context.Context, s sdktrace.ReadWriteSpan) {
	if isTaskRoot(s) {
		f.mu.Lock()
		f.held[s.SpanContext().TraceID()] = &heldTask{}
		f.mu.Unlock()
	}
	f.SpanProcessor.OnStart(parent, s)
}

func (f *taskFilter) OnEnd(s sdktrace.ReadOnlySpan) {
	traceID := s.SpanContext().TraceID()
	f.mu.Lock()
	task, ok := f.held[traceID]
	if !ok || (task.released && !isTaskRoot(s)) {
		f.mu.Unlock()
		f.SpanProcessor.OnEnd(s)
		return
	}
	if !isTaskRoot(s) {
		task.spans = append(task.spans, s)
		var release []sdktrace.ReadOnlySpan
		if len(task.spans) >= heldSpanLimit {
			release, task.spans, task.released = task.spans, nil, true
		}
		f.mu.Unlock()
		f.forward(release)
		return
	}
	delete(f.held, traceID)
	f.mu.Unlock()

	if !task.released && isIdle(s) {
		return
	}
	f.forward(task.spans)
	f.SpanProcessor.OnEnd(s)
}

func (f *taskFilter) forward(spans []sdktrace.ReadOnlySpan) {
	for _, s := range spans {
		f.SpanProcessor.OnEnd(s)
	}
}

func isTaskRoot(s sdktrace.ReadOnlySpan) bool {
	return !s.Parent().IsValid() && s.SpanKind() == trace.SpanKindConsumer
}

func isIdle(s sdktrace.ReadOnlySpan) bool {
	if s.Status().Code == codes.Error {
		return false
	}
	for _, kv := range s.Attributes() {
		if kv.Key == idleTaskAttribute {
			return kv.Value.AsBool()
		}
	}
	return false
}
