package otelcontrollers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/controllers/clientcontrollers"
	"github.com/tracewayapp/traceway/backend/app/models"
	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
)

type aiTraceConversation struct {
	StorageKey string
	Content    []byte
}

func convertTraces(projectId uuid.UUID, req *coltracepb.ExportTraceServiceRequest) (
	endpoints []models.Endpoint,
	tasks []models.Task,
	spans []models.Span,
	exceptions []models.ExceptionStackTrace,
	aiTraces []models.AiTrace,
	aiConversations []aiTraceConversation,
) {

	for _, rs := range req.ResourceSpans {
		resourceAttrs := rs.GetResource().GetAttributes()
		serverName := getStringAttribute(resourceAttrs, "service.name")
		appVersion := getStringAttribute(resourceAttrs, "service.version")
		if appVersion == "" {
			if scriptVersionId := getStringAttribute(resourceAttrs, "cloudflare.script_version.id"); scriptVersionId != "" {
				if idx := strings.LastIndex(scriptVersionId, "-"); idx != -1 {
					appVersion = scriptVersionId[idx+1:]
				}
			}
		}

		// First pass: collect all spans and build parent→child relationships
		type spanEntry struct {
			span      *tracepb.Span
			scopeName string
		}
		var allSpans []spanEntry
		parentMap := map[string]string{}        // childSpanId → parentSpanId
		spanById := map[string]*tracepb.Span{}  // spanId → span (for trace_id lookup)
		for _, ss := range rs.ScopeSpans {
			for _, span := range ss.Spans {
				allSpans = append(allSpans, spanEntry{span: span, scopeName: ss.GetScope().GetName()})
				spanById[string(span.SpanId)] = span
				if len(span.ParentSpanId) > 0 {
					parentMap[string(span.SpanId)] = string(span.ParentSpanId)
				}
			}
		}

		// Build spanToTraceId. Prefer each span's native OTel trace_id so logs on
		// the wire (which carry trace_id as hex) can be joined against the
		// converted traces via traceIdUuidToHex(endpoint.id). Fall back to walking
		// parents + minting a UUID only when the OTel trace_id is missing or
		// malformed — an unusual case but worth handling defensively.
		spanToTraceId := map[string]uuid.UUID{}
		var resolveTraceId func(spanIdStr string) uuid.UUID
		resolveTraceId = func(spanIdStr string) uuid.UUID {
			if tid, ok := spanToTraceId[spanIdStr]; ok {
				return tid
			}
			if sp, ok := spanById[spanIdStr]; ok && len(sp.TraceId) > 0 {
				if tid := otelTraceIDToUUID(sp.TraceId); tid != uuid.Nil {
					spanToTraceId[spanIdStr] = tid
					return tid
				}
			}
			if parentId, hasParent := parentMap[spanIdStr]; hasParent {
				tid := resolveTraceId(parentId)
				spanToTraceId[spanIdStr] = tid
				return tid
			}
			tid := uuid.New()
			spanToTraceId[spanIdStr] = tid
			return tid
		}
		for _, entry := range allSpans {
			resolveTraceId(string(entry.span.SpanId))
		}

		// Second pass: process spans and create records
		for _, entry := range allSpans {
			span := entry.span
			spanAttrs := span.Attributes
			allAttrs := extractAttributes(spanAttrs)

			isRoot := len(span.ParentSpanId) == 0
			traceId := spanToTraceId[string(span.SpanId)]

			spanId := otelSpanIDToUUID(span.SpanId)
			startTime := nanoToTime(span.StartTimeUnixNano)
			endTime := nanoToTime(span.EndTimeUnixNano)
			duration := endTime.Sub(startTime)

			var distributedTraceId *uuid.UUID
			if dtid := getStringAttribute(spanAttrs, "traceway.distributed_trace_id"); dtid != "" {
				if parsed, err := uuid.Parse(dtid); err == nil {
					distributedTraceId = &parsed
				}
			}

			if isRoot {
				rootSpanId := spanId
				if (span.Kind == tracepb.Span_SPAN_KIND_SERVER || span.Kind == tracepb.Span_SPAN_KIND_INTERNAL) && hasHTTPAttributes(spanAttrs) {
					ep := buildEndpoint(
						traceId, projectId, span, spanAttrs, allAttrs,
						startTime, duration, serverName, appVersion,
					)
					ep.DistributedTraceId = distributedTraceId
					ep.SpanId = &rootSpanId
					endpoints = append(endpoints, ep)
				} else if span.Kind == tracepb.Span_SPAN_KIND_CONSUMER {
					t := buildTask(
						traceId, projectId, span, allAttrs,
						startTime, endTime, duration, serverName, appVersion,
					)
					t.DistributedTraceId = distributedTraceId
					t.SpanId = &rootSpanId
					tasks = append(tasks, t)
				} else if hasGenAiAttributes(spanAttrs) {
					aiTrace := buildAiTrace(
						traceId, projectId, span, spanAttrs, allAttrs,
						startTime, duration, serverName, appVersion,
					)
					aiTraces = append(aiTraces, aiTrace)
					if conv := extractConversation(spanAttrs, projectId, traceId); conv != nil {
						aiConversations = append(aiConversations, *conv)
					}
				} else {
					continue
				}
			} else {
				spanName := span.Name
				if dbQuery := getStringAttribute(spanAttrs, "db.query.text"); dbQuery != "" {
					spanName = dbQuery
				} else if dbStatement := getStringAttribute(spanAttrs, "db.statement"); dbStatement != "" {
					spanName = dbStatement
				}

				spans = append(spans, models.Span{
					Id:           spanId,
					TraceId:      traceId,
					ProjectId:    projectId,
					Name:         spanName,
					StartTime:    startTime,
					Duration:     duration,
					RecordedAt:   startTime,
					ParentSpanId: ptrSpanUUID(span.ParentSpanId),
				})
			}

			traceType := "task"
			if isRoot && (span.Kind == tracepb.Span_SPAN_KIND_SERVER || span.Kind == tracepb.Span_SPAN_KIND_INTERNAL) && hasHTTPAttributes(spanAttrs) {
				traceType = "endpoint"
			}

			for _, event := range span.Events {
				if event.Name == "exception" {
					exc := buildException(
						projectId, traceId, traceType, event,
						allAttrs, serverName, appVersion,
					)
					exc.DistributedTraceId = distributedTraceId
					exceptions = append(exceptions, exc)
				}
			}
		}
	}
	return
}

