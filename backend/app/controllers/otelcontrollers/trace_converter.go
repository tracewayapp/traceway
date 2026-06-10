package otelcontrollers

import (
	"context"
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

type entityKind int

const (
	entityNone entityKind = iota
	entityEndpoint
	entityTask
	entityAiTrace
)

func (k entityKind) traceType() string {
	switch k {
	case entityEndpoint:
		return "endpoint"
	case entityTask:
		return "task"
	case entityAiTrace:
		return "ai_trace"
	}
	return ""
}

func convertTraces(ctx context.Context, projectId uuid.UUID, req *coltracepb.ExportTraceServiceRequest) (
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
		language := getStringAttribute(resourceAttrs, "telemetry.sdk.language")
		if appVersion == "" {
			if scriptVersionId := getStringAttribute(resourceAttrs, "cloudflare.script_version.id"); scriptVersionId != "" {
				if idx := strings.LastIndex(scriptVersionId, "-"); idx != -1 {
					appVersion = scriptVersionId[idx+1:]
				}
			}
		}

		type spanEntry struct {
			span      *tracepb.Span
			scopeName string
		}
		var allSpans []spanEntry
		parentMap := map[string]string{}
		spanById := map[string]*tracepb.Span{}
		for _, ss := range rs.ScopeSpans {
			for _, span := range ss.Spans {
				allSpans = append(allSpans, spanEntry{span: span, scopeName: ss.GetScope().GetName()})
				spanById[string(span.SpanId)] = span
				if len(span.ParentSpanId) > 0 {
					parentMap[string(span.SpanId)] = string(span.ParentSpanId)
				}
			}
		}

		// Pass 1: classify each span by Kind/attrs and assign an entity id.
		// Roots get id = otelTraceIDToUUID(trace_id); non-roots get id = otelSpanIDToUUID(span_id).
		// distributed_trace_id = otelTraceIDToUUID(trace_id) for both, unless overridden by
		// the vendor `traceway.distributed_trace_id` attribute.
		type promotion struct {
			kind               entityKind
			id                 uuid.UUID
			isRoot             bool
			distributedTraceId *uuid.UUID
		}
		spanIdToPromotion := map[string]promotion{}

		for _, entry := range allSpans {
			span := entry.span
			kind := classifySpan(span, spanById)
			if kind == entityNone {
				continue
			}

			isRoot := len(span.ParentSpanId) == 0

			var id uuid.UUID
			if isRoot {
				id = otelTraceIDToUUID(span.TraceId)
				if id == uuid.Nil {
					id = otelSpanIDToUUID(span.SpanId)
				}
			} else {
				id = otelSpanIDToUUID(span.SpanId)
				if id == uuid.Nil {
					id = uuid.New()
				}
			}

			dtId := otelTraceIDToUUID(span.TraceId)
			var distributedTraceId *uuid.UUID
			if dtId != uuid.Nil {
				distributedTraceId = &dtId
			}
			if override := getStringAttribute(span.Attributes, "traceway.distributed_trace_id"); override != "" {
				if parsed, err := uuid.Parse(override); err == nil {
					distributedTraceId = &parsed
				}
			}

			spanIdToPromotion[string(span.SpanId)] = promotion{
				kind:               kind,
				id:                 id,
				isRoot:             isRoot,
				distributedTraceId: distributedTraceId,
			}
		}

		// resolveOwner walks parents until it finds a promoted entity. Returns the
		// promotion (so callers know the kind for trace_type) and whether one was
		// found. Cached per span id within this resource batch.
		ownerCache := map[string]*promotion{}
		var resolveOwner func(spanIdStr string) *promotion
		resolveOwner = func(spanIdStr string) *promotion {
			if cached, ok := ownerCache[spanIdStr]; ok {
				return cached
			}
			if p, ok := spanIdToPromotion[spanIdStr]; ok {
				pp := p
				ownerCache[spanIdStr] = &pp
				return &pp
			}
			parentId, hasParent := parentMap[spanIdStr]
			if !hasParent {
				ownerCache[spanIdStr] = nil
				return nil
			}
			owner := resolveOwner(parentId)
			ownerCache[spanIdStr] = owner
			return owner
		}

		// Pass 2: emit entity rows + span rows + exceptions.
		for _, entry := range allSpans {
			span := entry.span
			spanAttrs := span.Attributes
			allAttrs := extractAttributes(spanAttrs)
			startTime := nanoToTime(span.StartTimeUnixNano)
			endTime := nanoToTime(span.EndTimeUnixNano)
			duration := endTime.Sub(startTime)

			prom, promoted := spanIdToPromotion[string(span.SpanId)]

			// Determine the owning entity for span/exception trace_id.
			var owner *promotion
			if promoted {
				p := prom
				owner = &p
			} else {
				owner = resolveOwner(string(span.SpanId))
			}

			// trace_id for span rows / exceptions: owning entity id when known;
			// otherwise fall back to the OTel trace_id (orphan path — preserves
			// today's behavior for cross-process children whose parent never
			// matched a promoted entity within this batch).
			var ownerId uuid.UUID
			var ownerTraceType string
			if owner != nil {
				ownerId = owner.id
				ownerTraceType = owner.kind.traceType()
			} else {
				ownerId = otelTraceIDToUUID(span.TraceId)
				if ownerId == uuid.Nil {
					ownerId = uuid.New()
				}
			}

			if promoted {
				rootSpanId := otelSpanIDToUUID(span.SpanId)
				switch prom.kind {
				case entityEndpoint:
					ep := buildEndpoint(
						prom.id, projectId, span, spanAttrs, allAttrs,
						startTime, duration, serverName, appVersion,
					)
					ep.DistributedTraceId = prom.distributedTraceId
					ep.SpanId = &rootSpanId
					ep.IsRoot = prom.isRoot
					endpoints = append(endpoints, ep)
				case entityTask:
					t := buildTask(
						prom.id, projectId, span, allAttrs,
						startTime, endTime, duration, serverName, appVersion,
					)
					t.DistributedTraceId = prom.distributedTraceId
					t.SpanId = &rootSpanId
					t.IsRoot = prom.isRoot
					tasks = append(tasks, t)
				case entityAiTrace:
					aiTrace := buildAiTrace(
						prom.id, projectId, span, spanAttrs, allAttrs,
						startTime, duration, serverName, appVersion,
					)
					aiTrace.DistributedTraceId = prom.distributedTraceId
					aiTrace.IsRoot = prom.isRoot
					aiTraces = append(aiTraces, aiTrace)
					if conv := extractConversation(spanAttrs, projectId, prom.id); conv != nil {
						aiConversations = append(aiConversations, *conv)
					}
				}
			} else if len(span.ParentSpanId) > 0 {
				// Non-root, unpromoted span → goes to the generic spans table,
				// re-rooted to its nearest enclosing entity (or the OTel
				// trace_id as fallback when the parent chain doesn't reach a
				// promoted span in this batch).
				spanName := span.Name
				if dbQuery := getStringAttribute(spanAttrs, "db.query.text"); dbQuery != "" {
					spanName = dbQuery
				} else if dbStatement := getStringAttribute(spanAttrs, "db.statement"); dbStatement != "" {
					spanName = dbStatement
				}

				spans = append(spans, models.Span{
					Id:           otelSpanIDToUUID(span.SpanId),
					TraceId:      ownerId,
					ProjectId:    projectId,
					Name:         spanName,
					StartTime:    startTime,
					Duration:     duration,
					RecordedAt:   startTime,
					ParentSpanId: ptrSpanUUID(span.ParentSpanId),
					Attributes:   allAttrs,
				})
			} else {
				// Unpromoted root span — match historical behavior and drop
				// it (no entity row, no span row, no exception). Common case:
				// CLIENT-kind roots or non-HTTP SERVER roots from custom
				// instrumentation that we don't have a dedicated page for.
				continue
			}

			traceType := ownerTraceType
			if traceType == "" {
				traceType = "task"
			}

			for _, event := range span.Events {
				if event.Name == "exception" {
					exc := buildException(
						ctx, projectId, ownerId, traceType, event,
						allAttrs, serverName, appVersion, language, entry.scopeName,
					)
					if owner != nil {
						exc.DistributedTraceId = owner.distributedTraceId
					}
					exceptions = append(exceptions, exc)
				}
			}
		}
	}
	return
}

