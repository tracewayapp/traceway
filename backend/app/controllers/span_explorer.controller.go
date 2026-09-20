package controllers

import (
	"context"
	"database/sql"
	"encoding/hex"
	"errors"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/middleware"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry/shared"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
	traceway "go.tracewayapp.com"
)

type spanExplorerController struct{}

var SpanExplorerController = spanExplorerController{}

type SpanAttributeFilterRequest struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type SpanSearchRequest struct {
	FromDate         time.Time                    `json:"fromDate"`
	ToDate           time.Time                    `json:"toDate"`
	OrderBy          string                       `json:"orderBy"`
	ServiceName      string                       `json:"serviceName"`
	Name             string                       `json:"name"`
	TraceId          string                       `json:"traceId"`
	Kind             *int32                       `json:"kind"`
	Status           *int32                       `json:"status"`
	MinDurationMs    float64                      `json:"minDurationMs"`
	MaxDurationMs    float64                      `json:"maxDurationMs"`
	AttributeFilters []SpanAttributeFilterRequest `json:"attributeFilters"`
	Pagination       PaginationParams             `json:"pagination"`
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

func validTraceHex(id string) bool {
	decoded, err := hex.DecodeString(id)
	return err == nil && len(decoded) == 16
}

// An OTel span id is 8 bytes. A span of the native protocol keeps its client's 16 byte id.
func validSpanHex(id string) bool {
	decoded, err := hex.DecodeString(id)
	return err == nil && (len(decoded) == 8 || len(decoded) == 16)
}

func (s spanExplorerController) searchParams(request SpanSearchRequest) (shared.OtelSpanSearch, string) {
	search := shared.OtelSpanSearch{From: request.FromDate, To: request.ToDate, OrderBy: request.OrderBy, Service: request.ServiceName,
		Name: request.Name, TraceId: shared.NormalizeTraceId(request.TraceId), Kind: request.Kind, Status: request.Status,
		MinDuration: time.Duration(request.MinDurationMs * float64(time.Millisecond)), MaxDuration: time.Duration(request.MaxDurationMs * float64(time.Millisecond)),
		Page: request.Pagination.Page, PageSize: request.Pagination.PageSize}
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
		if strings.ContainsAny(filter.Key, `"\`) || len(filter.Key) > 256 {
			return search, "Attribute keys cannot contain quotes or backslashes."
		}
		// A ClickHouse Map answers '' for a key it does not hold, so an empty value would match every span without the attribute there and none elsewhere.
		if filter.Value == "" {
			return search, "An attribute filter needs a value."
		}
		search.Attributes = append(search.Attributes, shared.OtelAttributeFilter{Key: filter.Key, Value: filter.Value})
	}
	return search, ""
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
	search, problem := s.searchParams(request)
	if problem != "" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": problem})
		return
	}
	search.ProjectId = projectId
	ctx, cancel := context.WithTimeout(c.Request.Context(), 20*time.Second)
	defer cancel()

	span := traceway.StartSpan(c, "searching spans")
	found, total, err := telemetry.OtelSpanRepository.Search(ctx, search)
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

// organizationProjects lists what a whole trace is read from: every project of the current project's organization.
// Services of one trace usually report to different projects, and membership of the organization is what grants
// read access to each of them. Other organizations of the same user stay out.
func organizationProjects(c *gin.Context, projectId uuid.UUID) ([]*models.Project, error) {
	projects, err := db.ExecuteTransaction(func(tx *sql.Tx) ([]*models.Project, error) {
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
	span := traceway.StartSpan(c, "loading trace spans")
	graph, err := telemetry.SpanRepository.FindTrace(c, projectIds, traceId, at)
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
