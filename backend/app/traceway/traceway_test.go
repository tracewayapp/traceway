package traceway

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/codes"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

func installRecorder(t *testing.T) *tracetest.SpanRecorder {
	t.Helper()
	recorder := tracetest.NewSpanRecorder()
	restore := Install(Providers{TracerProvider: sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))})
	t.Cleanup(restore)
	return recorder
}

func eventAttr(t *testing.T, span sdktrace.ReadOnlySpan, key string) string {
	t.Helper()
	for _, event := range span.Events() {
		if event.Name != "exception" {
			continue
		}
		for _, kv := range event.Attributes {
			if string(kv.Key) == key {
				return kv.Value.AsString()
			}
		}
	}
	return ""
}

func TestParseConnectionString(t *testing.T) {
	cases := []struct {
		in, token, base string
		wantErr         bool
	}{
		{in: "tok@https://traceway.example.com/api/report", token: "tok", base: "https://traceway.example.com"},
		{in: "tok@http://localhost:8082", token: "tok", base: "http://localhost:8082"},
		{in: "tok@https://example.com/traceway/api/otel/", token: "tok", base: "https://example.com/traceway"},
		{in: "https://example.com", wantErr: true},
		{in: "tok@", wantErr: true},
		{in: "tok@ftp://example.com", wantErr: true},
	}
	for _, tc := range cases {
		token, base, err := ParseConnectionString(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("%q: expected an error", tc.in)
			}
			continue
		}
		if err != nil || token != tc.token || base != tc.base {
			t.Errorf("%q: got (%q, %q, %v), want (%q, %q)", tc.in, token, base, err, tc.token, tc.base)
		}
	}
}

// The native SDK formatted frames exactly like this, and Traceway hashes the
// result: a drift here regroups every issue of the self-monitoring project.
func TestFormatFramesMatchesNativeSDK(t *testing.T) {
	frames := []runtime.Frame{
		{Function: "github.com/tracewayapp/traceway/backend/v2/app/controllers.(*projectController).Get", File: "/src/backend/app/controllers/project.controller.go", Line: 42},
		{Function: "github.com/gin-gonic/gin.(*Context).Next", File: "/go/pkg/mod/github.com/gin-gonic/gin@v1.12.0/context.go", Line: 192},
		{Function: "runtime.goexit", File: "/usr/local/go/src/runtime/asm_arm64.s", Line: 1223},
	}
	want := "(*projectController).Get()\n    /src/backend/app/controllers/project.controller.go:42\n" +
		"(*Context).Next()\n    /go/pkg/mod/github.com/gin-gonic/gin@v1.12.0/context.go:192\n" +
		"goexit()\n    /usr/local/go/src/runtime/asm_arm64.s:1223\n"
	if got := formatFrames(frames); got != want {
		t.Fatalf("frame format drifted:\n%s\nwant:\n%s", got, want)
	}
	err := &StackTraceError{Err: errors.New("boom")}
	if got := FormatWithStack(err, frames[:1]); !strings.HasPrefix(got, "*traceway.StackTraceError: boom\n(*projectController).Get()\n") {
		t.Fatalf("unexpected header: %q", got)
	}
}

func newStackError() *StackTraceError {
	return NewStackTraceErrorf("wrapped: %w", errors.New("root"))
}

func TestNewStackTraceErrorfStartsAtCaller(t *testing.T) {
	err := newStackError()
	if !strings.HasSuffix(err.Frames[0].Function, ".newStackError") {
		t.Fatalf("first frame = %s, want newStackError", err.Frames[0].Function)
	}
	again := NewStackTraceErrorf("outer: %w", err)
	if again.Frames[0] != err.Frames[0] {
		t.Fatal("a wrapped StackTraceError must keep the original frames")
	}
}

func TestCaptureExceptionIsAnUnlinkedIssue(t *testing.T) {
	recorder := installRecorder(t)

	CaptureException(errors.New("disk full"))

	spans := recorder.Ended()
	if len(spans) != 1 {
		t.Fatalf("want 1 span, got %d", len(spans))
	}
	span := spans[0]
	if span.Parent().IsValid() || span.SpanKind() != trace.SpanKindInternal {
		t.Fatalf("exception span must be an internal root, got kind %v parent %v", span.SpanKind(), span.Parent())
	}
	if got := eventAttr(t, span, "exception.type"); got != "*errors.errorString" {
		t.Fatalf("exception.type = %q", got)
	}
	stack := eventAttr(t, span, "exception.stacktrace")
	if !strings.HasPrefix(stack, "TestCaptureExceptionIsAnUnlinkedIssue()\n") {
		t.Fatalf("stack must start at the caller, got %q", stack)
	}
}

