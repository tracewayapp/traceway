package otelcontrollers

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/hooks"
	"github.com/tracewayapp/traceway/backend/app/middleware"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/monitoring"
	"github.com/tracewayapp/traceway/backend/app/repositories"
	"github.com/tracewayapp/traceway/backend/app/services"
	"github.com/tracewayapp/traceway/backend/app/storage"
	"github.com/tracewayapp/traceway/backend/app/symbolicator/jsstack"
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

type otelController struct{}

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

	if project != nil && project.OrganizationId != nil {
		if attrs := traceway.GetAttributesFromContext(c); attrs != nil {
			attrs.SetTag("organization_id", fmt.Sprintf("%d", *project.OrganizationId))
		}
		if !hooks.CanReport(*project.OrganizationId) {
			monitoring.RecordRateLimited(*project.OrganizationId)
			c.AbortWithStatus(http.StatusTooManyRequests)
			return
		}
	}
	req, bodyBytes, err := decodeTraceRequest(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	convertStart := time.Now()
	endpoints, tasks, spans, exceptions, aiTraces, aiConversations := convertTraces(c, project, projectId, req)

	var droppedHealthchecks int
	endpoints, spans, droppedHealthchecks = services.FilterHealthchecks(project, endpoints, spans, exceptions)
	if droppedHealthchecks > 0 {
		monitoring.RecordHealthchecksDropped(monitoring.SignalTraces, droppedHealthchecks)
	}

	convertMs := msSince(convertStart)

	insertStart := time.Now()

	if len(endpoints) > 0 {
		if err := repositories.EndpointRepository.InsertAsync(c, endpoints); err != nil {
			c.AbortWithError(500, traceway.NewStackTraceErrorf("error inserting OTEL endpoints: %w", err))
			return
		}
	}

	if len(tasks) > 0 {
		if err := repositories.TaskRepository.InsertAsync(c, tasks); err != nil {
			c.AbortWithError(500, traceway.NewStackTraceErrorf("error inserting OTEL tasks: %w", err))
			return
		}
	}

	if err := repositories.ExceptionStackTraceRepository.InsertAsync(c, exceptions); err != nil {
		c.AbortWithError(500, traceway.NewStackTraceErrorf("error inserting OTEL exceptions: %w", err))
		return
	}

	if err := repositories.SpanRepository.InsertAsync(c, spans); err != nil {
		c.AbortWithError(500, traceway.NewStackTraceErrorf("error inserting OTEL spans: %w", err))
		return
	}

	if len(aiTraces) > 0 {
		if err := repositories.AiTraceRepository.InsertAsync(c, aiTraces); err != nil {
			c.AbortWithError(500, traceway.NewStackTraceErrorf("error inserting OTEL ai traces: %w", err))
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
	totalSize := len(endpoints) + len(tasks) + len(spans) + len(exceptions) + len(aiTraces)
	monitoring.RecordIngestBatch(monitoring.SignalTraces, "traces", convertMs, insertMs, totalSize, bodyBytes)

	var exceptionHashes []string
	for _, ex := range exceptions {
		exceptionHashes = append(exceptionHashes, ex.ExceptionHash)
	}

	if project != nil && project.OrganizationId != nil {
		hooks.BroadcastReport(hooks.ReportEvent{
			OrganizationId:  *project.OrganizationId,
			ProjectId:       projectId,
			EndpointCount:   len(endpoints),
			ErrorCount:      len(exceptions),
			TaskCount:       len(tasks),
			ExceptionHashes: exceptionHashes,
		})
	}

	writeTraceResponse(c)
}

func (o otelController) ExportMetrics(c *gin.Context) {
	monitoring.IngestStarted()
	defer monitoring.IngestFinished()

	projectId, err := middleware.GetProjectId(c)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("UseClientAuth middleware must be applied: %w", err))
		return
	}

	if project, exists := c.Get(middleware.ProjectContextKey); exists {
		if p, ok := project.(*models.Project); ok && p.OrganizationId != nil {
			if attrs := traceway.GetAttributesFromContext(c); attrs != nil {
				attrs.SetTag("organization_id", fmt.Sprintf("%d", *p.OrganizationId))
			}
			if !hooks.CanReport(*p.OrganizationId) {
				monitoring.RecordRateLimited(*p.OrganizationId)
				c.AbortWithStatus(http.StatusTooManyRequests)
				return
			}
		}
	}

	req, bodyBytes, err := decodeMetricsRequest(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	convertStart := time.Now()
	result := convertMetricPoints(projectId, req)
	convertMs := msSince(convertStart)

	insertMs := 0.0
	if len(result.Points) > 0 {
		insertStart := time.Now()
		if err := repositories.MetricPointRepository.InsertAsync(c, result.Points); err != nil {
			c.AbortWithError(500, traceway.NewStackTraceErrorf("error inserting OTEL metric points: %w", err))
			return
		}
		insertMs = msSince(insertStart)

		if len(result.Entries) > 0 {
			go services.AutoRegisterMetricsWithUnits(projectId, result.Entries)
		}
	}

	monitoring.RecordIngestBatch(monitoring.SignalMetrics, "metric_points", convertMs, insertMs, len(result.Points), bodyBytes)

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
	if project, exists := c.Get(middleware.ProjectContextKey); exists {
		if p, ok := project.(*models.Project); ok && p.OrganizationId != nil {
			existingProject = p
			if attrs := traceway.GetAttributesFromContext(c); attrs != nil {
				attrs.SetTag("organization_id", fmt.Sprintf("%d", *p.OrganizationId))
			}
			if !hooks.CanReport(*p.OrganizationId) {
				monitoring.RecordRateLimited(*p.OrganizationId)
				c.AbortWithStatus(http.StatusTooManyRequests)
				return
			}
		}
	}

	req, bodyBytes, err := decodeLogsRequest(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	convertStart := time.Now()
	records := convertLogs(existingProject, c, projectId, req)
	convertMs := msSince(convertStart)

	insertMs := 0.0
	if len(records) > 0 {
		insertStart := time.Now()
		if err := repositories.LogRecordRepository.InsertAsync(c, records); err != nil {
			c.AbortWithError(500, traceway.NewStackTraceErrorf("error inserting OTEL log records: %w", err))
			return
		}
		insertMs = msSince(insertStart)
	}

	monitoring.RecordIngestBatch(monitoring.SignalLogs, "log_records", convertMs, insertMs, len(records), bodyBytes)

	writeLogsResponse(c)
}
