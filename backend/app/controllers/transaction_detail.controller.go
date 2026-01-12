package controllers

import (
	"backend/app/models"
	"backend/app/repositories"
	"net/http"

	"github.com/gin-gonic/gin"
)

type transactionDetailController struct{}

type TransactionDetailRequest struct {
	ProjectId string `json:"projectId"`
}

type ExceptionInfo struct {
	ExceptionHash string `json:"exceptionHash"`
	StackTrace    string `json:"stackTrace"`
	RecordedAt    string `json:"recordedAt"`
}

type TransactionDetailResponse struct {
	Transaction *models.Transaction `json:"transaction"`
	Segments    []models.Segment    `json:"segments"`
	HasSegments bool                `json:"hasSegments"`
	Exception   *ExceptionInfo      `json:"exception,omitempty"`
}

func (c transactionDetailController) GetTransactionDetail(ctx *gin.Context) {
	transactionId := ctx.Param("transactionId")
	if transactionId == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "transactionId is required"})
		return
	}

	var request TransactionDetailRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get transaction
	transaction, err := repositories.TransactionRepository.FindById(ctx, request.ProjectId, transactionId)
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Transaction not found"})
		return
	}

	// Get segments (flat list ordered by start_time)
	segments, err := repositories.SegmentRepository.FindByTransactionId(ctx, request.ProjectId, transactionId)
	if err != nil {
		panic(err)
	}

	// Get linked exception if any
	var exceptionInfo *ExceptionInfo
	exception, err := repositories.ExceptionStackTraceRepository.FindByTransactionId(ctx, request.ProjectId, transactionId)
	if err != nil {
		panic(err)
	}
	if exception != nil {
		exceptionInfo = &ExceptionInfo{
			ExceptionHash: exception.ExceptionHash,
			StackTrace:    exception.StackTrace,
			RecordedAt:    exception.RecordedAt.Format("2006-01-02T15:04:05Z07:00"),
		}
	}

	ctx.JSON(http.StatusOK, TransactionDetailResponse{
		Transaction: transaction,
		Segments:    segments,
		HasSegments: len(segments) > 0,
		Exception:   exceptionInfo,
	})
}

var TransactionDetailController = transactionDetailController{}
