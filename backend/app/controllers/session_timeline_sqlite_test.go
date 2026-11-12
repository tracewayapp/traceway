//go:build !transactional_pg && !telemetry_ch && !telemetry_duckdb

package controllers

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/v2/app/config"
	"github.com/tracewayapp/traceway/backend/v2/app/controllers/otelcontrollers"
	"github.com/tracewayapp/traceway/backend/v2/app/dbtest"
	"github.com/tracewayapp/traceway/backend/v2/app/middleware"
	"github.com/tracewayapp/traceway/backend/v2/app/models"
	"github.com/tracewayapp/traceway/backend/v2/app/repositories/telemetry"
	"github.com/tracewayapp/traceway/backend/v2/app/repositories/telemetry/shared"
	"github.com/tracewayapp/traceway/backend/v2/app/traceway"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
	collogspb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/protobuf/proto"
)

func kvString(key, value string) *commonpb.KeyValue {
	return &commonpb.KeyValue{Key: key, Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: value}}}
}

func kvInt(key string, value int64) *commonpb.KeyValue {
	return &commonpb.KeyValue{Key: key, Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_IntValue{IntValue: value}}}
}

func postOtlp(t *testing.T, projectId uuid.UUID, path string, msg proto.Message, handler gin.HandlerFunc) {
	t.Helper()
	body, err := proto.Marshal(msg)
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/x-protobuf")
	c.Set(middleware.ProjectIdContextKey, projectId)
	handler(c)
	if recorder.Code != http.StatusOK || len(c.Errors) > 0 {
		t.Fatalf("%s failed: %d %v", path, recorder.Code, c.Errors)
	}
}

