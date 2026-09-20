package otelcontrollers

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/controllers/clientcontrollers"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry/shared"
	"github.com/tracewayapp/traceway/backend/app/services/contentflag"
	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
)

type aiTraceConversation struct {
	StorageKey string
	Content    []byte
	Input      string
	Output     string
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

func convertTraces(ctx context.Context, existingProject *models.Project, projectId uuid.UUID, req *coltracepb.ExportTraceServiceRequest) (
	endpoints []models.Endpoint,
	tasks []models.Task,
	exceptions []models.ExceptionStackTrace,
	aiTraces []models.AiTrace,
	aiConversations []aiTraceConversation,
) {
	suppressEntities := existingProject != nil && clientcontrollers.IsFrontendFramework(existingProject.Framework)

	// nil languages falls back to the default pack; a project that disabled
	// every pack carries an empty (non-nil) slice and scans custom terms only.
	var flagLanguages, customFlagTerms []string
	if existingProject != nil {
		flagLanguages = existingProject.AiFlaggedLanguages
		customFlagTerms = existingProject.AiFlaggedTerms
	}
	flagMatcher := contentflag.NewMatcher(flagLanguages, customFlagTerms)
	ingestedAt := time.Now().UTC()

	for _, rs := range req.ResourceSpans {
		resourceAttrs := rs.GetResource().GetAttributes()
		serverName := getStringAttribute(resourceAttrs, "service.name")
		appVersion := getStringAttribute(resourceAttrs, "service.version")
		language := getStringAttribute(resourceAttrs, "telemetry.sdk.language")
		proguardUuid := getStringAttribute(resourceAttrs, "app.debug.proguard_uuid")
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
		spanByKey := map[string]*tracepb.Span{}
		for _, ss := range rs.ScopeSpans {
			for _, span := range ss.Spans {
				if !validSourceSpanIDs(span) {
					continue
				}
				allSpans = append(allSpans, spanEntry{span: span, scopeName: ss.GetScope().GetName()})
				spanByKey[otelSpanKey(span)] = span
			}
		}
		parentInBatch := func(span *tracepb.Span) *tracepb.Span {
			if len(span.ParentSpanId) == 0 {
				return nil
			}
			return spanByKey[string(span.TraceId)+string(span.ParentSpanId)]
		}

		for _, entry := range allSpans {
			span := entry.span
			kind := entityNone
			if !suppressEntities {
				kind = classifySpan(span, parentInBatch)
			}
			if kind == entityNone && !spanCarriesException(span) {
				continue
			}

			spanAttrs := span.Attributes
			allAttrs := extractAttributes(spanAttrs)
			// A span carrying exception.stacktrace always yields an exception
			// record (from the attribute path, or from the event when both are
			// present), so the raw blob is never duplicated onto the
			// endpoint/task/span rows or the exception's attribute map.
			delete(allAttrs, "exception.stacktrace")
			recordedStart, _, _ := shared.OtelStorageTimes(span.StartTimeUnixNano, ingestedAt)
			duration := shared.OtelDuration(span.StartTimeUnixNano, span.EndTimeUnixNano)

			traceId := hex.EncodeToString(span.TraceId)
			spanId := hex.EncodeToString(span.SpanId)
			parentSpanId := hex.EncodeToString(span.ParentSpanId)
			linked := linkedTraceId(span, traceId)
			id := otelOccurrenceID(projectId, span)

			switch kind {
			case entityEndpoint:
				ep := buildEndpoint(
					id, projectId, span, spanAttrs, allAttrs,
					recordedStart, duration, serverName, appVersion,
				)
				ep.TraceId, ep.SpanId, ep.ParentSpanId, ep.LinkedTraceId = traceId, spanId, parentSpanId, linked
				ep.IsRoot = parentSpanId == ""
				endpoints = append(endpoints, ep)
			case entityTask:
				t := buildTask(
					id, projectId, span, allAttrs,
					recordedStart, duration, serverName, appVersion,
				)
				t.TraceId, t.SpanId, t.ParentSpanId, t.LinkedTraceId = traceId, spanId, parentSpanId, linked
				t.IsRoot = parentSpanId == ""
				tasks = append(tasks, t)
			case entityAiTrace:
				aiTrace := buildAiTrace(
					id, projectId, span, spanAttrs, allAttrs,
					recordedStart, duration, serverName, appVersion,
				)
				aiTrace.TraceId, aiTrace.SpanId, aiTrace.ParentSpanId, aiTrace.LinkedTraceId = traceId, spanId, parentSpanId, linked
				aiTrace.IsRoot = parentSpanId == ""
				aiTrace.ConversationId = resolveConversationId(spanAttrs, resourceAttrs, traceId)
				var convInput, convOutput string
				if conv := extractConversation(spanAttrs, projectId, id); conv != nil {
					convInput, convOutput = conv.Input, conv.Output
					aiConversations = append(aiConversations, *conv)
				}
				aiTrace.ToolCallCount, aiTrace.ToolNames = extractToolCalls(spanAttrs, convOutput)
				if terms := flagMatcher.Scan(convInput, convOutput); len(terms) > 0 {
					aiTrace.Flagged = true
					aiTrace.FlaggedTerms = terms
				}
				aiTraces = append(aiTraces, aiTrace)
			}

			appendException := func(attrs []*commonpb.KeyValue, timeUnixNano uint64) {
				exc := buildException(
					ctx, existingProject, projectId, attrs, timeUnixNano,
					allAttrs, serverName, appVersion, language, proguardUuid, entry.scopeName,
				)
				// The kind is stored only when this span says it. An exception further down the request finds its entity at read time.
				exc.TraceId, exc.SpanId, exc.LinkedTraceId, exc.TraceType = traceId, spanId, linked, kind.traceType()
				exceptions = append(exceptions, exc)
			}

			hadExceptionEvent := false
			for _, event := range span.Events {
				if event.Name == "exception" {
					hadExceptionEvent = true
					appendException(event.Attributes, event.TimeUnixNano)
				}
			}
			if !hadExceptionEvent && hasExceptionAttributes(span.Attributes) {
				appendException(span.Attributes, span.StartTimeUnixNano)
			}
		}
	}
	return
}

const (
	spanFlagHasIsRemote = uint32(tracepb.SpanFlags_SPAN_FLAGS_CONTEXT_HAS_IS_REMOTE_MASK)
	spanFlagIsRemote    = uint32(tracepb.SpanFlags_SPAN_FLAGS_CONTEXT_IS_REMOTE_MASK)
	// Bounds the walk when malformed input makes the parent chain a cycle.
	maxEntryPointDepth = 64
)

func isHTTPServerSpan(span *tracepb.Span) bool {
	return span.Kind == tracepb.Span_SPAN_KIND_SERVER && hasHTTPAttributes(span.Attributes)
}

// An HTTP SERVER span is a request of its own unless a local span above it already is one (Next.js under the HTTP
// instrumentation). The walk stays inside the process: it stops at a remote parent, which OTLP flags mark however
// the trace is batched, and at a CLIENT or PRODUCER span, which is the calling side. A plain wrapper span above
// the request does not make it nested. Missing parents have unknown kinds, even
// when flags establish locality, so they cannot suppress an HTTP entry point.
func isEntryPoint(span *tracepb.Span, parentOf func(*tracepb.Span) *tracepb.Span) bool {
	for current, depth := span, 0; depth < maxEntryPointDepth; depth++ {
		if len(current.ParentSpanId) == 0 {
			return true
		}
		localityKnown := current.Flags&spanFlagHasIsRemote != 0
		if localityKnown && current.Flags&spanFlagIsRemote != 0 {
			return true
		}
		parent := parentOf(current)
		if parent == nil {
			return true
		}
		if parent.Kind == tracepb.Span_SPAN_KIND_CLIENT || parent.Kind == tracepb.Span_SPAN_KIND_PRODUCER {
			return true
		}
		if isHTTPServerSpan(parent) {
			return false
		}
		current = parent
	}
	return true
}

// linkedTraceId is the other trace a span says it belongs with: the browser or mobile trace whose id the first backend
// service copied from the traceway-trace-id header.
func linkedTraceId(span *tracepb.Span, traceId string) string {
	linked := shared.NormalizeTraceId(getStringAttribute(span.Attributes, "traceway.distributed_trace_id"))
	if len(linked) != 32 || linked == traceId {
		return ""
	}
	if _, err := hex.DecodeString(linked); err != nil {
		return ""
	}
	return linked
}

func classifySpan(span *tracepb.Span, parentOf func(*tracepb.Span) *tracepb.Span) entityKind {
	attrs := span.Attributes
	// Honeycomb's browser SDK stamps page context (url.path etc.) on every
	// span, including the zero-duration INTERNAL `exception` spans emitted by
	// its global-errors instrumentation. Those must become exception rows,
	// not endpoints, so exception-bearing INTERNAL spans are never promoted.
	if span.Kind == tracepb.Span_SPAN_KIND_INTERNAL && hasExceptionAttributes(attrs) {
		return entityNone
	}
	if (isHTTPServerSpan(span) && isEntryPoint(span, parentOf)) ||
		(span.Kind == tracepb.Span_SPAN_KIND_INTERNAL && len(span.ParentSpanId) == 0 && hasHTTPAttributes(attrs)) {
		return entityEndpoint
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

	// A 404 collapses to UNMATCHED only when no real route matched: http.route
	// is missing/invalid, or a catch-all made of only slashes and wildcards
	// ("/", "/*", "/**", "*/*") — what Express middleware, Spring resource
	// handlers and not-found handlers report for unmatched requests. A concrete
	// matched route returning 404 is a deliberate response and keeps its identity.
	route := getStringAttribute(attrs, "http.route")
	if statusCode == 404 && (!strings.HasPrefix(route, "/") || strings.Trim(route, "/*") == "") {
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
	ctx context.Context,
	existingProject *models.Project,
	projectId uuid.UUID,
	excAttrs []*commonpb.KeyValue,
	timeUnixNano uint64,
	spanAttrs map[string]string,
	serverName, appVersion, language, proguardUuid, scopeName string,
) models.ExceptionStackTrace {
	excType := getStringAttribute(excAttrs, "exception.type")
	excMessage := getStringAttribute(excAttrs, "exception.message")

	stackTrace, ok := buildHoneycombStackTrace(excType, excMessage, excAttrs)
	if !ok {
		excStacktrace := getStringAttribute(excAttrs, "exception.stacktrace")
		if excStacktrace == "" {
			if frames, aok := buildAndroidStructuredStackTrace(excAttrs); aok {
				excStacktrace = frames
			}
		}
		stackTrace = formatExceptionStackTrace(excType, excMessage, excStacktrace)
	}

	stackTrace = otelSymbolicateJs(existingProject, projectId, ctx, stackTrace, language, scopeName)
	stackTrace = otelSymbolicateAndroid(existingProject, projectId, ctx, stackTrace, language, proguardUuid)

	hash := clientcontrollers.ComputeExceptionHash(stackTrace, false)
	recordedAt, _, _ := shared.OtelStorageTimes(timeUnixNano, time.Now())

	attrs := spanAttrs
	if isJsLanguage(language) || isAndroidLanguage(language) {
		attrs = cloneStringMap(spanAttrs)
		attrs["telemetry.sdk.language"] = language
	}

	return models.ExceptionStackTrace{
		Id:            uuid.New(),
		ProjectId:     projectId,
		ExceptionHash: hash,
		StackTrace:    stackTrace,
		RecordedAt:    recordedAt,
		Attributes:    attrs,
		AppVersion:    appVersion,
		ServerName:    serverName,
	}
}

func hasExceptionAttributes(attrs []*commonpb.KeyValue) bool {
	return getStringAttribute(attrs, "exception.type") != "" ||
		getStringAttribute(attrs, "exception.message") != "" ||
		getStringAttribute(attrs, "exception.stacktrace") != ""
}

func spanCarriesException(span *tracepb.Span) bool {
	for _, event := range span.Events {
		if event.Name == "exception" {
			return true
		}
	}
	return hasExceptionAttributes(span.Attributes)
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

func isAndroidLanguage(lang string) bool {
	switch strings.ToLower(strings.TrimSpace(lang)) {
	case "android", "java", "kotlin":
		return true
	}
	return false
}

func buildAndroidStructuredStackTrace(eventAttrs []*commonpb.KeyValue) (string, bool) {
	classes := getStringArray(eventAttrs, "exception.structured_stacktrace.classes")
	if len(classes) == 0 {
		return "", false
	}
	methods := getStringArray(eventAttrs, "exception.structured_stacktrace.methods")
	files := getStringArray(eventAttrs, "exception.structured_stacktrace.source_files")
	lines := getIntArray(eventAttrs, "exception.structured_stacktrace.lines")

	var b strings.Builder
	for i := range classes {
		method := ""
		if i < len(methods) {
			method = methods[i]
		}
		file := ""
		if i < len(files) {
			file = files[i]
		}
		loc := file
		if i < len(lines) && lines[i] > 0 {
			loc = fmt.Sprintf("%s:%d", file, lines[i])
		}
		fmt.Fprintf(&b, "\tat %s.%s(%s)\n", classes[i], method, loc)
	}
	return strings.TrimRight(b.String(), "\n"), true
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
		// finish_reasons is an array attribute in the OTel gen_ai conventions;
		// getStringValues handles both the scalar and array encodings.
		finishReason = strings.Join(getStringValues(attrs, "gen_ai.response.finish_reasons"), ",")
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
	"gen_ai.conversation.id",
	"gen_ai.tool.",
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
		Input:      input,
		Output:     output,
	}
}

// resolveConversationId picks the conversation grouping key for an AI trace:
// an explicit gen_ai.conversation.id, else session.id (span first, then
// resource, where browser SDKs stamp it), else the trace id so a single agent
// run still groups its calls.
func resolveConversationId(spanAttrs, resourceAttrs []*commonpb.KeyValue, traceId string) string {
	if id := getStringAttribute(spanAttrs, "gen_ai.conversation.id"); id != "" {
		return id
	}
	if id := getStringAttribute(spanAttrs, "session.id"); id != "" {
		return id
	}
	if id := getStringAttribute(resourceAttrs, "session.id"); id != "" {
		return id
	}
	// Keep fallback conversation keys compatible with history written before V2.
	if id, err := uuid.Parse(traceId); err == nil {
		return id.String()
	}
	return traceId
}

const maxToolNames = 50

// extractToolCalls pulls tool-call telemetry out of the completion payload
// (OpenAI choices/tool_calls, Anthropic content/tool_use, OTel gen_ai output
// messages with tool_call parts), falling back to the execute_tool span
// attributes when no payload is available. Names are deduplicated in
// first-seen order; commas are stripped because the names are persisted as a
// comma-separated column.
func extractToolCalls(attrs []*commonpb.KeyValue, output string) (int64, []string) {
	count, names := parseToolCallsFromOutput(output)
	if count == 0 && getStringAttribute(attrs, "gen_ai.operation.name") == "execute_tool" {
		count = 1
		if name := sanitizeToolName(getStringAttribute(attrs, "gen_ai.tool.name")); name != "" {
			names = []string{name}
		}
	}
	return count, names
}

func parseToolCallsFromOutput(output string) (int64, []string) {
	if output == "" {
		return 0, nil
	}

	var count int64
	var names []string
	seen := map[string]struct{}{}
	record := func(name string) {
		count++
		name = sanitizeToolName(name)
		if name == "" || len(names) >= maxToolNames {
			return
		}
		if _, dup := seen[name]; dup {
			return
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}

	type contentPart struct {
		Type string `json:"type"`
		Name string `json:"name"`
	}
	var objectShape struct {
		// OpenAI-style chat completion response.
		Choices []struct {
			Message struct {
				ToolCalls []struct {
					Function struct {
						Name string `json:"name"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
		// Anthropic-style response content blocks.
		Content []contentPart `json:"content"`
	}
	if err := json.Unmarshal([]byte(output), &objectShape); err == nil {
		for _, choice := range objectShape.Choices {
			for _, call := range choice.Message.ToolCalls {
				record(call.Function.Name)
			}
		}
		for _, part := range objectShape.Content {
			if part.Type == "tool_use" {
				record(part.Name)
			}
		}
		return count, names
	}

	// OTel gen_ai output messages: a top-level array of messages with parts.
	var messagesShape []struct {
		Parts []contentPart `json:"parts"`
	}
	if err := json.Unmarshal([]byte(output), &messagesShape); err == nil {
		for _, msg := range messagesShape {
			for _, part := range msg.Parts {
				if part.Type == "tool_call" || part.Type == "tool_use" {
					record(part.Name)
				}
			}
		}
	}
	return count, names
}

func sanitizeToolName(name string) string {
	return strings.TrimSpace(strings.ReplaceAll(name, ",", ""))
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
