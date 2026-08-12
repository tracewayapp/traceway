package controllers

import (
	"encoding/json"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/middleware"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry"
	"github.com/tracewayapp/traceway/backend/app/storage"
	traceway "go.tracewayapp.com"
)

type aiTraceController struct{}

type aiTraceDetailRequest struct {
	RecordedAt *time.Time `json:"recordedAt"`
}

type AiTraceSearchRequest struct {
	FromDate      time.Time        `json:"fromDate"`
	ToDate        time.Time        `json:"toDate"`
	OrderBy       string           `json:"orderBy"`
	SortDirection string           `json:"sortDirection"`
	Pagination    PaginationParams `json:"pagination"`
	Search        string           `json:"search"`
	RootFilter    string           `json:"rootFilter"`
}

type AiTraceInstancesRequest struct {
	FromDate      time.Time        `json:"fromDate"`
	ToDate        time.Time        `json:"toDate"`
	OrderBy       string           `json:"orderBy"`
	SortDirection string           `json:"sortDirection"`
	Pagination    PaginationParams `json:"pagination"`
}

type AiTraceInstancesResponse struct {
	Data       []models.AiTrace           `json:"data"`
	Stats      *models.AiTraceDetailStats `json:"stats"`
	Pagination Pagination                 `json:"pagination"`
}

type AiTraceDetailResponse struct {
	AiTrace      *models.AiTrace `json:"aiTrace"`
	Conversation json.RawMessage `json:"conversation,omitempty"`
}

type AiConversationSearchRequest struct {
	FromDate      time.Time        `json:"fromDate"`
	ToDate        time.Time        `json:"toDate"`
	OrderBy       string           `json:"orderBy"`
	SortDirection string           `json:"sortDirection"`
	Pagination    PaginationParams `json:"pagination"`
	Search        string           `json:"search"`
	UserId        string           `json:"userId"`
	Model         string           `json:"model"`
	ToolName      string           `json:"toolName"`
	FlaggedOnly   bool             `json:"flaggedOnly"`
}

type AiConversationFacets struct {
	Models []string `json:"models"`
	Tools  []string `json:"tools"`
}

type AiConversationListResponse struct {
	Data       []models.AiConversationStats     `json:"data"`
	Pagination Pagination                       `json:"pagination"`
	Thresholds *models.AiConversationThresholds `json:"thresholds"`
	Facets     AiConversationFacets             `json:"facets"`
}

type AiConversationDetailRequest struct {
	ConversationId string    `json:"conversationId"`
	FromDate       time.Time `json:"fromDate"`
	ToDate         time.Time `json:"toDate"`
}

type AiConversationTurn struct {
	models.AiTrace
	Input  string `json:"input"`
	Output string `json:"output"`
}

type AiConversationDetailResponse struct {
	Data  []AiConversationTurn              `json:"data"`
	Stats *models.AiConversationDetailStats `json:"stats"`
}

type AiUserSearchRequest struct {
	FromDate      time.Time        `json:"fromDate"`
	ToDate        time.Time        `json:"toDate"`
	OrderBy       string           `json:"orderBy"`
	SortDirection string           `json:"sortDirection"`
	Pagination    PaginationParams `json:"pagination"`
	Search        string           `json:"search"`
}

func (a aiTraceController) FindGroupedByTraceName(c *gin.Context) {
	projectId, err := middleware.GetProjectId(c)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}

	var request AiTraceSearchRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	stats, total, err := telemetry.AiTraceRepository.FindGroupedByTraceName(c, projectId, request.FromDate, request.ToDate, request.Pagination.Page, request.Pagination.PageSize, request.OrderBy, request.SortDirection, request.Search, request.RootFilter)
	if err != nil {
		c.AbortWithError(500, traceway.NewStackTraceErrorf("error loading ai trace stats: %w", err))
		return
	}

	c.JSON(http.StatusOK, PaginatedResponse[models.AiTraceStats]{
		Data: stats,
		Pagination: Pagination{
			Page:       request.Pagination.Page,
			PageSize:   request.Pagination.PageSize,
			Total:      total,
			TotalPages: (total + int64(request.Pagination.PageSize) - 1) / int64(request.Pagination.PageSize),
		},
	})
}

func (a aiTraceController) FindByTraceName(c *gin.Context) {
	projectId, err := middleware.GetProjectId(c)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}

	rawTraceName := c.Query("traceName")
	if rawTraceName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "traceName is required"})
		return
	}

	traceName, err := url.PathUnescape(rawTraceName)
	if err != nil {
		traceName = rawTraceName
	}

	var request AiTraceInstancesRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	traces, total, err := telemetry.AiTraceRepository.FindByTraceName(c, projectId, traceName, request.FromDate, request.ToDate, request.Pagination.Page, request.Pagination.PageSize, request.OrderBy, request.SortDirection)
	if err != nil {
		c.AbortWithError(500, traceway.NewStackTraceErrorf("error loading ai traces: %w", err))
		return
	}

	stats, err := telemetry.AiTraceRepository.GetTraceNameStats(c, projectId, traceName, request.FromDate, request.ToDate)
	if err != nil {
		stats = nil
	}

	c.JSON(http.StatusOK, AiTraceInstancesResponse{
		Data:  traces,
		Stats: stats,
		Pagination: Pagination{
			Page:       request.Pagination.Page,
			PageSize:   request.Pagination.PageSize,
			Total:      total,
			TotalPages: (total + int64(request.Pagination.PageSize) - 1) / int64(request.Pagination.PageSize),
		},
	})
}

