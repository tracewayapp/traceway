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

	healthyId := uuid.New()
	failingId := uuid.New()
	regularId := uuid.New()
	excId := uuid.New()

	endpoints := []models.Endpoint{
		{Id: healthyId, Endpoint: "GET /health", StatusCode: 200},
		{Id: failingId, Endpoint: "GET /health", StatusCode: 503},
		{Id: regularId, Endpoint: "GET /api/users", StatusCode: 200},
		{Id: excId, Endpoint: "GET /healthz", StatusCode: 200},
	}
	spans := []models.Span{
		{Id: uuid.New(), TraceId: healthyId},
		{Id: uuid.New(), TraceId: regularId},
		{Id: uuid.New(), TraceId: excId},
	}
	exceptions := []models.ExceptionStackTrace{
		{Id: uuid.New(), TraceId: &excId},
	}

	keptEndpoints, keptSpans, dropped := FilterHealthchecks(project, endpoints, spans, exceptions)

	if dropped != 1 {
		t.Errorf("dropped = %d, expected 1", dropped)
	}
	if len(keptEndpoints) != 3 {
		t.Fatalf("len(keptEndpoints) = %d, expected 3", len(keptEndpoints))
	}
	for _, e := range keptEndpoints {
		if e.Id == healthyId {
			t.Errorf("healthy healthcheck endpoint was not dropped")
		}
	}
	if len(keptSpans) != 2 {
		t.Fatalf("len(keptSpans) = %d, expected 2", len(keptSpans))
	}
	for _, s := range keptSpans {
		if s.TraceId == healthyId {
			t.Errorf("span of dropped healthcheck was not dropped")
		}
	}
}

func TestFilterHealthchecksDisabled(t *testing.T) {
	project := &models.Project{DropHealthyHealthchecks: false}
	endpoints := []models.Endpoint{
		{Id: uuid.New(), Endpoint: "GET /health", StatusCode: 200},
	}

	keptEndpoints, _, dropped := FilterHealthchecks(project, endpoints, nil, nil)

	if dropped != 0 || len(keptEndpoints) != 1 {
		t.Errorf("disabled filter dropped endpoints: kept=%d dropped=%d", len(keptEndpoints), dropped)
	}
}
