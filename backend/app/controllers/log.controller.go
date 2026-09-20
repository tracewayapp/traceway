package controllers

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	traceway "go.tracewayapp.com"

	"github.com/tracewayapp/traceway/backend/app/middleware"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry/shared"
)

type logController struct{}

var LogController = logController{}

type LogAttributeFilterRequest struct {
	Scope    string `json:"scope"` // "resource" | "scope" | "log"
	Key      string `json:"key"`
	Value    string `json:"value"`
	Exclude  bool   `json:"exclude"`
	Contains bool   `json:"contains"`
}

type LogPaginationParams struct {
	Page     int `json:"page" binding:"min=1"`
	PageSize int `json:"pageSize" binding:"min=1,max=500"`
}

type LogSearchRequest struct {
	DistributedTraceId string                      `json:"distributedTraceId"`
	ExcludeTraceId     string                      `json:"excludeTraceId"`
	FromDate           time.Time                   `json:"fromDate"`
	ToDate             time.Time                   `json:"toDate"`
	OrderBy            string                      `json:"orderBy"`
	SortDirection      string                      `json:"sortDirection"`
	Search             string                      `json:"search"`
	SearchType         string                      `json:"searchType"`
	MinSeverity        uint8                       `json:"minSeverity"`
	ServiceName        string                      `json:"serviceName"`
	TraceId            string                      `json:"traceId"`
	SpanId             string                      `json:"spanId"`
	ScopeName          string                      `json:"scopeName"`
	Body               string                      `json:"body"`
	AttributeFilters   []LogAttributeFilterRequest `json:"attributeFilters"`
	Pagination         LogPaginationParams         `json:"pagination"`
}

// Max time range allowed for body search without any other selector. Keeps a
// naïve "find all logs containing 'error' for the past 30 days" query from
// scanning the full body column. The slack matters: the frontend's "24h"
// preset rounds the range end up to the end of the current minute, so the
// received range is slightly over 24h and must not trip the gate.
const bodySearchUnscopedMaxRange = 24*time.Hour + 5*time.Minute

func (l logController) List(c *gin.Context) {
	projectId, err := middleware.GetProjectId(c)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}

	var request LogSearchRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		middleware.RejectBindError(c, err, err.Error())
		return
	}

	if request.ExcludeTraceId != "" {
		if request.DistributedTraceId == "" || !validTraceHex(shared.NormalizeTraceId(request.ExcludeTraceId)) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "excludeTraceId requires distributedTraceId and a valid trace ID"})
			return
		}
	}

	// Loki-style gate: a body substring search without any selector can scan
	// the entire body column across the requested range. Require at least one
	// selector (service / severity / trace / attribute) OR a short time range.
	searchType := request.SearchType
	if searchType == "" {
		searchType = "body"
	}
	if request.Search != "" && searchType == "body" {
		hasSelector := request.MinSeverity > 0 ||
			request.ServiceName != "" ||
			request.TraceId != "" ||
			request.DistributedTraceId != "" ||
			request.SpanId != "" ||
			request.ScopeName != "" ||
			request.Body != "" ||
			len(request.AttributeFilters) > 0
		rangeTooWide := request.ToDate.Sub(request.FromDate) > bodySearchUnscopedMaxRange
		if !hasSelector && rangeTooWide {
			c.JSON(http.StatusUnprocessableEntity, gin.H{
				"error": "Body search over more than 24 hours requires a filter (service, severity, trace, or attribute). Narrow the time range or add a filter to continue.",
			})
			return
		}
	}

	attrFilters := make([]telemetry.LogAttributeFilter, 0, len(request.AttributeFilters))
	for _, f := range request.AttributeFilters {
		if f.Key == "" || f.Scope == "" {
			continue
		}
		attrFilters = append(attrFilters, telemetry.LogAttributeFilter{
			Scope:    f.Scope,
			Key:      f.Key,
			Value:    f.Value,
			Exclude:  f.Exclude,
			Contains: f.Contains,
		})
	}

	params := telemetry.LogSearchParams{
		ProjectId:        projectId,
		FromDate:         request.FromDate,
		ToDate:           request.ToDate,
		Search:           request.Search,
		SearchType:       searchType,
		MinSeverity:      request.MinSeverity,
		ServiceName:      request.ServiceName,
		TraceId:          request.TraceId,
		SpanId:           request.SpanId,
		ScopeName:        request.ScopeName,
		Body:             request.Body,
		AttributeFilters: attrFilters,
		OrderBy:          request.OrderBy,
		SortDirection:    request.SortDirection,
		Page:             request.Pagination.Page,
		PageSize:         request.Pagination.PageSize,
	}

	if request.DistributedTraceId != "" {
		dtid, err := uuid.Parse(request.DistributedTraceId)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid distributedTraceId"})
			return
		}
		traceIds, err := l.resolveDistributedTraceIds(c, dtid, projectId, request.ExcludeTraceId, request.FromDate)
		if err != nil {
			c.AbortWithError(500, traceway.NewStackTraceErrorf("error resolving distributed trace: %w", err))
			return
		}
		if len(traceIds) == 0 {
			c.JSON(http.StatusOK, PaginatedResponse[models.LogRecord]{
				Data: []models.LogRecord{},
				Pagination: Pagination{
					Page:       request.Pagination.Page,
					PageSize:   request.Pagination.PageSize,
					Total:      0,
					TotalPages: 0,
				},
			})
			return
		}
		params.TraceIds = traceIds
		params.TraceId = ""
	}

	span := traceway.StartSpan(c, "loading logs")
	records, total, err := telemetry.LogRecordRepository.Search(c, params)
	span.End()
	if err != nil {
		c.AbortWithError(500, traceway.NewStackTraceErrorf("error loading logs: %w", err))
		return
	}

	c.JSON(http.StatusOK, PaginatedResponse[models.LogRecord]{
		Data: records,
		Pagination: Pagination{
			Page:       request.Pagination.Page,
			PageSize:   request.Pagination.PageSize,
			Total:      total,
			TotalPages: (total + int64(request.Pagination.PageSize) - 1) / int64(request.Pagination.PageSize),
		},
	})
}

func (l logController) resolveDistributedTraceIds(ctx context.Context, id uuid.UUID, projectId uuid.UUID, exclude string, at time.Time) ([]string, error) {
	var anchor *time.Time
	if !at.IsZero() {
		anchor = &at
	}
	traceId := shared.NormalizeTraceId(id.String())
	found, err := findTraceEntities(ctx, traceId, []uuid.UUID{projectId}, anchor)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var ids []string
	exclude = shared.NormalizeTraceId(exclude)
	add := func(values ...string) {
		for _, value := range values {
			if value != "" && value != exclude && !seen[value] {
				seen[value] = true
				ids = append(ids, value)
			}
		}
	}
	add(traceId)
	for _, row := range found.endpoints {
		add(row.TraceId, row.LinkedTraceId)
	}
	for _, row := range found.tasks {
		add(row.TraceId, row.LinkedTraceId)
	}
	for _, row := range found.aiTraces {
		add(row.TraceId, row.LinkedTraceId)
	}
	for _, row := range found.exceptions {
		add(row.TraceId, row.LinkedTraceId)
	}
	return ids, nil
}
