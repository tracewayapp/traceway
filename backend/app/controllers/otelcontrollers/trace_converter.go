package otelcontrollers

import (
	"backend/app/controllers/clientcontrollers"
	"backend/app/models"
	"fmt"
	"time"

	"github.com/google/uuid"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
)

func convertTraces(projectId uuid.UUID, req *coltracepb.ExportTraceServiceRequest) (
	endpoints []models.Endpoint,
	tasks []models.Task,
	spans []models.Span,
	exceptions []models.ExceptionStackTrace,
) {
	for _, rs := range req.ResourceSpans {
		resourceAttrs := rs.GetResource().GetAttributes()
		serverName := getStringAttribute(resourceAttrs, "service.name")
		appVersion := getStringAttribute(resourceAttrs, "service.version")

		for _, ss := range rs.ScopeSpans {
			for _, span := range ss.Spans {
				traceId := otelTraceIDToUUID(span.TraceId)
				spanId := otelSpanIDToUUID(span.SpanId)
				startTime := nanoToTime(span.StartTimeUnixNano)
				endTime := nanoToTime(span.EndTimeUnixNano)
				duration := endTime.Sub(startTime)
				spanAttrs := span.Attributes
				allAttrs := extractAttributes(spanAttrs)
				isRoot := len(span.ParentSpanId) == 0

				if isRoot {
					if span.Kind == tracepb.Span_SPAN_KIND_SERVER && hasHTTPAttributes(spanAttrs) {
						endpoints = append(endpoints, buildEndpoint(
							traceId, projectId, span, spanAttrs, allAttrs,
							startTime, duration, serverName, appVersion,
						))
					} else {
						tasks = append(tasks, buildTask(
							traceId, projectId, span, allAttrs,
							startTime, duration, serverName, appVersion,
						))
					}
				} else {
					spans = append(spans, models.Span{
						Id:         spanId,
						TraceId:    traceId,
						ProjectId:  projectId,
						Name:       span.Name,
						StartTime:  startTime,
						Duration:   duration,
						RecordedAt: startTime,
					})
				}

				traceType := "task"
				if isRoot && span.Kind == tracepb.Span_SPAN_KIND_SERVER && hasHTTPAttributes(spanAttrs) {
					traceType = "endpoint"
				}

				for _, event := range span.Events {
					if event.Name == "exception" {
						exc := buildException(
							projectId, traceId, traceType, event,
							serverName, appVersion,
						)
						exceptions = append(exceptions, exc)
					}
				}
			}
		}
	}
	return
}

func hasHTTPAttributes(attrs []*commonpb.KeyValue) bool {
	for _, kv := range attrs {
		switch kv.Key {
		case "http.request.method", "http.method", "http.route":
			return true
		}
	}
	return false
}

func buildEndpoint(
	id, projectId uuid.UUID,
	span *tracepb.Span,
	attrs []*commonpb.KeyValue,
	allAttrs map[string]string,
	startTime time.Time,
	duration time.Duration,
	serverName, appVersion string,
) models.Endpoint {
	endpoint := getHTTPEndpoint(attrs, span.Name)

	statusCode := int16(0)
	if code, ok := getIntAttribute(attrs, "http.response.status_code"); ok {
		statusCode = int16(code)
	} else if code, ok := getIntAttribute(attrs, "http.status_code"); ok {
		statusCode = int16(code)
	}

	bodySize := int32(0)
	if size, ok := getIntAttribute(attrs, "http.response.body.size"); ok {
		bodySize = int32(size)
	} else if size, ok := getIntAttribute(attrs, "http.response_content_length"); ok {
		bodySize = int32(size)
	}

	clientIP := getStringAttribute(attrs, "client.address")
	if clientIP == "" {
		clientIP = getStringAttribute(attrs, "net.peer.ip")
	}

	return models.Endpoint{
		Id:         id,
		ProjectId:  projectId,
		Endpoint:   endpoint,
		Duration:   duration,
		RecordedAt: startTime,
		StatusCode: statusCode,
		BodySize:   bodySize,
		ClientIP:   clientIP,
		Attributes: allAttrs,
		AppVersion: appVersion,
		ServerName: serverName,
	}
}

func getHTTPEndpoint(attrs []*commonpb.KeyValue, fallback string) string {
	method := getStringAttribute(attrs, "http.request.method")
	if method == "" {
		method = getStringAttribute(attrs, "http.method")
	}
	route := getStringAttribute(attrs, "http.route")

	if method != "" && route != "" {
		return method + " " + route
	}
	if method != "" {
		return method + " " + fallback
	}
	return fallback
}

func buildTask(
	id, projectId uuid.UUID,
	span *tracepb.Span,
	allAttrs map[string]string,
	startTime time.Time,
	duration time.Duration,
	serverName, appVersion string,
) models.Task {
	return models.Task{
		Id:         id,
		ProjectId:  projectId,
		TaskName:   span.Name,
		Duration:   duration,
		RecordedAt: startTime,
		Attributes: allAttrs,
		AppVersion: appVersion,
		ServerName: serverName,
	}
}

func buildException(
	projectId, traceId uuid.UUID,
	traceType string,
	event *tracepb.Span_Event,
	serverName, appVersion string,
) models.ExceptionStackTrace {
	eventAttrs := event.Attributes
	excType := getStringAttribute(eventAttrs, "exception.type")
	excMessage := getStringAttribute(eventAttrs, "exception.message")
	excStacktrace := getStringAttribute(eventAttrs, "exception.stacktrace")

	stackTrace := formatExceptionStackTrace(excType, excMessage, excStacktrace)
	hash := clientcontrollers.ComputeExceptionHash(stackTrace, false)

	return models.ExceptionStackTrace{
		Id:            uuid.New(),
		ProjectId:     projectId,
		TraceId:       &traceId,
		TraceType:     traceType,
		ExceptionHash: hash,
		StackTrace:    stackTrace,
		RecordedAt:    nanoToTime(event.TimeUnixNano),
		AppVersion:    appVersion,
		ServerName:    serverName,
	}
}

func formatExceptionStackTrace(excType, excMessage, excStacktrace string) string {
	header := excType
	if excMessage != "" {
		if header != "" {
			header += ": " + excMessage
		} else {
			header = excMessage
		}
	}
	if excStacktrace != "" {
		if header != "" {
			return fmt.Sprintf("%s\n%s", header, excStacktrace)
		}
		return excStacktrace
	}
	if header != "" {
		return header
	}
	return "unknown exception"
}
