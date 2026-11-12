package otelcontrollers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/v2/app/controllers/clientcontrollers"
	"github.com/tracewayapp/traceway/backend/v2/app/middleware"
	"github.com/tracewayapp/traceway/backend/v2/app/traceway"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
)

func recordSelfMonitoring(t *testing.T) *tracetest.SpanRecorder {
	t.Helper()
	recorder := tracetest.NewSpanRecorder()
	t.Cleanup(traceway.Install(traceway.Providers{
		TracerProvider: sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder)),
	}))
	return recorder
}

func reportedException(t *testing.T, span sdktrace.ReadOnlySpan) map[string]string {
	t.Helper()
	for _, event := range span.Events() {
		if event.Name != "exception" {
			continue
		}
		attrs := map[string]string{}
		for _, kv := range event.Attributes {
			attrs[string(kv.Key)] = kv.Value.Emit()
		}
		return attrs
	}
	t.Fatalf("span %s carries no exception", span.Name())
	return nil
}

func storedStack(attrs map[string]string) string {
	return formatExceptionStackTrace(attrs["exception.type"], attrs["exception.message"], attrs["exception.stacktrace"])
}

func otlpAttributes(attrs []attribute.KeyValue) []*commonpb.KeyValue {
	out := make([]*commonpb.KeyValue, 0, len(attrs))
	for _, kv := range attrs {
		value := &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: kv.Value.Emit()}}
		if kv.Value.Type() == attribute.BOOL {
			value = &commonpb.AnyValue{Value: &commonpb.AnyValue_BoolValue{BoolValue: kv.Value.AsBool()}}
		}
		out = append(out, &commonpb.KeyValue{Key: string(kv.Key), Value: value})
	}
	return out
}

// The backend used to report its own errors through the native SDK. Over
// OTLP the same error must rebuild the same stack text, or every issue of
// the self-monitoring project would be split into a new one.
func TestSelfMonitoringExceptionsKeepTheirNativeHash(t *testing.T) {
	recorder := recordSelfMonitoring(t)

	err := traceway.NewStackTraceErrorf("failed to load project: %w", errors.New("connection refused"))
	_, span := traceway.Tracer().Start(context.Background(), "GET /api/projects/:id")
	traceway.RecordError(span, err)
	span.End()

	rebuilt := storedStack(reportedException(t, recorder.Ended()[0]))
	native := traceway.FormatWithStack(err, err.Frames)

	if rebuilt != native {
		t.Fatalf("stack text differs from the native report:\n%s\nwant:\n%s", rebuilt, native)
	}
	if clientcontrollers.ComputeExceptionHash(rebuilt, false) != clientcontrollers.ComputeExceptionHash(native, false) {
		t.Fatal("hash differs from the native report")
	}
}

// Ingest hashes every frame with its file:line. Were the frames of otelgin
// and the self-monitoring middleware part of the stack, an otelgin upgrade
// or a refactor of the middleware would split every issue of the
// self-monitoring project, so a traced request must report the stack an
// untraced one has.
func TestSelfMonitoringStacksIgnoreTheRequestInstrumentation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := recordSelfMonitoring(t)

	var created []*traceway.StackTraceError
	failing := func(c *gin.Context) {
		err := traceway.NewStackTraceErrorf("failed to load project: %w", errors.New("connection refused"))
		created = append(created, err)
		c.AbortWithError(http.StatusInternalServerError, err)
	}
	panicking := func(c *gin.Context) {
		var counts map[string]int
		counts["boom"]++
	}
	untraced := gin.New()
	traced := gin.New()
	traced.Use(middleware.SelfMonitoring("self-token")...)
	for _, router := range []*gin.Engine{untraced, traced} {
		router.GET("/api/projects/:id", failing)
		router.GET("/api/panic", panicking)
		router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/projects/1", nil))
	}
	traced.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/panic", nil))

	spans := recorder.Ended()
	if len(spans) != 2 || len(created) != 2 {
		t.Fatalf("got %d spans and %d errors", len(spans), len(created))
	}
	reported := storedStack(reportedException(t, spans[0]))
	want := traceway.FormatWithStack(created[0], created[0].Frames)
	if reported != want {
		t.Fatalf("a traced request reported a different stack than an untraced one:\n%s\nwant:\n%s", reported, want)
	}
	if clientcontrollers.ComputeExceptionHash(reported, false) != clientcontrollers.ComputeExceptionHash(want, false) {
		t.Fatal("a traced request hashes differently from an untraced one")
	}

	panicked := storedStack(reportedException(t, spans[1]))
	for _, stack := range []string{reported, panicked} {
		for _, internal := range []string{"otelgin", "gintrace.go", "recordRequestOutcome", "self_monitoring.middleware.go"} {
			if strings.Contains(stack, internal) {
				t.Fatalf("the stack carries the instrumentation frame %q:\n%s", internal, stack)
			}
		}
		if n := strings.Count(stack, "(*Context).Next()"); n != 1 {
			t.Fatalf("the stack has %d gin dispatch frames, an untraced request has 1:\n%s", n, stack)
		}
	}
}