func (a aiTraceController) GetAiTraceDetail(c *gin.Context) {
	projectId, err := middleware.GetProjectId(c)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}

	traceId, err := uuid.Parse(c.Param("traceId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid traceId"})
		return
	}

	var request aiTraceDetailRequest
	_ = c.ShouldBindJSON(&request)

	aiTrace, err := telemetry.AiTraceRepository.FindById(c, projectId, traceId, request.RecordedAt)
	if aiTrace == nil && err == nil && request.RecordedAt != nil {
		aiTrace, err = telemetry.AiTraceRepository.FindById(c, projectId, traceId, nil)
	}
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("error loading ai trace: %w", err))
		return
	}
	if aiTrace == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "AI trace not found"})
		return
	}

	response := AiTraceDetailResponse{
		AiTrace: aiTrace,
	}

	if aiTrace.StorageKey != "" {
		data, err := storage.Store.Read(c, aiTrace.StorageKey)
		if err == nil {
			response.Conversation = json.RawMessage(data)
		} else {
			traceway.CaptureException(traceway.NewStackTraceErrorf("failed to read AI trace conversation (key=%s): %w", aiTrace.StorageKey, err))
		}
	}

	c.JSON(http.StatusOK, response)
}

func (a aiTraceController) FindConversations(c *gin.Context) {
	projectId, err := middleware.GetProjectId(c)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}

	var request AiConversationSearchRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	stats, total, thresholds, err := telemetry.AiTraceRepository.FindConversations(c, projectId, request.FromDate, request.ToDate, request.Pagination.Page, request.Pagination.PageSize, request.OrderBy, request.SortDirection, request.Search, request.UserId, request.Model, request.ToolName, request.FlaggedOnly)
	if err != nil {
		c.AbortWithError(500, traceway.NewStackTraceErrorf("error loading ai conversations: %w", err))
		return
	}

	facetModels, err := telemetry.AiTraceRepository.ListModels(c, projectId, request.FromDate, request.ToDate)
	if err != nil {
		facetModels = nil
		traceway.CaptureException(traceway.NewStackTraceErrorf("error loading ai conversation model facets: %w", err))
	}
	facetTools, err := telemetry.AiTraceRepository.ListToolNames(c, projectId, request.FromDate, request.ToDate)
	if err != nil {
		facetTools = nil
		traceway.CaptureException(traceway.NewStackTraceErrorf("error loading ai conversation tool facets: %w", err))
	}

	c.JSON(http.StatusOK, AiConversationListResponse{
		Data: stats,
		Pagination: Pagination{
			Page:       request.Pagination.Page,
			PageSize:   request.Pagination.PageSize,
			Total:      total,
			TotalPages: (total + int64(request.Pagination.PageSize) - 1) / int64(request.Pagination.PageSize),
		},
		Thresholds: thresholds,
		Facets:     AiConversationFacets{Models: facetModels, Tools: facetTools},
	})
}

// conversationPayloadCap bounds how many turns get their stored prompt and
// completion blobs attached on the detail endpoint; each blob can carry a
// full resent message history, so unbounded reads would balloon the response.
const conversationPayloadCap = 200

func (a aiTraceController) GetConversationDetail(c *gin.Context) {
	projectId, err := middleware.GetProjectId(c)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}

	var request AiConversationDetailRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if request.ConversationId == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "conversationId is required"})
		return
	}

	traces, stats, err := telemetry.AiTraceRepository.FindByConversationId(c, projectId, request.ConversationId, request.FromDate, request.ToDate)
	if err != nil {
		c.AbortWithError(500, traceway.NewStackTraceErrorf("error loading ai conversation: %w", err))
		return
	}
	if len(traces) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Conversation not found"})
		return
	}

	turns := make([]AiConversationTurn, len(traces))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 8)
	for i, trace := range traces {
		turns[i] = AiConversationTurn{AiTrace: trace}
		if trace.StorageKey == "" || i >= conversationPayloadCap {
			continue
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, storageKey string) {
			defer wg.Done()
			defer func() { <-sem }()
			data, err := storage.Store.Read(c, storageKey)
			if err != nil {
				// The blob write is best-effort at ingest, so a missing payload
				// is expected for some turns; the turn still renders from its
				// row data.
				return
			}
			var payload struct {
				Input  string `json:"input"`
				Output string `json:"output"`
			}
			if err := json.Unmarshal(data, &payload); err != nil {
				return
			}
			turns[i].Input = payload.Input
			turns[i].Output = payload.Output
		}(i, trace.StorageKey)
	}
	wg.Wait()

	c.JSON(http.StatusOK, AiConversationDetailResponse{
		Data:  turns,
		Stats: stats,
	})
}

func (a aiTraceController) FindAiUsers(c *gin.Context) {
	projectId, err := middleware.GetProjectId(c)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}

	var request AiUserSearchRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	stats, total, err := telemetry.AiTraceRepository.FindUserStats(c, projectId, request.FromDate, request.ToDate, request.Pagination.Page, request.Pagination.PageSize, request.OrderBy, request.SortDirection, request.Search)
	if err != nil {
		c.AbortWithError(500, traceway.NewStackTraceErrorf("error loading ai user stats: %w", err))
		return
	}

	c.JSON(http.StatusOK, PaginatedResponse[models.AiUserStats]{
		Data: stats,
		Pagination: Pagination{
			Page:       request.Pagination.Page,
			PageSize:   request.Pagination.PageSize,
			Total:      total,
			TotalPages: (total + int64(request.Pagination.PageSize) - 1) / int64(request.Pagination.PageSize),
		},
	})
}

var AiTraceController = aiTraceController{}
