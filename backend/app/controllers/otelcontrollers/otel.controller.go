package otelcontrollers

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/hooks"
	"github.com/tracewayapp/traceway/backend/app/middleware"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/monitoring"
	"github.com/tracewayapp/traceway/backend/app/profiling"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry/shared"
	"github.com/tracewayapp/traceway/backend/app/services"
	"github.com/tracewayapp/traceway/backend/app/storage"
	"github.com/tracewayapp/traceway/backend/app/symbolicator/sourcemap/jsstack"
	traceway "go.tracewayapp.com"
)

func msSince(t time.Time) float64 {
	return float64(time.Since(t).Microseconds()) / 1000.0
}

func otelSymbolicateJs(existingProject *models.Project, projectId uuid.UUID, ctx context.Context, stackTrace, language, scopeName string) string {
	if !isJsTelemetry(language, scopeName) {
		return stackTrace
	}
	canonical, _ := jsstack.Canonicalize(stackTrace)
	if existingProject == nil || existingProject.SourceMapToken == nil {
		return canonical
	}
	return services.ResolveStackTrace(ctx, projectId, canonical, nil)
}

func otelSymbolicateAndroid(existingProject *models.Project, projectId uuid.UUID, ctx context.Context, stackTrace, language, proguardUuid string) string {
	if !isAndroidLanguage(language) {
		return stackTrace
	}
	if existingProject == nil || existingProject.SourceMapToken == nil {
		return stackTrace
	}
	return services.ResolveAndroidStackTrace(ctx, projectId, stackTrace, proguardUuid)
}

type otelController struct{}

var traceStore = struct {
	spans      func(context.Context, []models.OtelSpan) ([]shared.SpanOwner, error)
	endpoints  func(context.Context, []models.Endpoint) error
	tasks      func(context.Context, []models.Task) error
	exceptions func(context.Context, []models.ExceptionStackTrace) error
	aiTraces   func(context.Context, []models.AiTrace) error
}{
	spans:      telemetry.OtelSpanRepository.InsertWithRejections,
	endpoints:  telemetry.EndpointRepository.InsertAsync,
	tasks:      telemetry.TaskRepository.InsertAsync,
	exceptions: telemetry.ExceptionStackTraceRepository.InsertAsync,
	aiTraces:   telemetry.AiTraceRepository.InsertAsync,
}

func abortIngestStorage(c *gin.Context, stage string, err error) {
	status := http.StatusInternalServerError
	if telemetry.IsTransientStorageError(err) {
		status = http.StatusServiceUnavailable
		c.Header("Retry-After", "2")
	}
	c.AbortWithError(status, traceway.NewStackTraceErrorf("error inserting OTEL %s: %w", stage, err))
}

var OtelController = otelController{}

