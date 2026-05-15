package controllers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/middleware"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/notifications"
	"github.com/tracewayapp/traceway/backend/app/repositories"
	traceway "go.tracewayapp.com"
)

type notificationChannelController struct{}

func (ctrl *notificationChannelController) List(ctx *gin.Context) {
	projectId, err := middleware.GetProjectId(ctx)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}

	tx := middleware.GetTx(ctx)
	channels, err := repositories.NotificationChannelRepository.FindByProject(tx, projectId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to list notification channels: %w", err))
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"channels": channels})
}

type createChannelRequest struct {
	Name        string          `json:"name"`
	ChannelType string          `json:"channelType"`
	Config      json.RawMessage `json:"config"`
}

var validChannelTypes = map[string]bool{
	"email": true, "webhook": true, "slack": true, "github": true, "pushover": true,
}

func (ctrl *notificationChannelController) Create(ctx *gin.Context) {
	projectId, err := middleware.GetProjectId(ctx)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}

	var req createChannelRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Name is required."})
		return
	}
	if len(req.Name) > 200 {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Name must be 200 characters or fewer."})
		return
	}
	if !validChannelTypes[req.ChannelType] {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Channel type must be one of: email, webhook, slack, github, pushover."})
		return
	}

	adapter, err := notifications.NewAdapter(req.ChannelType, req.Config)
	if err != nil {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	if err := adapter.Validate(); err != nil {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	userId := middleware.GetUserId(ctx)
	var createdBy *int
	if userId > 0 {
		createdBy = &userId
	}

	tx := middleware.GetTx(ctx)
	now := time.Now().UTC()
	channel := &models.NotificationChannel{
		ProjectId:   projectId,
		Name:        req.Name,
		ChannelType: req.ChannelType,
		Config:      req.Config,
		Enabled:     true,
		CreatedBy:   createdBy,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	id, err := repositories.NotificationChannelRepository.Create(tx, channel)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to create notification channel: %w", err))
		return
	}
	channel.Id = id

	ctx.JSON(http.StatusCreated, channel)
}

func (ctrl *notificationChannelController) Update(ctx *gin.Context) {
	projectId, err := middleware.GetProjectId(ctx)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}

	idStr := ctx.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid channel id"})
		return
	}

	var req createChannelRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Name is required."})
		return
	}
	if len(req.Name) > 200 {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Name must be 200 characters or fewer."})
		return
	}
	if !validChannelTypes[req.ChannelType] {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Channel type must be one of: email, webhook, slack, github, pushover."})
		return
	}

	adapter, err := notifications.NewAdapter(req.ChannelType, req.Config)
	if err != nil {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	if err := adapter.Validate(); err != nil {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	tx := middleware.GetTx(ctx)
	existing, err := repositories.NotificationChannelRepository.FindById(tx, id)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to find notification channel: %w", err))
		return
	}
	if existing == nil || existing.ProjectId != projectId {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "channel not found"})
		return
	}

	existing.Name = req.Name
	existing.ChannelType = req.ChannelType
	existing.Config = req.Config
	existing.UpdatedAt = time.Now().UTC()

	if err := repositories.NotificationChannelRepository.Update(tx, existing); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to update notification channel: %w", err))
		return
	}

	ctx.JSON(http.StatusOK, existing)
}

func (ctrl *notificationChannelController) Delete(ctx *gin.Context) {
	projectId, err := middleware.GetProjectId(ctx)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}

	idStr := ctx.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid channel id"})
		return
	}

	tx := middleware.GetTx(ctx)
	existing, err := repositories.NotificationChannelRepository.FindById(tx, id)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to delete notification channel: %w", err))
		return
	}
	if existing == nil || existing.ProjectId != projectId {
		ctx.JSON(http.StatusOK, gin.H{"deleted": true})
		return
	}

	if err := repositories.NotificationChannelRepository.Delete(tx, id); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to delete notification channel: %w", err))
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"deleted": true})
}

func (ctrl *notificationChannelController) Test(ctx *gin.Context) {
	projectId, err := middleware.GetProjectId(ctx)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}

	idStr := ctx.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid channel id"})
		return
	}

	channel, err := db.ExecuteTransaction(func(tx *sql.Tx) (*models.NotificationChannel, error) {
		return repositories.NotificationChannelRepository.FindById(tx, id)
	})
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to find notification channel: %w", err))
		return
	}
	if channel == nil || channel.ProjectId != projectId {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "channel not found"})
		return
	}

	adapter, err := notifications.NewAdapter(channel.ChannelType, channel.Config)
	if err != nil {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	testMsg := notifications.Message{
		Subject:  "Traceway Test Notification",
		Body:     "This is a test notification from Traceway. If you received this, your notification channel is configured correctly.",
		Severity: notifications.SeverityInfo,
		RuleType: "test",
		RuleName: "Test",
	}

	if err := adapter.Send(ctx.Request.Context(), testMsg); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("test notification failed: %w", err))
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"success": true})
}

var NotificationChannelController = notificationChannelController{}