func hasHTTPAttributes(attrs []*commonpb.KeyValue) bool {
	for _, kv := range attrs {
		switch kv.Key {
		case "http.request.method", "http.method", "http.route", "url.path":
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

	if statusCode == 404 {
		endpoint = "UNMATCHED"
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
		IsStream:   isOtelStreamingEndpoint(attrs, statusCode),
	}
}

// isOtelStreamingEndpoint detects long-lived streaming responses on OTel spans:
//   - status 101 (WebSocket upgrade)
//   - http.response.header.content-type contains text/event-stream (SSE)
//
// OTel has no standard `is_stream` attribute, so we sniff the captured headers.
// Clients that don't capture `http.response.header.content-type` won't trigger
// SSE detection — they can fall back to a vendor extension attribute
// `traceway.is_stream` (boolean) or the WebSocket signal.
func isOtelStreamingEndpoint(attrs []*commonpb.KeyValue, statusCode int16) bool {
	if statusCode == http.StatusSwitchingProtocols {
		return true
	}
	if b, ok := getBoolAttribute(attrs, "traceway.is_stream"); ok && b {
		return true
	}
	for _, ct := range getStringValues(attrs, "http.response.header.content-type") {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(ct)), "text/event-stream") {
			return true
		}
	}
	return false
}

func getBoolAttribute(attrs []*commonpb.KeyValue, key string) (bool, bool) {
	for _, kv := range attrs {
		if kv.Key == key && kv.Value != nil {
			if bv, ok := kv.Value.Value.(*commonpb.AnyValue_BoolValue); ok {
				return bv.BoolValue, true
			}
		}
	}
	return false, false
}

func getHTTPEndpoint(attrs []*commonpb.KeyValue, fallback string) string {
	method := getStringAttribute(attrs, "http.request.method")
	if method == "" {
		method = getStringAttribute(attrs, "http.method")
	}
	route := getStringAttribute(attrs, "http.route")
	if route != "" && !strings.HasPrefix(route, "/") {
		route = ""
	}
	if route == "" {
		route = getStringAttribute(attrs, "url.path")
	}

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
	startTime, endTime time.Time,
	duration time.Duration,
	serverName, appVersion string,
) models.Task {
	return models.Task{
		Id:         id,
		ProjectId:  projectId,
		TaskName:   span.Name,
		Duration:   duration,
		RecordedAt: endTime,
		Attributes: allAttrs,
		AppVersion: appVersion,
		ServerName: serverName,
	}
}

func buildException(
	projectId, traceId uuid.UUID,
	traceType string,
	event *tracepb.Span_Event,
	spanAttrs map[string]string,
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
		Attributes:    spanAttrs,
		AppVersion:    appVersion,
		ServerName:    serverName,
	}
}

func hasGenAiAttributes(attrs []*commonpb.KeyValue) bool {
	for _, kv := range attrs {
		if strings.HasPrefix(kv.Key, "gen_ai.") {
			return true
		}
	}
	return false
}

