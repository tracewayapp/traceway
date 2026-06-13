package controllers

import (
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

type notificationRuleController struct{}

var validRuleTypes = map[string]bool{
	"error_rate_threshold":    true,
	"endpoint_p95_threshold":  true,
	"endpoint_p99_threshold":  true,
	"apdex_drop":              true,
	"metric_threshold":        true,
	"no_data":                 true,
	"error_count_threshold":   true,
	"task_duration_threshold": true,
	"task_failure_rate":       true,
	"throughput_drop":         true,
	"endpoint_error_rate":     true,
	"new_error":               true,
	"error_regression":        true,
	"impact_score_critical":   true,
	"impact_score_high":       true,
	"impact_score_medium":     true,
	"ai_trace_cost":           true,
}

func (ctrl *notificationRuleController) List(ctx *gin.Context) {
	projectId, err := middleware.GetProjectId(ctx)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}

	tx := db.GetTx(ctx)
	rules, err := repositories.NotificationRuleRepository.FindByProjectWithChannel(tx, projectId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to list notification rules: %w", err))
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"rules": rules})
}

type createRuleRequest struct {
	ChannelId       int             `json:"channelId"`
	Name            string          `json:"name"`
	RuleType        string          `json:"ruleType"`
	Config          json.RawMessage `json:"config"`
	CooldownMinutes int             `json:"cooldownMinutes"`
	Severity        string          `json:"severity"`
}

var validSeverities = map[string]bool{
	"":         true,
	"critical": true,
	"warning":  true,
	"info":     true,
}

func (ctrl *notificationRuleController) Create(ctx *gin.Context) {
	projectId, err := middleware.GetProjectId(ctx)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}

	var req createRuleRequest
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
	if !validRuleTypes[req.RuleType] {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Invalid rule type."})
		return
	}
	if !validSeverities[req.Severity] {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Severity must be critical, warning, info, or empty for auto."})
		return
	}
	if req.CooldownMinutes < 0 {
		req.CooldownMinutes = 15
	}

	tx := db.GetTx(ctx)

	channel, err := repositories.NotificationChannelRepository.FindById(tx, req.ChannelId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to find notification channel: %w", err))
		return
	}
	if channel == nil || channel.ProjectId != projectId {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Channel not found in this project."})
		return
	}

	userId := middleware.GetUserId(ctx)
	var createdBy *int
	if userId > 0 {
		createdBy = &userId
	}

	now := time.Now().UTC()
	cooldown := req.CooldownMinutes
	if cooldown == 0 {
		cooldown = 15
	}
	rule := &models.NotificationRule{
		ProjectId:       projectId,
		ChannelId:       req.ChannelId,
		Name:            req.Name,
		RuleType:        req.RuleType,
		Config:          req.Config,
		Enabled:         true,
		CooldownMinutes: cooldown,
		Severity:        req.Severity,
		CreatedBy:       createdBy,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	id, err := repositories.NotificationRuleRepository.Create(tx, rule)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to create notification rule: %w", err))
		return
	}
	rule.Id = id

	ctx.JSON(http.StatusCreated, rule)
}

func (ctrl *notificationRuleController) Update(ctx *gin.Context) {
	projectId, err := middleware.GetProjectId(ctx)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}

	idStr := ctx.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid rule id"})
		return
	}

	var req createRuleRequest
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
	if !validRuleTypes[req.RuleType] {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Invalid rule type."})
		return
	}
	if !validSeverities[req.Severity] {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Severity must be critical, warning, info, or empty for auto."})
		return
	}

	tx := db.GetTx(ctx)

	existing, err := repositories.NotificationRuleRepository.FindById(tx, id)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to find notification rule: %w", err))
		return
	}
	if existing == nil || existing.ProjectId != projectId {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "rule not found"})
		return
	}

	channel, err := repositories.NotificationChannelRepository.FindById(tx, req.ChannelId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to find notification channel: %w", err))
		return
	}
	if channel == nil || channel.ProjectId != projectId {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Channel not found in this project."})
		return
	}

	cooldown := req.CooldownMinutes
	if cooldown <= 0 {
		cooldown = 15
	}

	existing.ChannelId = req.ChannelId
	existing.Name = req.Name
	existing.RuleType = req.RuleType
	existing.Config = req.Config
	existing.CooldownMinutes = cooldown
	existing.Severity = req.Severity
	existing.UpdatedAt = time.Now().UTC()

	if err := repositories.NotificationRuleRepository.Update(tx, existing); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to update notification rule: %w", err))
		return
	}
	notifications.ClearRuleState(id)

	ctx.JSON(http.StatusOK, existing)
}

func (ctrl *notificationRuleController) Delete(ctx *gin.Context) {
	projectId, err := middleware.GetProjectId(ctx)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}

	idStr := ctx.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid rule id"})
		return
	}

	tx := db.GetTx(ctx)
	existing, err := repositories.NotificationRuleRepository.FindById(tx, id)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to delete notification rule: %w", err))
		return
	}
	if existing == nil || existing.ProjectId != projectId {
		ctx.JSON(http.StatusOK, gin.H{"deleted": true})
		return
	}

	if err := repositories.NotificationRuleRepository.Delete(tx, id); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to delete notification rule: %w", err))
		return
	}
	notifications.ClearRuleState(id)

	ctx.JSON(http.StatusOK, gin.H{"deleted": true})
}

func (ctrl *notificationRuleController) Toggle(ctx *gin.Context) {
	projectId, err := middleware.GetProjectId(ctx)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}

	idStr := ctx.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid rule id"})
		return
	}

	tx := db.GetTx(ctx)
	existing, err := repositories.NotificationRuleRepository.FindById(tx, id)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to find notification rule: %w", err))
		return
	}
	if existing == nil || existing.ProjectId != projectId {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "rule not found"})
		return
	}

	newEnabled := !existing.Enabled
	if err := repositories.NotificationRuleRepository.UpdateEnabled(tx, id, newEnabled); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to toggle notification rule: %w", err))
		return
	}

	existing.Enabled = newEnabled
	ctx.JSON(http.StatusOK, existing)
}

type snoozeRequest struct {
	DurationMinutes int `json:"durationMinutes"`
}

func (ctrl *notificationRuleController) Snooze(ctx *gin.Context) {
	projectId, err := middleware.GetProjectId(ctx)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}

	idStr := ctx.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid rule id"})
		return
	}

	var req snoozeRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	tx := db.GetTx(ctx)
	existing, err := repositories.NotificationRuleRepository.FindById(tx, id)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to find notification rule: %w", err))
		return
	}
	if existing == nil || existing.ProjectId != projectId {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "rule not found"})
		return
	}

	var snoozedUntil *time.Time
	if req.DurationMinutes > 0 {
		t := time.Now().UTC().Add(time.Duration(req.DurationMinutes) * time.Minute)
		snoozedUntil = &t
	}

	if err := repositories.NotificationRuleRepository.UpdateSnoozedUntil(tx, id, snoozedUntil); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to snooze notification rule: %w", err))
		return
	}

	existing.SnoozedUntil = snoozedUntil
	ctx.JSON(http.StatusOK, existing)
}

var NotificationRuleController = notificationRuleController{}