func startFailingTask() {
	traceway.RunTask(context.Background(), "retention.auth_tokens", func(ctx context.Context) {
		traceway.CaptureExceptionContext(ctx, errors.New("prune failed"))
	})
}

func startPanickingTask() {
	defer func() { _ = recover() }()
	traceway.RunTask(context.Background(), "outbox.drain", func(context.Context) {
		panic("boom")
	})
}

// A task's stack goes from the work straight to the code that started the
// task, whether the task reports an error or panics.
func TestSelfMonitoringStacksIgnoreRunTask(t *testing.T) {
	recorder := recordSelfMonitoring(t)

	startFailingTask()
	startPanickingTask()

	spans := recorder.Ended()
	if len(spans) != 2 {
		t.Fatalf("got %d spans", len(spans))
	}
	for i, starter := range []string{"startFailingTask", "startPanickingTask"} {
		stack := storedStack(reportedException(t, spans[i]))
		lines := strings.Split(stack, "\n")
		for _, line := range lines {
			if line == "RunTask()" || strings.HasPrefix(line, "RunTask.") {
				t.Fatalf("the stack carries RunTask:\n%s", stack)
			}
		}
		if len(lines) < 5 || !strings.HasPrefix(lines[1], starter+".func") || lines[3] != starter+"()" {
			t.Fatalf("the task body must be followed by %s:\n%s", starter, stack)
		}
	}
}

// go-sdk's streamable MCP handler answers POSTs as event streams, which last
// as long as the client listens; ingest must keep them out of percentiles.
func TestSelfMonitoringEventStreamsAreStreams(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := recordSelfMonitoring(t)

	router := gin.New()
	router.Use(middleware.SelfMonitoring("self-token")...)
	router.POST("/mcp", func(c *gin.Context) {
		c.Header("Content-Type", "text/event-stream")
		c.String(http.StatusOK, "event: message\ndata: {}\n\n")
	})
	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/mcp", nil))

	span := recorder.Ended()[0]
	if !isOtelStreamingEndpoint(otlpAttributes(span.Attributes()), http.StatusOK) {
		t.Fatal("an event-stream answer must be stored as a stream")
	}
}

// The native SDK sent CaptureMessage with isMessage, which stores a message
// and hashes the text as written.
func TestSelfMonitoringMessagesStayMessages(t *testing.T) {
	recorder := recordSelfMonitoring(t)

	msg := "Invalid framework received: cobol"
	traceway.CaptureMessage(msg)

	eventAttrs := otlpAttributes(recorder.Ended()[0].Events()[0].Attributes)
	stored := buildException(context.Background(), nil, uuid.New(), eventAttrs, 0, exceptionContext{}, "api-1", "", "go", "", "")

	if !stored.IsMessage {
		t.Fatal("a captured message must be stored as a message")
	}
	if stored.ExceptionHash != clientcontrollers.ComputeExceptionHash(msg, true) {
		t.Fatal("a message must keep the hash the native SDK gave it")
	}
}