func buildAiTrace(
	id, projectId uuid.UUID,
	span *tracepb.Span,
	attrs []*commonpb.KeyValue,
	allAttrs map[string]string,
	startTime time.Time,
	duration time.Duration,
	serverName, appVersion string,
) models.AiTrace {
	model := getStringAttribute(attrs, "gen_ai.request.model")
	responseModel := getStringAttribute(attrs, "gen_ai.response.model")
	provider := getStringAttribute(attrs, "gen_ai.system")
	if provider == "" {
		provider = getStringAttribute(attrs, "gen_ai.provider.name")
	}
	operation := getStringAttribute(attrs, "gen_ai.operation.name")

	inputTokens, _ := getIntAttribute(attrs, "gen_ai.usage.input_tokens")
	outputTokens, _ := getIntAttribute(attrs, "gen_ai.usage.output_tokens")
	totalTokens, hasTotalTokens := getIntAttribute(attrs, "gen_ai.usage.total_tokens")
	if !hasTotalTokens {
		totalTokens = inputTokens + outputTokens
	}
	cachedTokens, _ := getIntAttribute(attrs, "gen_ai.usage.input_tokens.cached")
	reasoningTokens, _ := getIntAttribute(attrs, "gen_ai.usage.output_tokens.reasoning")

	inputCost := getFloatAttribute(attrs, "gen_ai.usage.input_cost")
	outputCost := getFloatAttribute(attrs, "gen_ai.usage.output_cost")
	totalCost := getFloatAttribute(attrs, "gen_ai.usage.total_cost")
	if totalCost == 0 {
		totalCost = inputCost + outputCost
	}

	traceName := getStringAttribute(attrs, "trace.name")
	if traceName == "" {
		traceName = span.Name
	}

	userId := getStringAttribute(attrs, "user.id")
	finishReason := getStringAttribute(attrs, "gen_ai.response.finish_reason")
	if finishReason == "" {
		finishReason = getStringAttribute(attrs, "gen_ai.response.finish_reasons")
	}

	statusCode := uint8(span.Status.GetCode())
	storageKey := fmt.Sprintf("ai-traces/%s/%s.json", projectId, id)

	filteredAttrs := filterNonStandardAiAttrs(allAttrs)

	return models.AiTrace{
		Id:              id,
		ProjectId:       projectId,
		RecordedAt:      startTime,
		Duration:        duration,
		StatusCode:      statusCode,
		Model:           model,
		ResponseModel:   responseModel,
		Provider:        provider,
		Operation:       operation,
		InputTokens:     inputTokens,
		OutputTokens:    outputTokens,
		TotalTokens:     totalTokens,
		CachedTokens:    cachedTokens,
		ReasoningTokens: reasoningTokens,
		InputCost:       inputCost,
		OutputCost:      outputCost,
		TotalCost:       totalCost,
		TraceName:       traceName,
		UserId:          userId,
		FinishReason:    finishReason,
		ServerName:      serverName,
		AppVersion:      appVersion,
		StorageKey:      storageKey,
		Attributes:      filteredAttrs,
	}
}

var standardAiAttrPrefixes = []string{
	"gen_ai.request.model",
	"gen_ai.response.model",
	"gen_ai.system",
	"gen_ai.provider.name",
	"gen_ai.operation.name",
	"gen_ai.usage.",
	"gen_ai.prompt",
	"gen_ai.completion",
	"gen_ai.response.finish_reason",
	"gen_ai.response.finish_reasons",
	"trace.name",
	"trace.input",
	"trace.output",
	"span.input",
	"span.output",
	"user.id",
}

func filterNonStandardAiAttrs(allAttrs map[string]string) map[string]string {
	if len(allAttrs) == 0 {
		return nil
	}
	result := make(map[string]string)
	for k, v := range allAttrs {
		standard := false
		for _, prefix := range standardAiAttrPrefixes {
			if k == prefix || strings.HasPrefix(k, prefix) {
				standard = true
				break
			}
		}
		if !standard {
			result[k] = v
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func extractConversation(attrs []*commonpb.KeyValue, projectId, traceId uuid.UUID) *aiTraceConversation {
	input := getStringAttribute(attrs, "gen_ai.prompt")
	if input == "" {
		input = getStringAttribute(attrs, "trace.input")
	}
	if input == "" {
		input = getStringAttribute(attrs, "span.input")
	}

	output := getStringAttribute(attrs, "gen_ai.completion")
	if output == "" {
		output = getStringAttribute(attrs, "trace.output")
	}
	if output == "" {
		output = getStringAttribute(attrs, "span.output")
	}

	if input == "" && output == "" {
		return nil
	}

	content := map[string]string{
		"input":  input,
		"output": output,
	}
	data, err := json.Marshal(content)
	if err != nil {
		return nil
	}

	return &aiTraceConversation{
		StorageKey: fmt.Sprintf("ai-traces/%s/%s.json", projectId, traceId),
		Content:    data,
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
		// JVM OTel agents embed the exception class name as the first line of the
		// stacktrace (e.g. "java.lang.RuntimeException: msg\n\tat ..."). Skip
		// prepending the header when it's already there to avoid a duplicate line.
		if header != "" && (excType == "" || !strings.HasPrefix(excStacktrace, excType)) {
			return fmt.Sprintf("%s\n%s", header, excStacktrace)
		}
		return excStacktrace
	}
	if header != "" {
		return header
	}
	return "unknown exception"
}
