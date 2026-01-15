package controllers

import (
	"backend/app/models"
	"backend/app/repositories"
	"net/http"

	"github.com/gin-gonic/gin"
)

type endpointDetailController struct{}

type EndpointDetailRequest struct {
	ProjectId string `json:"projectId"`
}

type EndpointExceptionInfo struct {
	ExceptionHash string `json:"exceptionHash"`
	StackTrace    string `json:"stackTrace"`
	RecordedAt    string `json:"recordedAt"`
}

type EndpointDetailResponse struct {
	Endpoint    *models.Endpoint       `json:"endpoint"`
	Segments    []models.Segment       `json:"segments"`
	HasSegments bool                   `json:"hasSegments"`
	Exception   *EndpointExceptionInfo `json:"exception,omitempty"`
}

func (c endpointDetailController) GetEndpointDetail(ctx *gin.Context) {
	endpointId := ctx.Param("endpointId")
	if endpointId == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "endpointId is required"})
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

	// Get linked exception if any
	var exceptionInfo *EndpointExceptionInfo
	exception, err := repositories.ExceptionStackTraceRepository.FindByTransactionId(ctx, request.ProjectId, endpointId)
	if err != nil {
		panic(err)
	}
	if exception != nil {
		exceptionInfo = &EndpointExceptionInfo{
			ExceptionHash: exception.ExceptionHash,
			StackTrace:    exception.StackTrace,
			RecordedAt:    exception.RecordedAt.Format("2006-01-02T15:04:05Z07:00"),
		}
	}

	ctx.JSON(http.StatusOK, EndpointDetailResponse{
		Endpoint:    endpoint,
		Segments:    segments,
		HasSegments: len(segments) > 0,
		Exception:   exceptionInfo,
	})
}

var EndpointDetailController = endpointDetailController{}
