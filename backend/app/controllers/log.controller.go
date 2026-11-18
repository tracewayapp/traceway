package controllers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/tracewayapp/traceway/backend/v2/app/middleware"
	"github.com/tracewayapp/traceway/backend/v2/app/models"
	"github.com/tracewayapp/traceway/backend/v2/app/repositories/telemetry"
	"github.com/tracewayapp/traceway/backend/v2/app/repositories/telemetry/shared"
	"github.com/tracewayapp/traceway/backend/v2/app/traceway"
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

// LogFilterRequest is the selection shared by the log list and its histogram.
type LogFilterRequest struct {
	FromDate         time.Time                   `json:"fromDate"`
	ToDate           time.Time                   `json:"toDate"`
	Search           string                      `json:"search"`
	SearchType       string                      `json:"searchType"`
	MinSeverity      uint8                       `json:"minSeverity"`
	ServiceName      string                      `json:"serviceName"`
	TraceId          string                      `json:"traceId"`
	WholeTrace       bool                        `json:"wholeTrace"`
	SpanId           string                      `json:"spanId"`
	ScopeName        string                      `json:"scopeName"`
	Body             string                      `json:"body"`
	AttributeFilters []LogAttributeFilterRequest `json:"attributeFilters"`
}

type LogSearchRequest struct {
	LogFilterRequest
	OrderBy       string              `json:"orderBy"`
	SortDirection string              `json:"sortDirection"`
	Pagination    LogPaginationParams `json:"pagination"`
}

type LogHistogramRequest struct {
	LogFilterRequest
	MaxBuckets int `json:"maxBuckets"`
}

type LogHistogramResponse struct {
	IntervalSeconds int64                       `json:"intervalSeconds"`
	Total           int64                       `json:"total"`
	Buckets         []shared.LogHistogramBucket `json:"buckets"`
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

	params, ok := l.searchParams(c, projectId, &request.LogFilterRequest)
	if !ok {
		return
	}
	params.OrderBy = request.OrderBy
	params.SortDirection = request.SortDirection
	params.Page = request.Pagination.Page
	params.PageSize = request.Pagination.PageSize

	spanCtx, span := traceway.StartSpan(c, "loading logs")
	traceway.SetAttribute(spanCtx, "logs.range_seconds", strconv.FormatFloat(request.ToDate.Sub(request.FromDate).Seconds(), 'f', 0, 64))
	traceway.SetAttribute(spanCtx, "logs.trace_filter", strconv.FormatBool(request.TraceId != ""))
	traceway.SetAttribute(spanCtx, "logs.body_search", strconv.FormatBool(request.Search != "" && params.SearchType == "body"))
	traceway.SetAttribute(spanCtx, "logs.attribute_filter_count", strconv.Itoa(len(params.AttributeFilters)))
	traceway.SetAttribute(spanCtx, "logs.page", strconv.Itoa(request.Pagination.Page))
	traceway.SetAttribute(spanCtx, "logs.page_size", strconv.Itoa(request.Pagination.PageSize))
	records, total, err := telemetry.LogRecordRepository.Search(spanCtx, params)
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

// Histogram counts the logs List would return per time bucket and level, for the
// volume strip above the log table. The bucket width is a round step chosen so the
// range splits into at most maxBuckets buckets (default 80).
func (l logController) Histogram(c *gin.Context) {
	projectId, err := middleware.GetProjectId(c)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}

	var request LogHistogramRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		middleware.RejectBindError(c, err, err.Error())
		return
	}
	if request.ToDate.Before(request.FromDate) {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "toDate must not be before fromDate"})
		return
	}
	maxBuckets := request.MaxBuckets
	if maxBuckets == 0 {
		maxBuckets = shared.LogHistogramDefaultBuckets
	}
	maxBuckets = min(max(maxBuckets, shared.LogHistogramMinBuckets), shared.LogHistogramMaxBuckets)

	params, ok := l.searchParams(c, projectId, &request.LogFilterRequest)
	if !ok {
		return
	}

	step := shared.LogHistogramStep(request.FromDate, request.ToDate, maxBuckets)
	spanCtx, span := traceway.StartSpan(c, "loading log histogram")
	traceway.SetAttribute(spanCtx, "logs.range_seconds", strconv.FormatFloat(request.ToDate.Sub(request.FromDate).Seconds(), 'f', 0, 64))
	traceway.SetAttribute(spanCtx, "logs.histogram_step_seconds", strconv.FormatInt(step, 10))
	traceway.SetAttribute(spanCtx, "logs.attribute_filter_count", strconv.Itoa(len(params.AttributeFilters)))
	rows, err := telemetry.LogRecordRepository.Histogram(spanCtx, params, step)
	span.End()
	if err != nil {
		c.AbortWithError(500, traceway.NewStackTraceErrorf("error loading log histogram: %w", err))
		return
	}

	buckets := shared.BuildLogHistogram(request.FromDate, request.ToDate, step, rows)
	var total int64
	for _, b := range buckets {
		total += b.Total()
	}
	c.JSON(http.StatusOK, LogHistogramResponse{IntervalSeconds: step, Total: total, Buckets: buckets})
}

// searchParams validates the shared log selection and turns it into repository
// params. It writes the error response itself and returns false when the request
// must stop.
func (l logController) searchParams(c *gin.Context, projectId uuid.UUID, request *LogFilterRequest) (telemetry.LogSearchParams, bool) {
	if request.TraceId != "" {
		request.TraceId = shared.NormalizeTraceId(request.TraceId)
		if !validTraceHex(request.TraceId) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid traceId"})
			return telemetry.LogSearchParams{}, false
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
			request.SpanId != "" ||
			request.ScopeName != "" ||
			request.Body != "" ||
			len(request.AttributeFilters) > 0
		rangeTooWide := request.ToDate.Sub(request.FromDate) > bodySearchUnscopedMaxRange
		if !hasSelector && rangeTooWide {
			c.JSON(http.StatusUnprocessableEntity, gin.H{
				"error": "Body search over more than 24 hours requires a filter (service, severity, trace, or attribute). Narrow the time range or add a filter to continue.",
			})
			return telemetry.LogSearchParams{}, false
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
	}
	if request.WholeTrace && request.TraceId == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "wholeTrace requires a traceId"})
		return telemetry.LogSearchParams{}, false
	}
	if request.WholeTrace {
		projects, err := organizationProjects(c, projectId)
		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("error listing trace projects: %w", err))
			return telemetry.LogSearchParams{}, false
		}
		for _, project := range projects {
			params.ProjectIds = append(params.ProjectIds, project.Id)
		}
	}
	return params, true
}

func rejectRetiredLogSelectors(data []byte) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	for _, key := range []string{"distributedTraceId", "excludeTraceId"} {
		if _, exists := fields[key]; exists {
			return fmt.Errorf("%s is no longer supported; use traceId to select one trace", key)
		}
	}
	return nil
}

func (r *LogSearchRequest) UnmarshalJSON(data []byte) error {
	type request LogSearchRequest
	if err := rejectRetiredLogSelectors(data); err != nil {
		return err
	}
	return json.Unmarshal(data, (*request)(r))
}

func (r *LogHistogramRequest) UnmarshalJSON(data []byte) error {
	type request LogHistogramRequest
	if err := rejectRetiredLogSelectors(data); err != nil {
		return err
	}
	return json.Unmarshal(data, (*request)(r))
}
