package controllers

import (
	"context"
	"database/sql"
	"encoding/hex"
	"errors"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/v2/app/db"
	"github.com/tracewayapp/traceway/backend/v2/app/middleware"
	"github.com/tracewayapp/traceway/backend/v2/app/models"
	"github.com/tracewayapp/traceway/backend/v2/app/repositories/telemetry"
	"github.com/tracewayapp/traceway/backend/v2/app/repositories/telemetry/shared"
	"github.com/tracewayapp/traceway/backend/v2/app/repositories/transactional"
	"github.com/tracewayapp/traceway/backend/v2/app/traceway"
)

type spanExplorerController struct{}

var SpanExplorerController = spanExplorerController{}

// SpanAttributeFilterRequest matches one span attribute with the operators of
// the logs page: Contains for a substring, Exclude to negate, Exists to match on
// the key alone.
type SpanAttributeFilterRequest struct {
	Key      string `json:"key"`
	Value    string `json:"value"`
	Exclude  bool   `json:"exclude"`
	Contains bool   `json:"contains"`
	Exists   bool   `json:"exists"`
}

// SpanFilterRequest is the selection the span search and its charts share.
type SpanFilterRequest struct {
	FromDate         time.Time                    `json:"fromDate"`
	ToDate           time.Time                    `json:"toDate"`
	ServiceName      string                       `json:"serviceName"`
	Name             string                       `json:"name"`
	TraceId          string                       `json:"traceId"`
	Kind             *int32                       `json:"kind"`
	Status           *int32                       `json:"status"`
	MinDurationMs    float64                      `json:"minDurationMs"`
	MaxDurationMs    float64                      `json:"maxDurationMs"`
	AttributeFilters []SpanAttributeFilterRequest `json:"attributeFilters"`
}

type SpanSearchRequest struct {
	SpanFilterRequest
	OrderBy    string           `json:"orderBy"`
	Pagination PaginationParams `json:"pagination"`
}

// SpanChartRequest charts the spans a search matches: their duration in
// milliseconds, or with Attribute the numeric values of that attribute. A
// series takes MaxBuckets, a distribution Bins.
type SpanChartRequest struct {
	SpanFilterRequest
	Attribute  string `json:"attribute"`
	MaxBuckets int    `json:"maxBuckets"`
	Bins       int    `json:"bins"`
}

type SpanSearchResponse struct {
	Data       []models.Span `json:"data"`
	Pagination Pagination    `json:"pagination"`
	Services   []string      `json:"services"`
}

type SpanAttributesResponse struct {
	Attributes        map[string]string `json:"attributes"`
	AttributesOmitted bool              `json:"attributesOmitted,omitempty"`
}

