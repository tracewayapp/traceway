package controllers

import (
	"backend/app/middleware"
	"backend/app/models"
	"backend/app/repositories"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	traceway "go.tracewayapp.com"
)

type endpointDetailController struct{}

type EndpointExceptionInfo struct {
	ExceptionHash string `json:"exceptionHash"`
	StackTrace    string `json:"stackTrace"`
	RecordedAt    string `json:"recordedAt"`
}

type EndpointMessageInfo struct {
	Id            uuid.UUID         `json:"id"`
	ExceptionHash string            `json:"exceptionHash"`
	StackTrace    string            `json:"stackTrace"`
	RecordedAt    string            `json:"recordedAt"`
	Attributes    map[string]string `json:"attributes,omitempty"`
}

type EndpointDetailResponse struct {
	Endpoint *models.Endpoint       `json:"endpoint"`
	Spans    []models.Span          `json:"spans"`
	HasSpans bool                   `json:"hasSpans"`
	Exception   *EndpointExceptionInfo `json:"exception,omitempty"`
	Messages    []EndpointMessageInfo  `json:"messages"`
}

func (t endpointDetailController) GetEndpointDetail(c *gin.Context) {
	projectId, err := middleware.GetProjectId(c)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}

	endpointId, err := uuid.Parse(c.Param("endpointId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid endpointId"})
		return
	}

	// Get endpoint
	span := traceway.StartSpan(c, "loading endpoint")
	endpoint, err := repositories.EndpointRepository.FindById(c, projectId, endpointId)
	span.End()
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Endpoint not found"})
		return
	}

	// Get spans (flat list ordered by start_time)
	span = traceway.StartSpan(c, "loading spans")
	spans, err := repositories.SpanRepository.FindByTraceId(c, projectId, endpointId)
	span.End()
	if err != nil {
		c.AbortWithError(500, traceway.NewStackTraceErrorf("error loading spans: %w", err))
		return
	}

	// Get all linked exceptions and messages
	var exceptionInfo *EndpointExceptionInfo
	var messages []EndpointMessageInfo

	span = traceway.StartSpan(c, "loading exceptions")
	allExceptions, err := repositories.ExceptionStackTraceRepository.FindAllByTraceId(c, projectId, endpointId)
	span.End()
	if err != nil {
		c.AbortWithError(500, traceway.NewStackTraceErrorf("error loading all exceptions: %w", err))
		return
	}

	for _, exc := range allExceptions {
		if exc.IsMessage {
			// Add to messages list
			messages = append(messages, EndpointMessageInfo{
				Id:            exc.Id,
				ExceptionHash: exc.ExceptionHash,
				StackTrace:    exc.StackTrace,
				RecordedAt:    exc.RecordedAt.Format("2006-01-02T15:04:05Z07:00"),
				Attributes:    exc.Attributes,
			})
		} else if exceptionInfo == nil {
			// Only take the first actual exception
			exceptionInfo = &EndpointExceptionInfo{
				ExceptionHash: exc.ExceptionHash,
				StackTrace:    exc.StackTrace,
				RecordedAt:    exc.RecordedAt.Format("2006-01-02T15:04:05Z07:00"),
			}
		}
	}

	if messages == nil {
		messages = []EndpointMessageInfo{}
	}

	c.JSON(http.StatusOK, EndpointDetailResponse{
		Endpoint: endpoint,
		Spans:    spans,
		HasSpans: len(spans) > 0,
		Exception:   exceptionInfo,
		Messages:    messages,
	})
}

var EndpointDetailController = endpointDetailController{}