func classifySpan(span *tracepb.Span, spanById map[string]*tracepb.Span) entityKind {
	attrs := span.Attributes
	if (span.Kind == tracepb.Span_SPAN_KIND_SERVER || span.Kind == tracepb.Span_SPAN_KIND_INTERNAL) && hasHTTPAttributes(attrs) {
		if len(span.ParentSpanId) == 0 {
			return entityEndpoint
		}
		if _, parentInBatch := spanById[string(span.ParentSpanId)]; !parentInBatch {
			return entityEndpoint
		}
	}
	if span.Kind == tracepb.Span_SPAN_KIND_CONSUMER {
		return entityTask
	}
	// keepsuit's ConsoleInstrumentation (and equivalents) emits a root INTERNAL
	// span with `console.command` set. Only promote when it is a root span —
	// otherwise we would scoop up arbitrary manual roots from other code paths.
	if span.Kind == tracepb.Span_SPAN_KIND_INTERNAL && len(span.ParentSpanId) == 0 && getStringAttribute(attrs, "console.command") != "" {
		return entityTask
	}
	if hasGenAiAttributes(attrs) {
		return entityAiTrace
	}
	return entityNone
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
	ctx context.Context,
	projectId, traceId uuid.UUID,
	traceType string,
	event *tracepb.Span_Event,
	spanAttrs map[string]string,
	serverName, appVersion, language, scopeName string,
) models.ExceptionStackTrace {
	eventAttrs := event.Attributes
	excType := getStringAttribute(eventAttrs, "exception.type")
	excMessage := getStringAttribute(eventAttrs, "exception.message")

	stackTrace, ok := buildHoneycombStackTrace(excType, excMessage, eventAttrs)
	if !ok {
		excStacktrace := getStringAttribute(eventAttrs, "exception.stacktrace")
		stackTrace = formatExceptionStackTrace(excType, excMessage, excStacktrace)
	}

	stackTrace = otelSymbolicateJs(projectId, ctx, stackTrace, language, scopeName)

	hash := clientcontrollers.ComputeExceptionHash(stackTrace, false)

	attrs := spanAttrs
	if isJsLanguage(language) {
		attrs = cloneStringMap(spanAttrs)
		attrs["telemetry.sdk.language"] = language
	}

	return models.ExceptionStackTrace{
		Id:            uuid.New(),
		ProjectId:     projectId,
		TraceId:       &traceId,
		TraceType:     traceType,
		ExceptionHash: hash,
		StackTrace:    stackTrace,
		RecordedAt:    nanoToTime(event.TimeUnixNano),
		Attributes:    attrs,
		AppVersion:    appVersion,
		ServerName:    serverName,
	}
}

