package controllers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tracewayapp/traceway/backend/app/middleware"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories"
	traceway "go.tracewayapp.com"
)

type notificationHistoryController struct{}

type NotificationHistorySearchRequest struct {
	Pagination PaginationParams `json:"pagination"`
	Search     string           `json:"search"`
	FromDate   string           `json:"fromDate"`
	ToDate     string           `json:"toDate"`
}

func (ctrl *notificationHistoryController) List(ctx *gin.Context) {
	projectId, err := middleware.GetProjectId(ctx)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}

	var request NotificationHistorySearchRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	page := request.Pagination.Page
	pageSize := request.Pagination.PageSize

	var fromTime, toTime *time.Time
	if request.FromDate != "" {
		t, err := time.Parse(time.RFC3339, request.FromDate)
		if err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid fromDate format"})
			return
		}
		fromTime = &t
	}
	if request.ToDate != "" {
		t, err := time.Parse(time.RFC3339, request.ToDate)
		if err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid toDate format"})
			return
		}
		toTime = &t
	}

	items, total, err := repositories.FiredNotificationRepository.FindByProject(ctx.Request.Context(), projectId, page, pageSize, request.Search, fromTime, toTime)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to list notification history: %w", err))
		return
	}

	totalPages := total / int64(pageSize)
	if total%int64(pageSize) != 0 {
		totalPages++
	}

	if items == nil {
		items = []*models.NotificationHistoryEntry{}
	}

	ctx.JSON(http.StatusOK, PaginatedResponse[*models.NotificationHistoryEntry]{
		Data: items,
		Pagination: Pagination{
			Page:       page,
			PageSize:   pageSize,
			Total:      total,
			TotalPages: totalPages,
		},
	})
}

var NotificationHistoryController = notificationHistoryController{}
