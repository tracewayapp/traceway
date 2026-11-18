package middleware

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/tracewayapp/traceway/backend/v2/app/traceway"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

func selfMonitoredRouter(t *testing.T) (*gin.Engine, *tracetest.SpanRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := tracetest.NewSpanRecorder()
	t.Cleanup(traceway.Install(traceway.Providers{
		TracerProvider: sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder)),
		ServiceName:    "traceway-test",
	}))

	router := gin.New()
	router.Use(SelfMonitoring("self-token")...)
	ok := func(c *gin.Context) { c.Status(http.StatusOK) }
	router.GET("/api/items/:id", ok)
	router.POST("/api/otel/v1/traces", ok)
	router.POST("/api/runners/poll", ok)
	router.POST("/api/otel/v1/opamp", ok)
	router.GET("/mcp", ok)
	router.POST("/mcp", ok)
	router.GET("/assets/*filepath", ok)
	router.GET("/api/invitations/:token", ok)
	router.GET("/api/fail", func(c *gin.Context) {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("loading items: %w", errors.New("db down")))
	})
	router.GET("/api/panic", func(c *gin.Context) {
		var m map[string]int
		m["boom"]++
	})
	router.NoRoute(ok)
	return router, recorder
}

func serve(router *gin.Engine, method, path, auth string) int {
	req := httptest.NewRequest(method, path, nil)
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w.Code
}

func TestSelfMonitoringNeverTracesItsOwnExports(t *testing.T) {
	router, recorder := selfMonitoredRouter(t)

	serve(router, http.MethodPost, "/api/otel/v1/traces", "Bearer self-token")
	serve(router, http.MethodGet, "/api/items/1", "Bearer self-token")

	if n := len(recorder.Ended()); n != 0 {
		t.Fatalf("requests carrying the monitoring token must not be traced, got %d spans", n)
	}

	serve(router, http.MethodPost, "/api/otel/v1/traces", "Bearer customer-token")
	if n := len(recorder.Ended()); n != 1 {
		t.Fatalf("customer ingest must stay traced, got %d spans", n)
	}
}

func TestSelfMonitoringSkipsStreamsStaticAndUnmatched(t *testing.T) {
	router, recorder := selfMonitoredRouter(t)

	serve(router, http.MethodPost, "/api/runners/poll", "")
	serve(router, http.MethodPost, "/api/otel/v1/opamp", "Bearer customer-token")
	serve(router, http.MethodGet, "/mcp", "")
	serve(router, http.MethodGet, "/assets/app.js", "")
	serve(router, http.MethodGet, "/some/spa/page", "")

	if n := len(recorder.Ended()); n != 0 {
		t.Fatalf("got %d spans, want none", n)
	}

	serve(router, http.MethodPost, "/mcp", "")
	if n := len(recorder.Ended()); n != 1 {
		t.Fatalf("MCP tool calls are ordinary requests and must be traced, got %d spans", n)
	}
}

func TestSelfMonitoringReportsTheRoute(t *testing.T) {
	router, recorder := selfMonitoredRouter(t)

	serve(router, http.MethodGet, "/api/items/1", "")
	serve(router, http.MethodGet, "/api/items/2", "")

	for _, span := range recorder.Ended() {
		if span.SpanKind() != trace.SpanKindServer || attr(span, "http.route") != "/api/items/:id" {
			t.Fatalf("got kind %v route %q", span.SpanKind(), attr(span, "http.route"))
		}
	}
}

func TestSelfMonitoringRedactsTokensInPaths(t *testing.T) {
	router, recorder := selfMonitoredRouter(t)

	serve(router, http.MethodGet, "/api/invitations/secret-invite-token", "")

	if got := attr(recorder.Ended()[0], "url.path"); got != "/api/invitations/:token" {
		t.Fatalf("url.path = %q, the token must not be recorded", got)
	}
}

