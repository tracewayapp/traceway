package otelcontrollers

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/controllers/clientcontrollers"
	"github.com/tracewayapp/traceway/backend/app/traceway"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
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

// The native SDK sent CaptureMessage with isMessage, which stores a message
// and hashes the text as written.
func TestSelfMonitoringMessagesStayMessages(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	t.Cleanup(traceway.Install(traceway.Providers{
		TracerProvider: sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder)),
	}))

	msg := "Invalid framework received: cobol"
	traceway.CaptureMessage(msg)

	var eventAttrs []*commonpb.KeyValue
	for _, kv := range recorder.Ended()[0].Events()[0].Attributes {
		value := &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: kv.Value.Emit()}}
		if kv.Value.Type() == attribute.BOOL {
			value = &commonpb.AnyValue{Value: &commonpb.AnyValue_BoolValue{BoolValue: kv.Value.AsBool()}}
		}
		eventAttrs = append(eventAttrs, &commonpb.KeyValue{Key: string(kv.Key), Value: value})
	}
	stored := buildException(context.Background(), nil, uuid.New(), eventAttrs, 0, map[string]string{}, "api-1", "", "go", "", "")

	if !stored.IsMessage {
		t.Fatal("a captured message must be stored as a message")
	}
	if stored.ExceptionHash != clientcontrollers.ComputeExceptionHash(msg, true) {
		t.Fatal("a message must keep the hash the native SDK gave it")
	}
}