func (o otelController) ExportTraces(c *gin.Context) {
	monitoring.IngestStarted()
	defer monitoring.IngestFinished()

	projectId, err := middleware.GetProjectId(c)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("UseClientAuth middleware must be applied: %w", err))
		return
	}
	var project *models.Project
	if projectAsAny, exists := c.Get(middleware.ProjectContextKey); exists {
		if p, ok := projectAsAny.(*models.Project); ok {
			project = p
		}
	}

	orgId := 0
	hasOrg := project != nil && project.OrganizationId != nil
	if hasOrg {
		orgId = *project.OrganizationId
		if attrs := traceway.GetAttributesFromContext(c); attrs != nil {
			attrs.SetTag("organization_id", fmt.Sprintf("%d", orgId))
		}
	}
	perm := hooks.IngestPermission{Exceptions: true, Data: true, Replay: true}
	if hasOrg {
		perm = hooks.IngestPermissionFor(orgId)
	}
	req, bodyBytes, err := decodeTraceRequest(c)
	if err != nil {
		writeDecodeError(c, err)
		return
	}

	convertStart := time.Now()
	result := convertTraces(c, project, projectId, req)
	endpoints, tasks, exceptions := result.Endpoints, result.Tasks, result.Exceptions
	aiTraces, aiConversations := result.AiTraces, result.AiConversations
	canonicalSpans := result.Spans
	if !perm.Data {
		canonicalSpans = nil
	}
	var droppedHealthchecks map[string]bool
	endpoints, droppedHealthchecks = services.FilterHealthchecks(project, endpoints, exceptions)
	if len(droppedHealthchecks) > 0 {
		retained := make(map[string]bool)
		for _, endpoint := range endpoints {
			retained[services.SpanKey(endpoint.TraceId, endpoint.SpanId)] = true
		}
		for _, task := range tasks {
			retained[services.SpanKey(task.TraceId, task.SpanId)] = true
		}
		for _, ai := range aiTraces {
			retained[services.SpanKey(ai.TraceId, ai.SpanId)] = true
		}
		canonicalSpans = services.DropSpanSubtrees(canonicalSpans, func(span models.OtelSpan) (string, string, string) {
			return span.TraceId, span.SpanId, span.ParentSpanId
		}, droppedHealthchecks, retained)
		monitoring.RecordHealthchecksDropped(monitoring.SignalTraces, len(droppedHealthchecks))
	}

	if !perm.Data {
		endpoints = nil
		tasks = nil
		aiTraces = nil
		aiConversations = nil
	}
	if !perm.Exceptions {
		exceptions = nil
	}
	if hasOrg && (!perm.Data || !perm.Exceptions) {
		monitoring.RecordRateLimited(orgId)
	}

	convertMs := msSince(convertStart)

	insertStart := time.Now()

	// Spans are the source the other rows are projected from, so a failed span write must not leave projections behind.
	notStored, err := traceStore.spans(c, canonicalSpans)
	if err != nil {
		abortIngestStorage(c, "spans", err)
		return
	}

	if len(notStored) > 0 {
		rejected := make(map[shared.SpanOwner]bool, len(notStored))
		for _, span := range notStored {
			rejected[span] = true
		}
		hasRejectedSource := func(traceId, spanId string) bool {
			return rejected[shared.SpanOwner{ProjectId: projectId, TraceId: traceId, SpanId: spanId}]
		}
		endpoints = slices.DeleteFunc(endpoints, func(row models.Endpoint) bool { return hasRejectedSource(row.TraceId, row.SpanId) })
		tasks = slices.DeleteFunc(tasks, func(row models.Task) bool { return hasRejectedSource(row.TraceId, row.SpanId) })
		exceptions = slices.DeleteFunc(exceptions, func(row models.ExceptionStackTrace) bool { return hasRejectedSource(row.TraceId, row.SpanId) })
		aiTraces = slices.DeleteFunc(aiTraces, func(row models.AiTrace) bool { return hasRejectedSource(row.TraceId, row.SpanId) })
		storedConversations := map[string]bool{}
		for _, row := range aiTraces {
			storedConversations[row.StorageKey] = true
		}
		aiConversations = slices.DeleteFunc(aiConversations, func(row aiTraceConversation) bool { return !storedConversations[row.StorageKey] })
	}

	if err := traceStore.endpoints(c, endpoints); err != nil {
		abortIngestStorage(c, "endpoints", err)
		return
	}

	if err := traceStore.tasks(c, tasks); err != nil {
		abortIngestStorage(c, "tasks", err)
		return
	}

	if err := traceStore.exceptions(c, exceptions); err != nil {
		abortIngestStorage(c, "exceptions", err)
		return
	}

	if len(aiTraces) > 0 {
		if err := traceStore.aiTraces(c, aiTraces); err != nil {
			abortIngestStorage(c, "ai traces", err)
			return
		}

		if len(aiConversations) > 0 {
			convs := aiConversations
			go func() {
				defer traceway.Recover()

				for _, conv := range convs {
					if err := storage.Store.Write(context.Background(), conv.StorageKey, conv.Content); err != nil {
						traceway.CaptureException(fmt.Errorf("failed to write AI trace conversation (key=%s): %w", conv.StorageKey, err))
					}
				}
			}()
		}
	}

	insertMs := msSince(insertStart)
	totalSize := len(endpoints) + len(tasks) + len(canonicalSpans) + len(exceptions) + len(aiTraces)
	monitoring.RecordIngestBatch(monitoring.SignalTraces, "traces", convertMs, insertMs, totalSize, bodyBytes)

	var exceptionHashes []string
	for _, ex := range exceptions {
		exceptionHashes = append(exceptionHashes, ex.ExceptionHash)
	}

	var aiTraceInfos []hooks.AiTraceInfo
	for _, at := range aiTraces {
		aiTraceInfos = append(aiTraceInfos, hooks.AiTraceInfo{
			TraceName:      at.TraceName,
			TotalCost:      at.TotalCost,
			ConversationId: at.ConversationId,
			UserId:         at.UserId,
			Flagged:        at.Flagged,
			FlaggedTerms:   at.FlaggedTerms,
		})
	}

	if hasOrg {
		ev := hooks.ReportEvent{
			OrganizationId: orgId,
			ProjectId:      projectId,
			AiTraces:       aiTraceInfos,
		}
		if perm.Data {
			ev.EndpointCount = len(endpoints)
			ev.TaskCount = len(tasks)
			ev.TelemetryBytes = bodyBytes
		}
		if perm.Exceptions {
			ev.ErrorCount = len(exceptions)
			ev.ExceptionHashes = exceptionHashes
		}
		hooks.BroadcastReport(ev)
	}

	writeTraceResponse(c, result.InvalidSpanIDs, int64(len(notStored)))
}

