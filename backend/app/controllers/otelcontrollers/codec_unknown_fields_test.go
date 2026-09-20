package otelcontrollers

import (
	"bytes"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	colprofilespb "go.opentelemetry.io/proto/otlp/collector/profiles/v1development"
	"google.golang.org/protobuf/proto"
)

func jsonContext(path string, body string) *gin.Context {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", path, bytes.NewReader([]byte(body)))
	c.Request.Header.Set("Content-Type", "application/json")
	return c
}

func TestOTLPJSONSignalsIgnoreUnknownFields(t *testing.T) {
	metrics, _, err := decodeMetricsRequest(jsonContext("/api/otel/v1/metrics", `{"futureTopLevel":{"a":1},"resourceMetrics":[{"futureResourceField":true,"scopeMetrics":[{"metrics":[{"name":"requests","futureMetricField":[1,2],"gauge":{"dataPoints":[{"asInt":"7","futurePointField":"x"}]}}]}]}]}`))
	if err != nil {
		t.Fatalf("metrics: %v", err)
	}
	if point := metrics.ResourceMetrics[0].ScopeMetrics[0].Metrics[0].GetGauge().DataPoints[0]; point.GetAsInt() != 7 {
		t.Fatalf("metrics known fields changed: %v", point)
	}

	logs, _, err := decodeLogsRequest(jsonContext("/api/otel/v1/logs", `{"futureTopLevel":1,"resourceLogs":[{"futureResourceField":"x","scopeLogs":[{"logRecords":[{"severityText":"ERROR","body":{"stringValue":"boom"},"futureRecordField":{"nested":true}}]}]}]}`))
	if err != nil {
		t.Fatalf("logs: %v", err)
	}
	if record := logs.ResourceLogs[0].ScopeLogs[0].LogRecords[0]; record.SeverityText != "ERROR" || record.Body.GetStringValue() != "boom" {
		t.Fatalf("logs known fields changed: %v", record)
	}

	payload, _, err := decodeProfilesPayload(jsonContext("/api/otel/v1development/profiles", `{"futureTopLevel":true,"resourceProfiles":[{"futureResourceField":1,"scopeProfiles":[{"futureScopeField":"x"}]}]}`))
	if err != nil {
		t.Fatalf("profiles: %v", err)
	}
	var profiles colprofilespb.ExportProfilesServiceRequest
	if err := proto.Unmarshal(payload, &profiles); err != nil || len(profiles.ResourceProfiles) != 1 || len(profiles.ResourceProfiles[0].ScopeProfiles) != 1 {
		t.Fatalf("profiles known fields changed: %v %v", &profiles, err)
	}
}

func TestOTLPJSONSignalsStillRejectMalformedKnownFields(t *testing.T) {
	if _, _, err := decodeMetricsRequest(jsonContext("/api/otel/v1/metrics", `{"resourceMetrics":"not an array"}`)); err == nil {
		t.Fatal("metrics accepted a malformed known field")
	}
	if _, _, err := decodeLogsRequest(jsonContext("/api/otel/v1/logs", `{"resourceLogs":[{"scopeLogs":[{"logRecords":[{"timeUnixNano":{"not":"a number"}}]}]}]}`)); err == nil {
		t.Fatal("logs accepted a malformed known field")
	}
	if _, _, err := decodeProfilesPayload(jsonContext("/api/otel/v1development/profiles", `{"resourceProfiles":{"not":"an array"}}`)); err == nil {
		t.Fatal("profiles accepted a malformed known field")
	}
}

func TestLogJSONNormalizesOnlyContextIDs(t *testing.T) {
	body := `{"resourceLogs":[{"resource":{"attributes":[{"key":"traceId","value":{"stringValue":"resource ID"}}]},"scopeLogs":[{"scope":{"attributes":[{"key":"spanId","value":{"stringValue":"scope ID"}}]},"logRecords":[{"traceId":"0123456789ABCDEF0123456789ABCDEF","spanId":"ABCDEF0123456789","attributes":[{"key":"traceId","value":{"stringValue":"attribute ID"}}],"body":{"kvlistValue":{"values":[{"key":"spanId","value":{"stringValue":"body ID"}}]}},"futureField":true}]}]}]}`
	snakeCase := strings.NewReplacer(
		`"resourceLogs":`, `"resource_logs":`, `"scopeLogs":`, `"scope_logs":`,
		`"logRecords":`, `"log_records":`, `"traceId":`, `"trace_id":`, `"spanId":`, `"span_id":`,
	).Replace(body)
	for _, input := range []string{body, snakeCase} {
		req, _, err := decodeLogsRequest(jsonContext("/api/otel/v1/logs", input))
		if err != nil {
			t.Fatal(err)
		}
		resource := req.ResourceLogs[0]
		scope := resource.ScopeLogs[0]
		log := scope.LogRecords[0]
		if len(log.TraceId) != 16 || len(log.SpanId) != 8 {
			t.Fatalf("IDs were not decoded from hex: %v", log)
		}
		if getStringAttribute(resource.Resource.Attributes, "traceId") != "resource ID" ||
			getStringAttribute(scope.Scope.Attributes, "spanId") != "scope ID" ||
			getStringAttribute(log.Attributes, "traceId") != "attribute ID" ||
			getStringAttribute(log.Body.GetKvlistValue().Values, "spanId") != "body ID" {
			t.Fatalf("normalization changed user attributes or body: %v", req)
		}
	}
}

func TestLogJSONRejectsMalformedHexIDs(t *testing.T) {
	for _, id := range []string{`"abc"`, `"gggggggggggggggg"`, `42`} {
		body := `{"resourceLogs":[{"scopeLogs":[{"logRecords":[{"spanId":` + id + `}]}]}]}`
		if _, _, err := decodeLogsRequest(jsonContext("/api/otel/v1/logs", body)); err == nil {
			t.Fatalf("accepted malformed span ID %s", id)
		}
	}
}