func TestCaptureExceptionContextMarksTheTask(t *testing.T) {
	recorder := installRecorder(t)

	RunTask(context.Background(), "outbox.drain", func(ctx context.Context) {
		CaptureExceptionContext(ctx, errors.New("tick failed"))
	})

	spans := recorder.Ended()
	if len(spans) != 1 {
		t.Fatalf("want only the task span, got %d spans", len(spans))
	}
	task := spans[0]
	if task.Name() != "outbox.drain" || task.SpanKind() != trace.SpanKindConsumer {
		t.Fatalf("got %s/%v, want outbox.drain/consumer", task.Name(), task.SpanKind())
	}
	if eventAttr(t, task, "exception.message") != "tick failed" {
		t.Fatal("exception must be recorded on the task span")
	}
}

func TestRecoverStackStartsAtThePanic(t *testing.T) {
	recorder := installRecorder(t)

	func() {
		defer Recover()
		panicHere()
	}()

	stack := eventAttr(t, recorder.Ended()[0], "exception.stacktrace")
	if !strings.HasPrefix(stack, "panicHere()\n") {
		t.Fatalf("stack must start where the panic happened, got %q", stack)
	}
}

func panicHere() {
	panic("bad state")
}

func TestRecordErrorUsesTheEmbeddedStack(t *testing.T) {
	recorder := installRecorder(t)
	_, span := Tracer().Start(context.Background(), "GET /api/projects")

	RecordError(span, newStackError())
	span.End()

	ended := recorder.Ended()[0]
	if got := eventAttr(t, ended, "exception.type"); got != "*traceway.StackTraceError" {
		t.Fatalf("exception.type = %q", got)
	}
	if !strings.HasPrefix(eventAttr(t, ended, "exception.stacktrace"), "newStackError()\n") {
		t.Fatal("stack must be the one captured by NewStackTraceErrorf")
	}
}

func TestParentContextReadsTheGinRequest(t *testing.T) {
	installRecorder(t)
	requestCtx, span := Tracer().Start(context.Background(), "GET /x")
	defer span.End()

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/x", nil).WithContext(requestCtx)

	parent, ok := ParentContext(context.WithoutCancel(c))
	if !ok || trace.SpanFromContext(parent).SpanContext().SpanID() != span.SpanContext().SpanID() {
		t.Fatal("the request span must be found through the gin context")
	}
	if _, ok := ParentContext(context.Background()); ok {
		t.Fatal("a context without a span has no parent")
	}
}

func TestDisabledIsANoop(t *testing.T) {
	CaptureException(errors.New("ignored"))
	ctx := context.Background()
	spanCtx, span := StartSpan(ctx, "x")
	if span != nil || spanCtx != ctx {
		t.Fatal("StartSpan must return no span and the same context without a parent")
	}
	span.End()
	if err := Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown must be a no-op when monitoring is off: %v", err)
	}
	ran := false
	RunTask(context.Background(), "x", func(context.Context) { ran = true })
	if !ran {
		t.Fatal("RunTask must still run the work when monitoring is off")
	}
}

func TestDeltaMetrics(t *testing.T) {
	reader := sdkmetric.NewManualReader(sdkmetric.WithTemporalitySelector(temporality))
	restore := Install(Providers{MeterProvider: sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))})
	defer restore()

	tags := map[string]string{"signal": "traces"}
	CaptureMetricWithTags("traceway.ingest.batch.size", 10, tags)
	CaptureMetricWithTags("traceway.ingest.batch.size", 20, tags)
	CaptureCount("traceway.ingest.rate_limited", 1, nil)
	CaptureCount("traceway.ingest.rate_limited", 1, nil)

	values := collect(t, reader)
	if values["traceway.ingest.batch.size"] != 20 {
		t.Fatalf("gauge must report its last value, got %v", values["traceway.ingest.batch.size"])
	}
	if values["traceway.ingest.rate_limited"] != 2 {
		t.Fatalf("counter must sum its events, got %v", values["traceway.ingest.rate_limited"])
	}
	if again := collect(t, reader); len(again) != 0 {
		t.Fatalf("a value must be exported once, got %v on the next collection", again)
	}
}

func collect(t *testing.T, reader *sdkmetric.ManualReader) map[string]float64 {
	t.Helper()
	var rm metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &rm); err != nil {
		t.Fatal(err)
	}
	values := map[string]float64{}
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			switch data := m.Data.(type) {
			case metricdata.Gauge[float64]:
				for _, dp := range data.DataPoints {
					values[m.Name] = dp.Value
				}
			case metricdata.Sum[int64]:
				for _, dp := range data.DataPoints {
					values[m.Name] = float64(dp.Value)
				}
			}
		}
	}
	return values
}