func TestSessionTimelineReadsBreadcrumbsFromTelemetry(t *testing.T) {
	spanRecorder := tracetest.NewSpanRecorder()
	t.Cleanup(traceway.Install(traceway.Providers{
		TracerProvider: sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(spanRecorder)),
	}))
	dbtest.SetupSQLite(t)
	previous := config.Config
	config.Config = &config.Cfg{}
	t.Cleanup(func() { config.Config = previous })
	gin.SetMode(gin.TestMode)

	projectId := uuid.New()
	session := uuid.New().String()
	other := uuid.New().String()
	base := time.Now().UTC().Add(-time.Minute).Truncate(time.Millisecond)
	at := func(offset time.Duration) uint64 { return uint64(base.Add(offset).UnixNano()) }
	record := func(offset time.Duration, event string, severity int32, body string, attrs ...*commonpb.KeyValue) *logspb.LogRecord {
		r := &logspb.LogRecord{EventName: event, TimeUnixNano: at(offset), SeverityNumber: logspb.SeverityNumber(severity), Attributes: attrs}
		if body != "" {
			r.Body = &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: body}}
		}
		return r
	}
	sid := kvString("session.id", session)

	postOtlp(t, projectId, "/api/otel/v1/logs", &collogspb.ExportLogsServiceRequest{ResourceLogs: []*logspb.ResourceLogs{{
		Resource: &resourcepb.Resource{Attributes: []*commonpb.KeyValue{kvString("service.name", "shop-web")}},
		ScopeLogs: []*logspb.ScopeLogs{{LogRecords: []*logspb.LogRecord{
			record(0, "browser.navigation", 9, "", sid, kvString("url.full", "https://shop.test/cart"), kvString("browser.navigation.type", "push")),
			record(time.Second, "browser.console", 17, "payment failed for order 7", sid),
			record(2*time.Second, "traceway.action", 9, "", sid, kvString("traceway.action.category", "cart"),
				kvString("traceway.action.name", "add_item"), kvString("traceway.action.data", `{"sku":"A1"}`)),
			record(3*time.Second, "browser.web_vital", 9, "", sid, kvString("browser.web_vital.name", "LCP")),
			record(4*time.Second, "checkout_failed", 9, "", sid, kvString("step", "payment"), kvString("url", "https://shop.test/cart")),
			record(4500*time.Millisecond, "browser.navigation", 9, "", sid, kvString("url.full", "https://shop.test/cart#pay"),
				kvString("browser.navigation.same_document", "true"), kvString("browser.navigation.hash_change", "true")),
			record(5*time.Second, "browser.console", 9, "someone else's session", kvString("session.id", other)),
		}}},
	}}}, otelcontrollers.OtelController.ExportLogs)

	traceID := uuid.New()
	postOtlp(t, projectId, "/api/otel/v1/traces", &coltracepb.ExportTraceServiceRequest{ResourceSpans: []*tracepb.ResourceSpans{{
		Resource: &resourcepb.Resource{Attributes: []*commonpb.KeyValue{kvString("service.name", "shop-web")}},
		ScopeSpans: []*tracepb.ScopeSpans{{Spans: []*tracepb.Span{{
			TraceId: traceID[:], SpanId: []byte{1, 2, 3, 4, 5, 6, 7, 8}, Name: "POST", Kind: tracepb.Span_SPAN_KIND_CLIENT,
			StartTimeUnixNano: at(1500 * time.Millisecond), EndTimeUnixNano: at(1800 * time.Millisecond),
			Attributes: []*commonpb.KeyValue{sid, kvString("http.request.method", "POST"),
				kvString("url.full", "https://shop.test/api/checkout"), kvInt("http.response.status_code", 500)},
		}}}},
	}}}, otelcontrollers.OtelController.ExportTraces)

	router := gin.New()
	router.Use(middleware.SelfMonitoring("self-token")...)
	router.GET("/api/sessions/:sessionId/timeline", func(c *gin.Context) {
		c.Set(middleware.ProjectIdContextKey, projectId)
	}, SessionTimelineController.Timeline)
	recorder := httptest.NewRecorder()
	query := "?from=" + base.Add(-time.Minute).Format(time.RFC3339Nano) + "&to=" + base.Add(time.Minute).Format(time.RFC3339Nano)
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/sessions/"+session+"/timeline"+query, nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("timeline = %d %s", recorder.Code, recorder.Body.String())
	}
	spans := spanRecorder.Ended()
	if len(spans) != 3 {
		t.Fatalf("want the request and its two database queries, got %d spans", len(spans))
	}
	requestSpan := spans[len(spans)-1]
	if requestSpan.SpanKind() != trace.SpanKindServer {
		t.Fatalf("request span kind = %v", requestSpan.SpanKind())
	}
	for _, span := range spans[:len(spans)-1] {
		if span.SpanKind() != trace.SpanKindClient || span.Parent().SpanID() != requestSpan.SpanContext().SpanID() ||
			span.SpanContext().TraceID() != requestSpan.SpanContext().TraceID() {
			t.Fatalf("database span %q is not a child of the timeline request", span.Name())
		}
	}
	var timeline struct {
		Logs    []map[string]any `json:"logs"`
		Actions []map[string]any `json:"actions"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &timeline); err != nil {
		t.Fatal(err)
	}
	if len(timeline.Logs) != 1 || timeline.Logs[0]["message"] != "payment failed for order 7" || timeline.Logs[0]["level"] != "error" {
		t.Fatalf("logs = %v", timeline.Logs)
	}
	var kinds []string
	for _, a := range timeline.Actions {
		kinds = append(kinds, a["type"].(string)+":"+firstString(a, "name", "method", "action"))
	}
	want := []string{"navigation:push", "network:POST", "custom:add_item", "custom:checkout_failed", "navigation:hash"}
	if len(kinds) != len(want) {
		t.Fatalf("actions = %v, want %v", kinds, want)
	}
	for i := range want {
		if kinds[i] != want[i] {
			t.Fatalf("actions = %v, want %v", kinds, want)
		}
	}
	network := timeline.Actions[1]
	if network["statusCode"].(float64) != 500 || network["url"] != "https://shop.test/api/checkout" || network["traceId"] == "" || network["durationMs"].(float64) != 300 {
		t.Errorf("network action = %v", network)
	}
	if data := timeline.Actions[3]["data"].(map[string]any); data["step"] != "payment" || data["url"] != nil {
		t.Errorf("custom event data should keep its own attributes only: %v", data)
	}
	if hash := timeline.Actions[4]; hash["from"] != "https://shop.test/cart" || hash["to"] != "https://shop.test/cart#pay" {
		t.Errorf("hash navigation = %v", hash)
	}

	bad := httptest.NewRecorder()
	router.ServeHTTP(bad, httptest.NewRequest(http.MethodGet, "/api/sessions/"+session+"/timeline?from=2026-01-01T00:00:00Z&to=2026-01-02T00:00:00Z", nil))
	if bad.Code != http.StatusUnprocessableEntity {
		t.Errorf("a day-long window = %d, want 422", bad.Code)
	}
}

func firstString(m map[string]any, keys ...string) string {
	for _, key := range keys {
		if v, ok := m[key].(string); ok && v != "" {
			return v
		}
	}
	return ""
}

type timelineResponse struct {
	Logs      []map[string]any `json:"logs"`
	Actions   []map[string]any `json:"actions"`
	Truncated bool             `json:"truncated"`
}

func setupTimeline(t *testing.T, projectId uuid.UUID) *gin.Engine {
	t.Helper()
	dbtest.SetupSQLite(t)
	previous := config.Config
	config.Config = &config.Cfg{}
	t.Cleanup(func() { config.Config = previous })
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/sessions/:sessionId/timeline", func(c *gin.Context) {
		c.Set(middleware.ProjectIdContextKey, projectId)
	}, SessionTimelineController.Timeline)
	return router
}

func getTimeline(t *testing.T, router *gin.Engine, session string, from, to time.Time, extra string) (int, timelineResponse) {
	t.Helper()
	recorder := httptest.NewRecorder()
	query := "?from=" + from.Format(time.RFC3339Nano) + "&to=" + to.Format(time.RFC3339Nano) + extra
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/sessions/"+session+"/timeline"+query, nil))
	var timeline timelineResponse
	if recorder.Code == http.StatusOK {
		if err := json.Unmarshal(recorder.Body.Bytes(), &timeline); err != nil {
			t.Fatal(err)
		}
	}
	return recorder.Code, timeline
}

// SDKs may write the session id in any form uuid.Parse accepts; ingest stores the canonical one, which is what the
// timeline looks up.
func TestSessionTimelineFindsNonCanonicalSessionIds(t *testing.T) {
	projectId := uuid.New()
	router := setupTimeline(t, projectId)
	session := uuid.New()
	sent := kvString("session.id", "{"+strings.ToUpper(session.String())+"}")
	base := time.Now().UTC().Add(-time.Minute).Truncate(time.Millisecond)

	postOtlp(t, projectId, "/api/otel/v1/logs", &collogspb.ExportLogsServiceRequest{ResourceLogs: []*logspb.ResourceLogs{{
		ScopeLogs: []*logspb.ScopeLogs{{LogRecords: []*logspb.LogRecord{{
			EventName: "browser.console", TimeUnixNano: uint64(base.UnixNano()), SeverityNumber: 9, Attributes: []*commonpb.KeyValue{sent},
			Body: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "cart opened"}},
		}}}},
	}}}, otelcontrollers.OtelController.ExportLogs)
	traceID := uuid.New()
	postOtlp(t, projectId, "/api/otel/v1/traces", &coltracepb.ExportTraceServiceRequest{ResourceSpans: []*tracepb.ResourceSpans{{
		ScopeSpans: []*tracepb.ScopeSpans{{Spans: []*tracepb.Span{{
			TraceId: traceID[:], SpanId: []byte{1, 2, 3, 4, 5, 6, 7, 8}, Name: "GET", Kind: tracepb.Span_SPAN_KIND_CLIENT,
			StartTimeUnixNano: uint64(base.UnixNano()), EndTimeUnixNano: uint64(base.Add(time.Second).UnixNano()),
			Attributes: []*commonpb.KeyValue{sent, kvString("url.full", "https://shop.test/api/cart")},
		}}}},
	}}}, otelcontrollers.OtelController.ExportTraces)

	code, timeline := getTimeline(t, router, session.String(), base.Add(-time.Minute), base.Add(time.Minute), "")
	if code != http.StatusOK || len(timeline.Logs) != 1 || len(timeline.Actions) != 1 {
		t.Fatalf("timeline = %d %+v, want the console line and the request", code, timeline)
	}
}

// On a chatty page the window before an exception holds more breadcrumbs than one read returns; anchored at the end,
// the timeline keeps the newest and says it is truncated.
func TestSessionTimelineAnchoredAtTheEnd(t *testing.T) {
	projectId := uuid.New()
	router := setupTimeline(t, projectId)
	session := uuid.New().String()
	base := time.Now().UTC().Add(-time.Hour).Truncate(time.Millisecond)
	total := shared.SessionBreadcrumbLimit + 10
	records := make([]models.LogRecord, 0, total+1)
	for i := range total {
		records = append(records, models.LogRecord{Id: uuid.New(), ProjectId: projectId, Timestamp: base.Add(time.Duration(i) * time.Second),
			SeverityNumber: 9, Body: fmt.Sprintf("line %d", i), LogAttributes: map[string]string{"session.id": session, "event.name": "browser.console"}})
	}
	records = append(records, models.LogRecord{Id: uuid.New(), ProjectId: projectId, Timestamp: base.Add(time.Duration(total) * time.Second),
		LogAttributes: map[string]string{"session.id": session, "event.name": "traceway.session.update"}})
	if err := telemetry.LogRecordRepository.InsertAsync(context.Background(), records); err != nil {
		t.Fatal(err)
	}
	trace := uuid.New()
	var spans []models.OtelSpan
	for _, offset := range []int{5, total - 5} {
		start := base.Add(time.Duration(offset) * time.Second)
		spans = append(spans, models.OtelSpan{Span: models.Span{ProjectId: projectId, TraceId: hex.EncodeToString(trace[:]),
			SpanId: fmt.Sprintf("%016x", offset+1), Name: "GET", SpanKind: 3, StartTime: start, RecordedAt: start, Duration: time.Second,
			Attributes: map[string]string{"session.id": session, "url.full": fmt.Sprintf("/api/%d", offset)}}})
	}
	if _, err := telemetry.OtelSpanRepository.InsertAsync(context.Background(), spans); err != nil {
		t.Fatal(err)
	}
	from, to := base.Add(-time.Minute), base.Add(time.Duration(total+60)*time.Second)

	code, newest := getTimeline(t, router, session, from, to, "&anchor=end")
	if code != http.StatusOK || !newest.Truncated || len(newest.Logs) != shared.SessionBreadcrumbLimit {
		t.Fatalf("anchor=end = %d, %d logs, truncated %v", code, len(newest.Logs), newest.Truncated)
	}
	if first, last := newest.Logs[0]["message"], newest.Logs[len(newest.Logs)-1]["message"]; first != "line 10" || last != fmt.Sprintf("line %d", total-1) {
		t.Errorf("anchor=end kept %v .. %v, want the newest lines in time order", first, last)
	}
	if len(newest.Actions) != 1 || newest.Actions[0]["url"] != fmt.Sprintf("/api/%d", total-5) {
		t.Errorf("anchor=end actions = %v, want only the request inside the logs' window", newest.Actions)
	}

	code, oldest := getTimeline(t, router, session, from, to, "")
	if code != http.StatusOK || !oldest.Truncated || len(oldest.Logs) != shared.SessionBreadcrumbLimit || oldest.Logs[0]["message"] != "line 0" {
		t.Fatalf("default anchor = %d, %d logs from %v, truncated %v", code, len(oldest.Logs), oldest.Logs[0]["message"], oldest.Truncated)
	}
	if len(oldest.Actions) != 1 || oldest.Actions[0]["url"] != "/api/5" {
		t.Errorf("default anchor actions = %v, want only the request inside the logs' window", oldest.Actions)
	}

	if code, _ := getTimeline(t, router, session, from, to, "&anchor=middle"); code != http.StatusBadRequest {
		t.Errorf("anchor=middle = %d, want 400", code)
	}
}
