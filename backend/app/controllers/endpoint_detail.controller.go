package controllers

import (
	"backend/app/models"
	"backend/app/repositories"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
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

func (c endpointDetailController) GetEndpointDetail(ctx *gin.Context) {
	endpointId, err := uuid.Parse(ctx.Param("endpointId"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid endpointId"})
		return
	}

	var request EndpointDetailRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get endpoint
	endpoint, err := repositories.EndpointRepository.FindById(ctx, request.ProjectId, endpointId)
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Endpoint not found"})
		return
	}

	// Get segments (flat list ordered by start_time)
	segments, err := repositories.SegmentRepository.FindByTransactionId(ctx, request.ProjectId, endpointId)
	if err != nil {
		panic(err)
	}

	// Get all linked exceptions and messages
	var exceptionInfo *EndpointExceptionInfo
	var messages []EndpointMessageInfo

	allExceptions, err := repositories.ExceptionStackTraceRepository.FindAllByTransactionId(ctx, request.ProjectId, endpointId)
	if err != nil {
		panic(err)
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

	ctx.JSON(http.StatusOK, EndpointDetailResponse{
		Endpoint:    endpoint,
		Segments:    segments,
		HasSegments: len(segments) > 0,
		Exception:   exceptionInfo,
		Messages:    messages,
	})
}

var EndpointDetailController = endpointDetailController{}
