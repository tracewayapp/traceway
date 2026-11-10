package otelcontrollers

import (
	"context"
	"errors"
	"testing"

	"github.com/tracewayapp/traceway/backend/app/controllers/clientcontrollers"
	"github.com/tracewayapp/traceway/backend/app/traceway"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// The backend used to report its own errors through the native SDK. Over
// OTLP the same error must rebuild the same stack text, or every issue of
// the self-monitoring project would be split into a new one.
func TestSelfMonitoringExceptionsKeepTheirNativeHash(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	t.Cleanup(traceway.Install(traceway.Providers{
		TracerProvider: sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder)),
	}))

	err := traceway.NewStackTraceErrorf("failed to load project: %w", errors.New("connection refused"))
	_, span := traceway.Tracer().Start(context.Background(), "GET /api/projects/:id")
	traceway.RecordError(span, err)
	span.End()

	attrs := map[string]string{}
	for _, kv := range recorder.Ended()[0].Events()[0].Attributes {
		attrs[string(kv.Key)] = kv.Value.AsString()
	}
	rebuilt := formatExceptionStackTrace(attrs["exception.type"], attrs["exception.message"], attrs["exception.stacktrace"])
	native := traceway.FormatWithStack(err, err.Frames)

	if rebuilt != native {
		t.Fatalf("stack text differs from the native report:\n%s\nwant:\n%s", rebuilt, native)
	}
	if clientcontrollers.ComputeExceptionHash(rebuilt, false) != clientcontrollers.ComputeExceptionHash(native, false) {
		t.Fatal("hash differs from the native report")
	}
}
