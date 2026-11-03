//go:build !transactional_pg && !telemetry_ch && !telemetry_duckdb

package controllers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/config"
	"github.com/tracewayapp/traceway/backend/app/controllers/otelcontrollers"
	"github.com/tracewayapp/traceway/backend/app/dbtest"
	"github.com/tracewayapp/traceway/backend/app/middleware"
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

	trace := uuid.New()
	postOtlp(t, projectId, "/api/otel/v1/traces", &coltracepb.ExportTraceServiceRequest{ResourceSpans: []*tracepb.ResourceSpans{{
		Resource: &resourcepb.Resource{Attributes: []*commonpb.KeyValue{kvString("service.name", "shop-web")}},
		ScopeSpans: []*tracepb.ScopeSpans{{Spans: []*tracepb.Span{{
			TraceId: trace[:], SpanId: []byte{1, 2, 3, 4, 5, 6, 7, 8}, Name: "POST", Kind: tracepb.Span_SPAN_KIND_CLIENT,
			StartTimeUnixNano: at(1500 * time.Millisecond), EndTimeUnixNano: at(1800 * time.Millisecond),
			Attributes: []*commonpb.KeyValue{sid, kvString("http.request.method", "POST"),
				kvString("url.full", "https://shop.test/api/checkout"), kvInt("http.response.status_code", 500)},
		}}}},
	}}}, otelcontrollers.OtelController.ExportTraces)

	router := gin.New()
	router.GET("/api/sessions/:sessionId/timeline", func(c *gin.Context) {
		c.Set(middleware.ProjectIdContextKey, projectId)
	}, SessionTimelineController.Timeline)
	recorder := httptest.NewRecorder()
	query := "?from=" + base.Add(-time.Minute).Format(time.RFC3339Nano) + "&to=" + base.Add(time.Minute).Format(time.RFC3339Nano)
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/sessions/"+session+"/timeline"+query, nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("timeline = %d %s", recorder.Code, recorder.Body.String())
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
