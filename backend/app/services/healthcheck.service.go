package services

import (
	"strings"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/models"
)

var defaultHealthcheckPaths = map[string]bool{
	"/health":          true,
	"/healthz":         true,
	"/healthcheck":     true,
	"/health-check":    true,
	"/health_check":    true,
	"/ping":            true,
	"/livez":           true,
	"/readyz":          true,
	"/live":            true,
	"/ready":           true,
	"/alive":           true,
	"/up":              true,
	"/heartbeat":       true,
	"/status":          true,
	"/ht":              true,
	"/actuator/health": true,
}

func ShouldDropHealthcheck(project *models.Project, endpoint string, statusCode int16) bool {
	if project == nil || !project.DropHealthyHealthchecks {
		return false
	}
	if statusCode >= 400 {
		return false
	}
	method, path, found := strings.Cut(endpoint, " ")
	if !found || (method != "GET" && method != "HEAD") {
		return false
	}
	return isHealthcheckPath(path, project.HealthcheckPaths)
}

func isHealthcheckPath(path string, customPaths []string) bool {
	path = strings.ToLower(strings.TrimSpace(path))
	if len(path) > 1 {
		path = strings.TrimRight(path, "/")
	}
	if defaultHealthcheckPaths[path] {
		return true
	}
	if strings.HasPrefix(path, "/actuator/health/") {
		return true
	}
	if strings.HasSuffix(path, "/health") {
		return true
	}
	for _, custom := range customPaths {
		if matchesCustomPath(path, strings.ToLower(strings.TrimSpace(custom))) {
			return true
		}
	}
	return false
}

func matchesCustomPath(path, pattern string) bool {
	if pattern == "" || pattern == "*" {
		return false
	}
	startsWithStar := strings.HasPrefix(pattern, "*")
	endsWithStar := strings.HasSuffix(pattern, "*")
	if startsWithStar && endsWithStar {
		return strings.Contains(path, pattern[1:len(pattern)-1])
	}
	if startsWithStar {
		return strings.HasSuffix(path, pattern[1:])
	}
	if endsWithStar {
		return strings.HasPrefix(path, pattern[:len(pattern)-1])
	}
	if len(pattern) > 1 {
		pattern = strings.TrimRight(pattern, "/")
	}
	return path == pattern
}

func FilterHealthchecks(project *models.Project, endpoints []models.Endpoint, spans []models.Span, exceptions []models.ExceptionStackTrace) ([]models.Endpoint, []models.Span, int) {
	if project == nil || !project.DropHealthyHealthchecks || len(endpoints) == 0 {
		return endpoints, spans, 0
	}

	dropped := map[uuid.UUID]bool{}
	for _, e := range endpoints {
		if ShouldDropHealthcheck(project, e.Endpoint, e.StatusCode) {
			dropped[e.Id] = true
		}
	}
	if len(dropped) == 0 {
		return endpoints, spans, 0
	}

	for _, exc := range exceptions {
		if exc.TraceId != nil {
			delete(dropped, *exc.TraceId)
		}
	}
	if len(dropped) == 0 {
		return endpoints, spans, 0
	}

	keptEndpoints := endpoints[:0]
	for _, e := range endpoints {
		if !dropped[e.Id] {
			keptEndpoints = append(keptEndpoints, e)
		}
	}
	keptSpans := spans[:0]
	for _, s := range spans {
		if !dropped[s.TraceId] {
			keptSpans = append(keptSpans, s)
		}
	}
	return keptEndpoints, keptSpans, len(dropped)
}
