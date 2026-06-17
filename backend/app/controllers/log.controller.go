package controllers

import (
	"context"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	traceway "go.tracewayapp.com"

	"github.com/tracewayapp/traceway/backend/app/middleware"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories"
)

type logController struct{}

var LogController = logController{}

type LogAttributeFilterRequest struct {
	Scope string `json:"scope"` // "resource" | "scope" | "log"
	Key   string `json:"key"`
	Value string `json:"value"`
}

type LogSearchRequest struct {
	FromDate           time.Time                   `json:"fromDate"`
	ToDate             time.Time                   `json:"toDate"`
	OrderBy            string                      `json:"orderBy"`
	SortDirection      string                      `json:"sortDirection"`
	Search             string                      `json:"search"`
	SearchType         string                      `json:"searchType"`
	MinSeverity        uint8                       `json:"minSeverity"`
	ServiceName        string                      `json:"serviceName"`
	TraceId            string                      `json:"traceId"`
	DistributedTraceId string                      `json:"distributedTraceId"`
	ExcludeTraceId     string                      `json:"excludeTraceId"`
	AttributeFilters   []LogAttributeFilterRequest `json:"attributeFilters"`
	Pagination         PaginationParams            `json:"pagination"`
}

// Max time range allowed for body search without any other selector. Keeps a
// naïve "find all logs containing 'error' for the past 30 days" query from
// scanning the full body column.
const bodySearchUnscopedMaxRange = 24 * time.Hour

func (l logController) List(c *gin.Context) {
	projectId, err := middleware.GetProjectId(c)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}

	var request LogSearchRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
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
			len(request.AttributeFilters) > 0
		rangeTooWide := request.ToDate.Sub(request.FromDate) > bodySearchUnscopedMaxRange
		if !hasSelector && rangeTooWide {
			c.JSON(http.StatusUnprocessableEntity, gin.H{
				"error": "Body search over more than 24 hours requires a filter (service, severity, trace, or attribute). Narrow the time range or add a filter to continue.",
			})
			return
		}
	}

	attrFilters := make([]repositories.LogAttributeFilter, 0, len(request.AttributeFilters))
	for _, f := range request.AttributeFilters {
		if f.Key == "" || f.Scope == "" {
			continue
		}
		attrFilters = append(attrFilters, repositories.LogAttributeFilter{
			Scope: f.Scope,
			Key:   f.Key,
			Value: f.Value,
		})
	}

	params := repositories.LogSearchParams{
		ProjectId:        projectId,
		FromDate:         request.FromDate,
		ToDate:           request.ToDate,
		Search:           request.Search,
		SearchType:       searchType,
		MinSeverity:      request.MinSeverity,
		ServiceName:      request.ServiceName,
		TraceId:          request.TraceId,
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
		traceIds, err := l.resolveDistributedTraceIds(c, dtid, projectId, request.ExcludeTraceId)
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
	records, total, err := repositories.LogRecordRepository.Search(c, params)
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

func (l logController) resolveDistributedTraceIds(ctx context.Context, dtid uuid.UUID, projectId uuid.UUID, excludeTraceHex string) ([]string, error) {
	projectIds := []uuid.UUID{projectId}

	endpoints, err := repositories.EndpointRepository.FindByDistributedTraceId(ctx, dtid, projectIds, nil)
	if err != nil {
		return nil, err
	}
	tasks, err := repositories.TaskRepository.FindByDistributedTraceId(ctx, dtid, projectIds, nil)
	if err != nil {
		return nil, err
	}
	aiTraces, err := repositories.AiTraceRepository.FindByDistributedTraceId(ctx, dtid, projectIds, nil)
	if err != nil {
		return nil, err
	}

	seen := make(map[string]struct{}, len(endpoints)+len(tasks)+len(aiTraces))
	result := make([]string, 0, len(endpoints)+len(tasks)+len(aiTraces))
	add := func(id uuid.UUID) {
		h := hex.EncodeToString(id[:])
		if _, ok := seen[h]; ok {
			return
		}
		if h == excludeTraceHex {
			return
		}
		seen[h] = struct{}{}
		result = append(result, h)
	}
	for _, ep := range endpoints {
		add(ep.Id)
	}
	for _, t := range tasks {
		add(t.Id)
	}
	for _, a := range aiTraces {
		add(a.Id)
	}
	return result, nil
}
