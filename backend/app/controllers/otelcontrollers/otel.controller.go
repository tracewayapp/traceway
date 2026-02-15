package otelcontrollers

import (
	"backend/app/hooks"
	"backend/app/middleware"
	"backend/app/models"
	"backend/app/repositories"
	"net/http"

	"github.com/gin-gonic/gin"
	traceway "go.tracewayapp.com"
)

type otelController struct{}

var OtelController = otelController{}

func (o otelController) ExportTraces(c *gin.Context) {
	projectId, err := middleware.GetProjectId(c)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("UseClientAuth middleware must be applied: %w", err))
		return
	}

	if project, exists := c.Get(middleware.ProjectContextKey); exists {
		if p, ok := project.(*models.Project); ok && p.OrganizationId != nil {
			if !hooks.CanReport(*p.OrganizationId) {
				c.AbortWithStatus(http.StatusTooManyRequests)
				return
			}
		}
	}

	req, err := decodeTraceRequest(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	endpoints, tasks, spans, exceptions := convertTraces(projectId, req)

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

	if project, exists := c.Get(middleware.ProjectContextKey); exists {
		if p, ok := project.(*models.Project); ok && p.OrganizationId != nil {
			hooks.BroadcastReport(hooks.ReportEvent{
				OrganizationId: *p.OrganizationId,
				EndpointCount:  len(endpoints),
				ErrorCount:     len(exceptions),
				TaskCount:      len(tasks),
			})
		}
	}

	writeTraceResponse(c)
}

func (o otelController) ExportMetrics(c *gin.Context) {
	projectId, err := middleware.GetProjectId(c)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("UseClientAuth middleware must be applied: %w", err))
		return
	}

	if project, exists := c.Get(middleware.ProjectContextKey); exists {
		if p, ok := project.(*models.Project); ok && p.OrganizationId != nil {
			if !hooks.CanReport(*p.OrganizationId) {
				c.AbortWithStatus(http.StatusTooManyRequests)
				return
			}
		}
	}

	req, err := decodeMetricsRequest(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	records := convertMetrics(projectId, req, "")

	if err := repositories.MetricRecordRepository.InsertAsync(c, records); err != nil {
		c.AbortWithError(500, traceway.NewStackTraceErrorf("error inserting OTEL metrics: %w", err))
		return
	}

	writeMetricsResponse(c)
}