type SpanTraceProject struct {
	Id   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

type SpanTraceResponse struct {
	SpanGraphStatus *models.SpanGraphStatus `json:"spanGraphStatus"`
	Spans           []models.Span           `json:"spans"`
	// Projects names every project that holds spans of the trace, so rows from another project can say where they live.
	Projects []SpanTraceProject `json:"projects"`
}

// The spans table is the largest one. A search reads every span of the project in the range, so the range is capped.
const spanSearchMaxRange = 31*24*time.Hour + 5*time.Minute

const spanSearchTooSlow = "The search took too long. Narrow the time range or add a filter."

const spanSearchTimeout = 20 * time.Second

func validTraceHex(id string) bool {
	decoded, err := hex.DecodeString(id)
	return err == nil && len(decoded) == 16 && strings.Trim(id, "0") != ""
}

// An OTel span id is 8 bytes. A span of the native protocol keeps its client's 16 byte id.
func validSpanHex(id string) bool {
	decoded, err := hex.DecodeString(id)
	return err == nil && (len(decoded) == 8 || len(decoded) == 16)
}

// spanAttributeKeyProblem rejects keys the SQLite and DuckDB JSON paths cannot quote.
func spanAttributeKeyProblem(key string) string {
	if strings.ContainsAny(key, `"\`) || len(key) > 256 {
		return "Attribute keys cannot contain quotes or backslashes."
	}
	return ""
}

func (s spanExplorerController) searchParams(request SpanFilterRequest) (shared.OtelSpanSearch, string) {
	search := shared.OtelSpanSearch{From: request.FromDate, To: request.ToDate, Service: request.ServiceName,
		Name: request.Name, TraceId: shared.NormalizeTraceId(request.TraceId), Kind: request.Kind, Status: request.Status,
		MinDuration: time.Duration(request.MinDurationMs * float64(time.Millisecond)), MaxDuration: time.Duration(request.MaxDurationMs * float64(time.Millisecond))}
	switch {
	case request.FromDate.IsZero() || request.ToDate.IsZero():
		return search, "A time range is required."
	case request.ToDate.Before(request.FromDate):
		return search, "The time range ends before it starts."
	case request.ToDate.Sub(request.FromDate) > spanSearchMaxRange:
		return search, "Span search covers at most 31 days. Narrow the time range."
	case search.TraceId != "" && !validTraceHex(search.TraceId):
		return search, "A trace ID is 32 hexadecimal characters."
	case request.Kind != nil && (*request.Kind < 0 || *request.Kind > 5), request.Status != nil && (*request.Status < 0 || *request.Status > 2):
		return search, "Unknown span kind or status."
	case len(request.AttributeFilters) > shared.MaxOtelSearchAttributeFilters:
		return search, "Too many attribute filters."
	case request.MinDurationMs < 0 || request.MaxDurationMs < 0 ||
		math.IsNaN(request.MinDurationMs) || math.IsNaN(request.MaxDurationMs) ||
		request.MinDurationMs >= float64(math.MaxInt64)/float64(time.Millisecond) ||
		request.MaxDurationMs >= float64(math.MaxInt64)/float64(time.Millisecond):
		return search, "Duration limits must be non-negative and fit within 292 years."
	case request.MaxDurationMs > 0 && request.MinDurationMs > request.MaxDurationMs:
		return search, "The maximum duration must be at least the minimum duration."
	}
	for _, filter := range request.AttributeFilters {
		if filter.Key == "" {
			continue
		}
		if problem := spanAttributeKeyProblem(filter.Key); problem != "" {
			return search, problem
		}
		if filter.Exists {
			search.Attributes = append(search.Attributes, shared.OtelAttributeFilter{Key: filter.Key, Exists: true, Exclude: filter.Exclude})
			continue
		}
		// A ClickHouse Map answers '' for a key it does not hold, so an empty value would match every span without the attribute there and none elsewhere.
		if filter.Value == "" {
			return search, "An attribute filter needs a value. To find spans without an attribute, filter on the key not existing."
		}
		search.Attributes = append(search.Attributes, shared.OtelAttributeFilter{Key: filter.Key, Value: filter.Value, Exclude: filter.Exclude, Contains: filter.Contains})
	}
	return search, ""
}

// traceSpanSearch records on the request's span what a search or chart was asked for: the range and which filters
// narrowed it. Filter values stay out, they are whatever the user typed.
func traceSpanSearch(ctx context.Context, search shared.OtelSpanSearch) {
	traceway.SetAttribute(ctx, "spans.range_seconds", strconv.FormatFloat(search.To.Sub(search.From).Seconds(), 'f', 0, 64))
	traceway.SetAttribute(ctx, "spans.name_filter", strconv.FormatBool(search.Name != ""))
	traceway.SetAttribute(ctx, "spans.service_filter", strconv.FormatBool(search.Service != ""))
	traceway.SetAttribute(ctx, "spans.trace_filter", strconv.FormatBool(search.TraceId != ""))
	traceway.SetAttribute(ctx, "spans.kind_filter", strconv.FormatBool(search.Kind != nil))
	traceway.SetAttribute(ctx, "spans.status_filter", strconv.FormatBool(search.Status != nil))
	traceway.SetAttribute(ctx, "spans.duration_filter", strconv.FormatBool(search.MinDuration > 0 || search.MaxDuration > 0))
	traceway.SetAttribute(ctx, "spans.attribute_filter_count", strconv.Itoa(len(search.Attributes)))
}

func spanSearchTimedOut(ctx context.Context, err error) bool {
	return errors.Is(err, context.DeadlineExceeded) || ctx.Err() != nil || telemetry.OtelSpanRepository.IsReadLimitError(err)
}

func (s spanExplorerController) Search(c *gin.Context) {
	projectId, err := middleware.GetProjectId(c)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}
	var request SpanSearchRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		middleware.RejectBindError(c, err, err.Error())
		return
	}
	search, problem := s.searchParams(request.SpanFilterRequest)
	if problem != "" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": problem})
		return
	}
	search.ProjectId = projectId
	search.OrderBy, search.Page, search.PageSize = request.OrderBy, request.Pagination.Page, request.Pagination.PageSize
	ctx, cancel := context.WithTimeout(c.Request.Context(), spanSearchTimeout)
	defer cancel()

	spanCtx, span := traceway.StartSpan(ctx, "searching spans")
	traceSpanSearch(spanCtx, search)
	traceway.SetAttribute(spanCtx, "spans.order_by", search.OrderBy)
	traceway.SetAttribute(spanCtx, "spans.page", strconv.Itoa(search.Page))
	traceway.SetAttribute(spanCtx, "spans.page_size", strconv.Itoa(search.PageSize))
	found, total, err := telemetry.OtelSpanRepository.Search(spanCtx, search)
	if err == nil {
		traceway.SetAttribute(spanCtx, "spans.total", strconv.FormatUint(total, 10))
	} else {
		traceway.SetAttribute(spanCtx, "spans.read_limit_hit", strconv.FormatBool(spanSearchTimedOut(ctx, err)))
	}
	span.End()
	if err != nil {
		if spanSearchTimedOut(ctx, err) {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": spanSearchTooSlow})
			return
		}
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("error searching spans: %w", err))
		return
	}
	services, err := telemetry.OtelSpanRepository.Services(ctx, projectId, search.From, search.To)
	if err != nil {
		if !spanSearchTimedOut(ctx, err) {
			traceway.CaptureException(traceway.NewStackTraceErrorf("error listing span services: %w", err))
		}
		services = []string{}
	}
	spans := make([]models.Span, len(found))
	for i, item := range found {
		spans[i] = item.Span
	}
	c.JSON(http.StatusOK, SpanSearchResponse{Data: spans, Services: services, Pagination: Pagination{
		Page: search.Page, PageSize: search.PageSize, Total: int64(total), TotalPages: (int64(total) + int64(search.PageSize) - 1) / int64(search.PageSize)}})
}

// chartRequest reads the selection of a span chart. It answers the request itself and returns false when it must stop.
func (s spanExplorerController) chartRequest(c *gin.Context) (SpanChartRequest, shared.OtelSpanSearch, bool) {
	var request SpanChartRequest
	projectId, err := middleware.GetProjectId(c)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return request, shared.OtelSpanSearch{}, false
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		middleware.RejectBindError(c, err, err.Error())
		return request, shared.OtelSpanSearch{}, false
	}
	search, problem := s.searchParams(request.SpanFilterRequest)
	if problem == "" && request.Attribute != "" {
		problem = spanAttributeKeyProblem(request.Attribute)
	}
	if problem != "" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": problem})
		return request, search, false
	}
	search.ProjectId = projectId
	return request, search, true
}

// A chart reads every span the search matches, like its count, so a slow one answers the way a slow search does.
func (s spanExplorerController) answerChartError(c *gin.Context, ctx context.Context, err error) {
	if spanSearchTimedOut(ctx, err) {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": spanSearchTooSlow})
		return
	}
	c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("error charting spans: %w", err))
}

// Series charts the duration of the matching spans, or one numeric attribute, over time: count, sum, average,
// minimum, maximum and P50/P95/P99 per bucket.
func (s spanExplorerController) Series(c *gin.Context) {
	request, search, ok := s.chartRequest(c)
	if !ok {
		return
	}
	maxBuckets := request.MaxBuckets
	if maxBuckets == 0 {
		maxBuckets = shared.LogHistogramDefaultBuckets
	}
	maxBuckets = min(max(maxBuckets, shared.LogHistogramMinBuckets), shared.LogHistogramMaxBuckets)
	step := shared.LogHistogramStep(search.From, search.To, maxBuckets)
	ctx, cancel := context.WithTimeout(c.Request.Context(), spanSearchTimeout)
	defer cancel()

	spanCtx, span := traceway.StartSpan(ctx, "charting spans")
	traceSpanSearch(spanCtx, search)
	traceway.SetAttribute(spanCtx, "spans.chart_attribute", strconv.FormatBool(request.Attribute != ""))
	traceway.SetAttribute(spanCtx, "spans.chart_step_seconds", strconv.FormatInt(step, 10))
	rows, err := telemetry.OtelSpanRepository.ChartStats(spanCtx, search, request.Attribute, step)
	span.End()
	if err != nil {
		s.answerChartError(c, ctx, err)
		return
	}
	buckets, present, numeric := shared.BuildChartSeries(rows)
	c.JSON(http.StatusOK, ChartSeriesResponse{IntervalSeconds: step, Present: present, Numeric: numeric, Buckets: buckets})
}

// Distribution counts the duration of the matching spans, or one numeric attribute, in bins next to its summary
// statistics. Durations that span a wide range get bins of equal ratio (log scale).
func (s spanExplorerController) Distribution(c *gin.Context) {
	request, search, ok := s.chartRequest(c)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), spanSearchTimeout)
	defer cancel()

	spanCtx, span := traceway.StartSpan(ctx, "charting span distribution")
	traceSpanSearch(spanCtx, search)
	traceway.SetAttribute(spanCtx, "spans.chart_attribute", strconv.FormatBool(request.Attribute != ""))
	stats, err := telemetry.OtelSpanRepository.ChartStats(spanCtx, search, request.Attribute, 0)
	var response ChartDistributionResponse
	if err == nil {
		response, err = chartDistribution(stats, chartBinCount(request.Bins), request.Attribute == "", func(binning shared.ChartBinning) ([]shared.ChartBinRow, error) {
			return telemetry.OtelSpanRepository.ChartBins(spanCtx, search, request.Attribute, binning)
		})
	}
	span.End()
	if err != nil {
		s.answerChartError(c, ctx, err)
		return
	}
	c.JSON(http.StatusOK, response)
}

// organizationProjects lists what a whole trace is read from: every project of the current project's organization.
// Services of one trace usually report to different projects, and membership of the organization is what grants
// read access to each of them. Other organizations of the same user stay out.
func organizationProjects(c *gin.Context, projectId uuid.UUID) ([]*models.Project, error) {
	projects, err := db.ExecuteTransactionContext(c, func(tx *sql.Tx) ([]*models.Project, error) {
		return transactional.ProjectRepository.FindByUserId(tx, middleware.GetUserId(c))
	})
	if err != nil {
		return nil, err
	}
	var organization *int
	for _, project := range projects {
		if project.Id == projectId {
			organization = project.OrganizationId
		}
	}
	readable := make([]*models.Project, 0, len(projects))
	for _, project := range projects {
		if project.Id == projectId || (organization != nil && project.OrganizationId != nil && *project.OrganizationId == *organization) {
			readable = append(readable, project)
		}
	}
	if len(readable) == 0 {
		readable = append(readable, &models.Project{Id: projectId})
	}
	return readable, nil
}

func (s spanExplorerController) GetTrace(c *gin.Context) {
	projectId, err := middleware.GetProjectId(c)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}
	traceId := strings.ToLower(c.Param("traceId"))
	at, err := time.Parse(time.RFC3339Nano, c.Query("at"))
	if !validTraceHex(traceId) || err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "A trace needs a 32 character hexadecimal ID and an RFC 3339 `at` time."})
		return
	}
	readable, err := organizationProjects(c, projectId)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("error listing the projects a trace is read from: %w", err))
		return
	}
	projectIds := make([]uuid.UUID, len(readable))
	for i, project := range readable {
		projectIds[i] = project.Id
	}
	spanCtx, span := traceway.StartSpan(c, "loading trace spans")
	graph, err := telemetry.SpanRepository.FindTrace(spanCtx, projectIds, traceId, at)
	span.End()
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("error loading trace spans: %w", err))
		return
	}
	if len(graph.Spans) == 0 && graph.Status.State == models.SpanGraphComplete {
		c.JSON(http.StatusNotFound, gin.H{"error": "Trace not found"})
		return
	}
	holds := make(map[uuid.UUID]bool)
	for _, found := range graph.Spans {
		holds[found.ProjectId] = true
	}
	projects := []SpanTraceProject{}
	for _, project := range readable {
		if holds[project.Id] {
			projects = append(projects, SpanTraceProject{Id: project.Id, Name: project.Name})
		}
	}
	c.JSON(http.StatusOK, SpanTraceResponse{SpanGraphStatus: &graph.Status, Spans: graph.Spans, Projects: projects})
}

// GetSpanAttributes serves the popover of one span in the whole trace view, which loads without attributes.
func (s spanExplorerController) GetSpanAttributes(c *gin.Context) {
	projectId, err := middleware.GetProjectId(c)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}
	traceId, spanId := strings.ToLower(c.Param("traceId")), strings.ToLower(c.Param("spanId"))
	at, err := time.Parse(time.RFC3339Nano, c.Query("at"))
	if !validTraceHex(traceId) || !validSpanHex(spanId) || err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Span attributes need a 32 character trace ID, a 16 or 32 character span ID and an RFC 3339 `at` time."})
		return
	}
	found, err := telemetry.SpanRepository.FindSpanAttributes(c, projectId, traceId, spanId, at)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("error loading span attributes: %w", err))
		return
	}
	if found == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Span not found"})
		return
	}
	attributes := found.Attributes
	if attributes == nil {
		attributes = map[string]string{}
	}
	c.JSON(http.StatusOK, SpanAttributesResponse{Attributes: attributes, AttributesOmitted: found.Omitted != ""})
}