func (o otelController) ExportMetrics(c *gin.Context) {
	monitoring.IngestStarted()
	defer monitoring.IngestFinished()

	projectId, err := middleware.GetProjectId(c)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("UseClientAuth middleware must be applied: %w", err))
		return
	}

	orgId := 0
	hasOrg := false
	if project, exists := c.Get(middleware.ProjectContextKey); exists {
		if p, ok := project.(*models.Project); ok && p.OrganizationId != nil {
			orgId = *p.OrganizationId
			hasOrg = true
			if attrs := traceway.GetAttributesFromContext(c); attrs != nil {
				attrs.SetTag("organization_id", fmt.Sprintf("%d", orgId))
			}
		}
	}
	perm := hooks.IngestPermission{Exceptions: true, Data: true, Replay: true}
	if hasOrg {
		perm = hooks.IngestPermissionFor(orgId)
	}

	req, bodyBytes, err := decodeMetricsRequest(c)
	if err != nil {
		writeDecodeError(c, err)
		return
	}

	convertStart := time.Now()
	result := convertMetricPoints(projectId, req)
	convertMs := msSince(convertStart)

	if !perm.Data {
		result.Points = nil
		result.Entries = nil
		if hasOrg {
			monitoring.RecordRateLimited(orgId)
		}
	}

	insertMs := 0.0
	if len(result.Points) > 0 {
		insertStart := time.Now()
		if err := telemetry.MetricPointRepository.InsertAsync(c, result.Points); err != nil {
			abortIngestStorage(c, "metric points", err)
			return
		}
		insertMs = msSince(insertStart)

		if len(result.Entries) > 0 {
			go services.AutoRegisterMetricsWithUnits(projectId, result.Entries)
		}
	}

	monitoring.RecordIngestBatch(monitoring.SignalMetrics, "metric_points", convertMs, insertMs, len(result.Points), bodyBytes)

	if hasOrg && perm.Data {
		hooks.BroadcastReport(hooks.ReportEvent{
			OrganizationId: orgId,
			ProjectId:      projectId,
			TelemetryBytes: bodyBytes,
		})
	}

	writeMetricsResponse(c)
}