func TestStartSpanNestsWorkUnderIt(t *testing.T) {
	recorder := installRecorder(t)
	requestCtx, request := Tracer().Start(context.Background(), "GET /x")

	spanCtx, span := StartSpan(requestCtx, "loading")
	_, child := Tracer().Start(spanCtx, "SELECT spans_v2")
	child.End()
	span.End()
	request.End()

	byName := map[string]sdktrace.ReadOnlySpan{}
	for _, s := range recorder.Ended() {
		byName[s.Name()] = s
	}
	if byName["SELECT spans_v2"].Parent().SpanID() != byName["loading"].SpanContext().SpanID() {
		t.Fatal("work run with the returned context must nest under the span")
	}
	if byName["loading"].Parent().SpanID() != byName["GET /x"].SpanContext().SpanID() {
		t.Fatal("the span must nest under the request")
	}
}

func TestStatementParentHonoursTheOptOut(t *testing.T) {
	installRecorder(t)
	requestCtx, span := Tracer().Start(context.Background(), "POST /api/otel/v1/traces")
	defer span.End()

	if _, ok := StatementParent(requestCtx); !ok {
		t.Fatal("a request span is a statement parent by default")
	}
	if _, ok := StatementParent(WithoutStatementSpans(requestCtx)); ok {
		t.Fatal("an opted-out context must not parent statements")
	}

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/api/otel/v1/traces", nil).WithContext(WithoutStatementSpans(requestCtx))
	if _, ok := StatementParent(context.WithoutCancel(c)); ok {
		t.Fatal("the opt-out must be found through the gin request")
	}
	spanCtx, child := StartSpan(c, "report.insert.spans")
	defer child.End()
	if _, ok := StatementParent(spanCtx); ok {
		t.Fatal("the opt-out must cover spans started under the request")
	}
}

func TestRepanicStackStartsAtTheRepanic(t *testing.T) {
	recorder := installRecorder(t)

	func() {
		defer Recover()
		rollbackAndRepanic()
	}()

	stack := eventAttr(t, recorder.Ended()[0], "exception.stacktrace")
	if !strings.HasPrefix(stack, "rollbackAndRepanic.func1()\n") {
		t.Fatalf("stack must start at the re-panic, as the native SDK did, got %q", stack)
	}
}

func rollbackAndRepanic() {
	defer func() {
		if r := recover(); r != nil {
			panic(r)
		}
	}()
	panicHere()
}

func TestRunTaskRecordsAPanicOnce(t *testing.T) {
	recorder := installRecorder(t)

	func() {
		defer Recover()
		RunTask(context.Background(), "outbox.drain", func(context.Context) {
			panicHere()
		})
	}()

	spans := recorder.Ended()
	if len(spans) != 1 {
		t.Fatalf("want only the task span, got %d spans", len(spans))
	}
	if spans[0].Status().Code != codes.Error {
		t.Fatal("the task must be marked failed")
	}
	if !strings.HasPrefix(eventAttr(t, spans[0], "exception.stacktrace"), "panicHere()\n") {
		t.Fatal("the panic must be recorded on the task, starting where it happened")
	}
}

func TestRunTaskStillPanicsWithoutARecover(t *testing.T) {
	installRecorder(t)
	defer func() {
		r := recover()
		if r == nil || !strings.Contains(fmt.Sprint(r), "bad state") {
			t.Fatalf("the panic must reach the caller, got %v", r)
		}
	}()
	RunTask(context.Background(), "x", func(context.Context) { panicHere() })
}

func TestCaptureMessageIsFlaggedAsAMessage(t *testing.T) {
	recorder := installRecorder(t)

	CaptureMessage("Invalid framework received: cobol")

	for _, event := range recorder.Ended()[0].Events() {
		for _, kv := range event.Attributes {
			if kv.Key == "traceway.is_message" && kv.Value.AsBool() {
				return
			}
		}
	}
	t.Fatal("a message must carry traceway.is_message")
}

