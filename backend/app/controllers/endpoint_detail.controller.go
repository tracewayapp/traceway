package controllers

import (
	"backend/app/models"
	"backend/app/repositories"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	traceway "go.tracewayapp.com"
)

type endpointDetailController struct{}

type EndpointDetailRequest struct {
	ProjectId uuid.UUID `json:"projectId"`
}

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
	Scope         map[string]string `json:"scope,omitempty"`
}

type EndpointDetailResponse struct {
	Endpoint    *models.Endpoint       `json:"endpoint"`
	Segments    []models.Segment       `json:"segments"`
	HasSegments bool                   `json:"hasSegments"`
	Exception   *EndpointExceptionInfo `json:"exception,omitempty"`
	Messages    []EndpointMessageInfo  `json:"messages"`
}

func (t endpointDetailController) GetEndpointDetail(c *gin.Context) {
	endpointId, err := uuid.Parse(c.Param("endpointId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid endpointId"})
		return
	}

	var request EndpointDetailRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get endpoint
	endpoint, err := repositories.EndpointRepository.FindById(c, request.ProjectId, endpointId)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Endpoint not found"})
		return
	}

	// Get segments (flat list ordered by start_time)
	segments, err := repositories.SegmentRepository.FindByTransactionId(c, request.ProjectId, endpointId)
	if err != nil {
		c.AbortWithError(500, traceway.NewStackTraceErrorf("error loading segments: %w", err))
		return
	}

	// Get all linked exceptions and messages
	var exceptionInfo *EndpointExceptionInfo
	var messages []EndpointMessageInfo

	allExceptions, err := repositories.ExceptionStackTraceRepository.FindAllByTransactionId(c, request.ProjectId, endpointId)
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
				Scope:         exc.Scope,
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
		Endpoint:    endpoint,
		Segments:    segments,
		HasSegments: len(segments) > 0,
		Exception:   exceptionInfo,
		Messages:    messages,
	})
}

var EndpointDetailController = endpointDetailController{}