func (o otelController) ExportLogs(c *gin.Context) {
	monitoring.IngestStarted()
	defer monitoring.IngestFinished()

	projectId, err := middleware.GetProjectId(c)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("UseClientAuth middleware must be applied: %w", err))
		return
	}
	var existingProject *models.Project
	orgId := 0
	hasOrg := false
	if project, exists := c.Get(middleware.ProjectContextKey); exists {
		if p, ok := project.(*models.Project); ok && p.OrganizationId != nil {
			existingProject = p
			orgId = *p.OrganizationId
			hasOrg = true
			if attrs := traceway.GetAttributesFromContext(c); attrs != nil {
				attrs.SetTag("organization_id", fmt.Sprintf("%d", orgId))
			}
		}
	}
	perm := hooks.IngestPermission{Exceptions: true, Data: true, Replay: true}
	if hasOrg {
		perm = hooks.IngestPermissionFor(orgId)
	}

	req, bodyBytes, err := decodeLogsRequest(c)
	if err != nil {
		writeDecodeError(c, err)
		return
	}

	convertStart := time.Now()
	records := convertLogs(existingProject, c, projectId, req)
	convertMs := msSince(convertStart)

	if !perm.Data {
		records = nil
		if hasOrg {
			monitoring.RecordRateLimited(orgId)
		}
	}

	insertMs := 0.0
	if len(records) > 0 {
		insertStart := time.Now()
		if err := telemetry.LogRecordRepository.InsertAsync(c, records); err != nil {
			abortIngestStorage(c, "log records", err)
			return
		}
		insertMs = msSince(insertStart)
	}

	monitoring.RecordIngestBatch(monitoring.SignalLogs, "log_records", convertMs, insertMs, len(records), bodyBytes)

	if hasOrg && perm.Data {
		hooks.BroadcastReport(hooks.ReportEvent{
			OrganizationId: orgId,
			ProjectId:      projectId,
			TelemetryBytes: bodyBytes,
		})
	}

	writeLogsResponse(c)
}

func (o otelController) ExportProfiles(c *gin.Context) {
	monitoring.IngestStarted()
	defer monitoring.IngestFinished()

	projectId, err := middleware.GetProjectId(c)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("UseClientAuth middleware must be applied: %w", err))
		return
	}

	var project *models.Project
	if projectAsAny, exists := c.Get(middleware.ProjectContextKey); exists {
		if p, ok := projectAsAny.(*models.Project); ok {
			project = p
		}
	}
	if project != nil && project.OrganizationId != nil {
		if attrs := traceway.GetAttributesFromContext(c); attrs != nil {
			attrs.SetTag("organization_id", fmt.Sprintf("%d", *project.OrganizationId))
		}
	}

	payload, bodyBytes, err := decodeProfilesPayload(c)
	if err != nil {
		writeDecodeError(c, err)
		return
	}

	var labelAllowlist []string
	if project != nil {
		labelAllowlist = project.ProfileLabelAllowlist
	}

	convertStart := time.Now()
	decoded, err := profiling.OTLPDecoder{}.Decode(profiling.IngestContext{
		ProjectId:      projectId,
		ReceivedAt:     time.Now().UTC(),
		LabelAllowlist: profiling.NewLabelAllowSet(labelAllowlist),
	}, payload)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	stacks, samples, profiles := profiling.BuildRows(projectId, decoded)
	convertMs := msSince(convertStart)

	insertStart := time.Now()
	if err := telemetry.ProfileRepository.InsertStacksAsync(c, stacks); err != nil {
		c.AbortWithError(500, traceway.NewStackTraceErrorf("error inserting OTEL profiling stacks: %w", err))
		return
	}
	if err := telemetry.ProfileRepository.InsertSamplesAsync(c, samples); err != nil {
		c.AbortWithError(500, traceway.NewStackTraceErrorf("error inserting OTEL profiling samples: %w", err))
		return
	}
	if err := telemetry.ProfileRepository.InsertProfilesAsync(c, profiles); err != nil {
		c.AbortWithError(500, traceway.NewStackTraceErrorf("error inserting OTEL profiles: %w", err))
		return
	}
	insertMs := msSince(insertStart)

	monitoring.RecordIngestBatch(monitoring.SignalProfiles, "profiling_samples", convertMs, insertMs, len(samples), bodyBytes)

	writeProfilesResponse(c)
}