func TestIdleTasksAreDroppedWithTheirSpans(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	restore := Install(Providers{TracerProvider: sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(newTaskFilter(recorder)))})
	defer restore()

	RunTask(context.Background(), "outbox.drain", func(ctx context.Context) {
		_, statement := Tracer().Start(ctx, "SELECT notification_outbox")
		statement.End()
		SkipTask(ctx)
	})
	if n := len(recorder.Ended()); n != 0 {
		t.Fatalf("an idle tick must not be reported, got %d spans", n)
	}

	RunTask(context.Background(), "outbox.drain", func(ctx context.Context) {
		_, statement := Tracer().Start(ctx, "SELECT notification_outbox")
		statement.End()
	})
	if n := len(recorder.Ended()); n != 2 {
		t.Fatalf("a tick that did work must be reported with its spans, got %d spans", n)
	}

	RunTask(context.Background(), "outbox.drain", func(ctx context.Context) {
		CaptureExceptionContext(ctx, errors.New("claim failed"))
		SkipTask(ctx)
	})
	if n := len(recorder.Ended()); n != 3 {
		t.Fatalf("a failed tick must be reported even when marked idle, got %d spans", n)
	}

	_, request := Tracer().Start(context.Background(), "GET /x")
	request.End()
	if n := len(recorder.Ended()); n != 4 {
		t.Fatalf("spans outside tasks must pass straight through, got %d spans", n)
	}
}

func TestBusyTasksReleaseTheirSpansEarly(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	restore := Install(Providers{TracerProvider: sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(newTaskFilter(recorder)))})
	defer restore()

	RunTask(context.Background(), "slo.compute", func(ctx context.Context) {
		for i := 0; i < heldSpanLimit; i++ {
			_, statement := Tracer().Start(ctx, "SELECT slo_buckets")
			statement.End()
		}
		if n := len(recorder.Ended()); n != heldSpanLimit {
			t.Fatalf("a busy task must not hold its spans until it ends, got %d released", n)
		}
	})
	if n := len(recorder.Ended()); n != heldSpanLimit+1 {
		t.Fatalf("want every span and the task, got %d", n)
	}
}

func TestMaxMetricsKeepTheLargestValuePerWindow(t *testing.T) {
	reader := sdkmetric.NewManualReader(sdkmetric.WithTemporalitySelector(temporality))
	restore := Install(Providers{MeterProvider: sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))})
	defer restore()

	tags := map[string]string{"signal": "traces", "table": "spans"}
	CaptureMetricMax("traceway.ingest.batch.insert_ms", 40, tags)
	CaptureMetricMax("traceway.ingest.batch.insert_ms", 900, tags)
	CaptureMetricMax("traceway.ingest.batch.insert_ms", 60, tags)

	if got := collect(t, reader)["traceway.ingest.batch.insert_ms"]; got != 900 {
		t.Fatalf("the window must report its largest batch, got %v", got)
	}
	if again := collect(t, reader); len(again) != 0 {
		t.Fatalf("a window without batches must report nothing, got %v", again)
	}
	CaptureMetricMax("traceway.ingest.batch.insert_ms", 30, tags)
	if got := collect(t, reader)["traceway.ingest.batch.insert_ms"]; got != 30 {
		t.Fatalf("each window must start over, got %v", got)
	}
}

func TestSetupIgnoresTheCallersSamplingDecision(t *testing.T) {
	restoreSetup(t)
	shutdown, err := Setup(context.Background(), Options{ConnectionString: "tok@" + discardingCollector(t)})
	if err != nil {
		t.Fatal(err)
	}
	defer shutdown(context.Background())

	remote := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: trace.TraceID{1},
		SpanID:  trace.SpanID{1},
		Remote:  true,
	})
	_, span := Tracer().Start(trace.ContextWithRemoteSpanContext(context.Background(), remote), "GET /x", trace.WithSpanKind(trace.SpanKindServer))
	defer span.End()
	if !span.IsRecording() {
		t.Fatal("a caller's unsampled traceparent must not switch the backend's own tracing off")
	}
}

func TestSetupSurvivesMalformedResourceAttributes(t *testing.T) {
	restoreSetup(t)
	t.Setenv("OTEL_RESOURCE_ATTRIBUTES", "team=core,broken")
	shutdown, err := Setup(context.Background(), Options{ConnectionString: "tok@" + discardingCollector(t), ServiceName: "api-1"})
	if err != nil {
		t.Fatalf("a malformed entry must not turn self-monitoring off: %v", err)
	}
	defer shutdown(context.Background())
	if !Enabled() || ServiceName() != "api-1" {
		t.Fatalf("want monitoring on as api-1, got enabled=%v service=%q", Enabled(), ServiceName())
	}
}

func restoreSetup(t *testing.T) {
	t.Helper()
	previous := current.Load()
	t.Cleanup(func() { current.Store(previous) })
}

func discardingCollector(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	t.Cleanup(srv.Close)
	return srv.URL
}
