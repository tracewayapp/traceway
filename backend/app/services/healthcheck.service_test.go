package services

import (
	"testing"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/models"
)

func TestShouldDropHealthcheck(t *testing.T) {
	enabled := &models.Project{DropHealthyHealthchecks: true}
	disabled := &models.Project{DropHealthyHealthchecks: false}
	withCustom := &models.Project{
		DropHealthyHealthchecks: true,
		HealthcheckPaths:        models.StringSlice{"/internal/probe", "/checks/*", "*/liveness"},
	}

	tests := []struct {
		name       string
		project    *models.Project
		endpoint   string
		statusCode int16
		expected   bool
	}{
		{"nil project", nil, "GET /health", 200, false},
		{"disabled", disabled, "GET /health", 200, false},
		{"healthy default path", enabled, "GET /health", 200, true},
		{"healthz", enabled, "GET /healthz", 200, true},
		{"healthcheck", enabled, "GET /healthcheck", 204, true},
		{"hyphenated", enabled, "GET /health-check", 200, true},
		{"underscored", enabled, "GET /health_check", 200, true},
		{"ping", enabled, "GET /ping", 200, true},
		{"livez", enabled, "GET /livez", 200, true},
		{"readyz", enabled, "GET /readyz", 200, true},
		{"live", enabled, "GET /live", 200, true},
		{"ready", enabled, "GET /ready", 200, true},
		{"alive", enabled, "GET /alive", 200, true},
		{"rails up", enabled, "GET /up", 200, true},
		{"heartbeat", enabled, "GET /heartbeat", 200, true},
		{"status", enabled, "GET /status", 200, true},
		{"django ht", enabled, "GET /ht", 200, true},
		{"django ht trailing slash", enabled, "GET /ht/", 200, true},
		{"actuator", enabled, "GET /actuator/health", 200, true},
		{"actuator liveness", enabled, "GET /actuator/health/liveness", 200, true},
		{"prefixed health suffix", enabled, "GET /api/health", 200, true},
		{"deeply prefixed health", enabled, "GET /api/v1/health", 200, true},
		{"head method", enabled, "HEAD /health", 200, true},
		{"uppercase path", enabled, "GET /HEALTH", 200, true},
		{"redirect status kept dropped", enabled, "GET /health", 301, true},
		{"failing healthcheck kept", enabled, "GET /health", 503, false},
		{"client error kept", enabled, "GET /health", 404, false},
		{"post not dropped", enabled, "POST /health", 200, false},
		{"no method prefix", enabled, "/health", 200, false},
		{"unmatched", enabled, "UNMATCHED", 200, false},
		{"regular endpoint", enabled, "GET /api/users", 200, false},
		{"healthy substring not matched", enabled, "GET /healthyrecipes", 200, false},
		{"shipping not ping", enabled, "GET /api/shipping", 200, false},
		{"custom exact", withCustom, "GET /internal/probe", 200, true},
		{"custom prefix wildcard", withCustom, "GET /checks/db", 200, true},
		{"custom suffix wildcard", withCustom, "GET /svc/liveness", 200, true},
		{"custom no match", withCustom, "GET /internal/other", 200, false},
		{"custom failing kept", withCustom, "GET /internal/probe", 500, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ShouldDropHealthcheck(tt.project, tt.endpoint, tt.statusCode)
			if result != tt.expected {
				t.Errorf("ShouldDropHealthcheck(%q, %d) = %v, expected %v", tt.endpoint, tt.statusCode, result, tt.expected)
			}
		})
	}
}

func TestFilterHealthchecks(t *testing.T) {
	project := &models.Project{DropHealthyHealthchecks: true}

	endpoints := []models.Endpoint{
		{Id: uuid.New(), TraceId: "t-healthy", SpanId: "e1", Endpoint: "GET /health", StatusCode: 200},
		{Id: uuid.New(), TraceId: "t-failing", SpanId: "e2", Endpoint: "GET /health", StatusCode: 503},
		{Id: uuid.New(), TraceId: "t-regular", SpanId: "e3", Endpoint: "GET /api/users", StatusCode: 200},
		{Id: uuid.New(), TraceId: "t-exception", SpanId: "e4", Endpoint: "GET /healthz", StatusCode: 200},
	}
	exceptions := []models.ExceptionStackTrace{
		{Id: uuid.New(), TraceId: "t-exception", SpanId: "e4-child"},
	}

	kept, dropped := FilterHealthchecks(project, endpoints, exceptions)

	if len(dropped) != 1 || !dropped[SpanKey("t-healthy", "e1")] {
		t.Errorf("dropped = %v, expected only the healthy healthcheck", dropped)
	}
	if len(kept) != 3 {
		t.Fatalf("len(kept) = %d, expected 3", len(kept))
	}
	for _, e := range kept {
		if e.TraceId == "t-healthy" {
			t.Errorf("healthy healthcheck endpoint was not dropped")
		}
	}
}

func TestDropSpanSubtrees(t *testing.T) {
	spans := []models.Span{
		{TraceId: "t-healthy", SpanId: "e1"},
		{TraceId: "t-healthy", SpanId: "db", ParentSpanId: "e1"},
		{TraceId: "t-healthy", SpanId: "row", ParentSpanId: "db"},
		{TraceId: "t-healthy", SpanId: "job", ParentSpanId: "e1"},
		{TraceId: "t-healthy", SpanId: "job-child", ParentSpanId: "job"},
		{TraceId: "t-regular", SpanId: "e1"},
	}
	ids := func(span models.Span) (string, string, string) { return span.TraceId, span.SpanId, span.ParentSpanId }

	kept := DropSpanSubtrees(spans, ids, map[string]bool{SpanKey("t-healthy", "e1"): true}, map[string]bool{SpanKey("t-healthy", "job"): true})

	got := ""
	for _, span := range kept {
		got += span.TraceId + "/" + span.SpanId + " "
	}
	if got != "t-healthy/job t-healthy/job-child t-regular/e1 " {
		t.Fatalf("a dropped healthcheck takes its own spans and stops at a task it started, in its own trace only: %s", got)
	}
}

func TestFilterHealthchecksDisabled(t *testing.T) {
	project := &models.Project{DropHealthyHealthchecks: false}
	endpoints := []models.Endpoint{
		{Id: uuid.New(), Endpoint: "GET /health", StatusCode: 200},
	}

	kept, dropped := FilterHealthchecks(project, endpoints, nil)

	if len(dropped) != 0 || len(kept) != 1 {
		t.Errorf("disabled filter dropped endpoints: kept=%d dropped=%d", len(kept), len(dropped))
	}
}