func TestSelfMonitoringRecordsHandlerErrors(t *testing.T) {
	router, recorder := selfMonitoredRouter(t)

	if code := serve(router, http.MethodGet, "/api/fail", ""); code != http.StatusInternalServerError {
		t.Fatalf("status = %d", code)
	}

	span := recorder.Ended()[0]
	if exceptionAttr(span, "exception.type") != "*traceway.StackTraceError" {
		t.Fatalf("exception.type = %q", exceptionAttr(span, "exception.type"))
	}
	if !strings.Contains(exceptionAttr(span, "exception.stacktrace"), "self_monitoring.middleware_test.go") {
		t.Fatal("the exception must carry the stack captured where the error was created")
	}
}

func TestSelfMonitoringKeepsIngestStatementsOutOfTheTrace(t *testing.T) {
	router, recorder := selfMonitoredRouter(t)
	statementsTraced := map[string]bool{}
	probe := func(c *gin.Context) {
		_, statementsTraced[c.FullPath()] = traceway.StatementParent(c)
		c.Status(http.StatusOK)
	}
	router.POST("/api/report", probe)
	router.POST("/api/recordings", probe)
	router.GET("/api/projects", probe)

	serve(router, http.MethodPost, "/api/report", "")
	serve(router, http.MethodPost, "/api/recordings", "")
	serve(router, http.MethodGet, "/api/projects", "")

	if statementsTraced["/api/report"] {
		t.Fatal("ingest statements grow with customer traffic and must not be traced")
	}
	if statementsTraced["/api/recordings"] {
		t.Fatal("recording ingest statements grow with customer traffic and must not be traced")
	}
	if !statementsTraced["/api/projects"] {
		t.Fatal("dashboard statements must be traced")
	}
	if n := len(recorder.Ended()); n != 3 {
		t.Fatalf("all requests must still be traced, got %d spans", n)
	}
}

func TestSelfMonitoringRecoversPanics(t *testing.T) {
	router, recorder := selfMonitoredRouter(t)

	if code := serve(router, http.MethodGet, "/api/panic", ""); code != http.StatusInternalServerError {
		t.Fatalf("a panic must answer 500, got %d", code)
	}

	span := recorder.Ended()[0]
	if attr(span, "http.response.status_code") != "500" {
		t.Fatalf("span status code = %q", attr(span, "http.response.status_code"))
	}
	if !strings.Contains(exceptionAttr(span, "exception.stacktrace"), "self_monitoring.middleware_test.go") {
		t.Fatal("the panic stack must point at the handler")
	}
}

func TestSelfMonitoringMarksEventStreams(t *testing.T) {
	router, recorder := selfMonitoredRouter(t)
	router.POST("/mcp/write", func(c *gin.Context) {
		c.Header("Content-Type", "text/event-stream; charset=utf-8")
		c.String(http.StatusOK, "event: message\ndata: {}\n\n")
	})

	serve(router, http.MethodPost, "/mcp/write", "")
	serve(router, http.MethodPost, "/mcp", "")

	spans := recorder.Ended()
	if len(spans) != 2 {
		t.Fatalf("got %d spans", len(spans))
	}
	if attr(spans[0], "traceway.is_stream") != "true" {
		t.Fatal("an event-stream answer must be marked as a stream, or it skews the endpoint's latency")
	}
	if attr(spans[1], "traceway.is_stream") != "" {
		t.Fatal("an ordinary answer must not be marked as a stream")
	}
}

func attr(span sdktrace.ReadOnlySpan, key string) string {
	for _, kv := range span.Attributes() {
		if string(kv.Key) == key {
			return kv.Value.Emit()
		}
	}
	return ""
}

func exceptionAttr(span sdktrace.ReadOnlySpan, key string) string {
	for _, event := range span.Events() {
		for _, kv := range event.Attributes {
			if event.Name == "exception" && string(kv.Key) == key {
				return kv.Value.AsString()
			}
		}
	}
	return ""
}