func cloneStringMap(m map[string]string) map[string]string {
	out := make(map[string]string, len(m)+1)
	for k, v := range m {
		out[k] = v
	}
	return out
}

func isJsLanguage(lang string) bool {
	switch strings.ToLower(lang) {
	case "webjs", "nodejs", "javascript", "typescript":
		return true
	}
	return false
}

// JS instrumentations are also recognizable by their scope name when
// telemetry.sdk.language is absent: npm scoped packages ("@scope/pkg") are
// an npm-only naming convention across OTel SDKs, and Next.js's built-in
// tracer reports the unscoped scope name "next.js".
func isJsTelemetry(language, scopeName string) bool {
	if isJsLanguage(language) {
		return true
	}
	if strings.HasPrefix(scopeName, "@") && strings.Contains(scopeName, "/") {
		return true
	}
	return scopeName == "next.js"
}

func buildHoneycombStackTrace(excType, excMessage string, eventAttrs []*commonpb.KeyValue) (string, bool) {
	urls := getStringArray(eventAttrs, "exception.structured_stacktrace.urls")
	if len(urls) == 0 {
		return "", false
	}
	functions := getStringArray(eventAttrs, "exception.structured_stacktrace.functions")
	lines := getIntArray(eventAttrs, "exception.structured_stacktrace.lines")
	columns := getIntArray(eventAttrs, "exception.structured_stacktrace.columns")

	at := func(s []string, i int) string {
		if i < len(s) {
			return s[i]
		}
		return ""
	}
	atInt := func(s []int64, i int) int64 {
		if i < len(s) {
			return s[i]
		}
		return 0
	}

	var b strings.Builder
	if header := formatExceptionStackTrace(excType, excMessage, ""); header != "unknown exception" {
		b.WriteString(header)
		b.WriteByte('\n')
	}
	for i, url := range urls {
		if fn := at(functions, i); fn != "" {
			b.WriteString(fn)
			b.WriteString("()\n")
		}
		fmt.Fprintf(&b, "    %s:%d:%d\n", url, atInt(lines, i), atInt(columns, i))
	}
	return strings.TrimRight(b.String(), "\n"), true
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
