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

type TransactionDetailResponse struct {
	Transaction *models.Transaction `json:"transaction"`
	Segments    []models.Segment    `json:"segments"`
	HasSegments bool                `json:"hasSegments"`
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

	ctx.JSON(http.StatusOK, TransactionDetailResponse{
		Transaction: transaction,
		Segments:    segments,
		HasSegments: len(segments) > 0,
	})
}

var TransactionDetailController = transactionDetailController{}
