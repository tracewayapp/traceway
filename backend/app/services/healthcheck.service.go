package services

import (
	"strings"

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

// SpanKey names one span of one trace inside a payload.
func SpanKey(traceId, spanId string) string { return traceId + ":" + spanId }

// FilterHealthchecks drops healthy healthcheck endpoints and returns the spans they were promoted from, keyed by
// SpanKey. A healthcheck whose trace carries an exception in the same payload is kept.
func FilterHealthchecks(project *models.Project, endpoints []models.Endpoint, exceptions []models.ExceptionStackTrace) ([]models.Endpoint, map[string]bool) {
	if project == nil || !project.DropHealthyHealthchecks || len(endpoints) == 0 {
		return endpoints, nil
	}
	failing := map[string]bool{}
	for _, exception := range exceptions {
		if exception.TraceId != "" {
			failing[exception.TraceId] = true
		}
	}
	dropped := map[string]bool{}
	kept := endpoints[:0]
	for _, endpoint := range endpoints {
		if ShouldDropHealthcheck(project, endpoint.Endpoint, endpoint.StatusCode) && !failing[endpoint.TraceId] {
			dropped[SpanKey(endpoint.TraceId, endpoint.SpanId)] = true
			continue
		}
		kept = append(kept, endpoint)
	}
	return kept, dropped
}

// DropSpanSubtrees removes the dropped spans and everything under them, stopping at a retained span: a task or an AI
// trace started by a healthcheck keeps its own spans.
func DropSpanSubtrees[T any](spans []T, ids func(T) (traceId, spanId, parentSpanId string), dropped, retained map[string]bool) []T {
	if len(dropped) == 0 {
		return spans
	}
	children := make(map[string][]int)
	pending := make([]int, 0)
	for i, span := range spans {
		traceId, spanId, parentSpanId := ids(span)
		if dropped[SpanKey(traceId, spanId)] {
			pending = append(pending, i)
		}
		if parentSpanId != "" {
			children[SpanKey(traceId, parentSpanId)] = append(children[SpanKey(traceId, parentSpanId)], i)
		}
	}
	removed := make(map[int]bool)
	for len(pending) > 0 {
		i := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		traceId, spanId, _ := ids(spans[i])
		if removed[i] || retained[SpanKey(traceId, spanId)] {
			continue
		}
		removed[i] = true
		pending = append(pending, children[SpanKey(traceId, spanId)]...)
	}
	kept := spans[:0]
	for i, span := range spans {
		if !removed[i] {
			kept = append(kept, span)
		}
	}
	return kept
}
