package controllers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/middleware"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/notifications"
	"github.com/tracewayapp/traceway/backend/app/oncall"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
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
	"ai_conversation_cost":    true,
	"ai_flagged_content":      true,
	"check_down":              true,
	"slo_burn_rate":           true,
	"slo_budget_consumed":     true,
}

func (ctrl *notificationRuleController) List(ctx *gin.Context) {
	projectId, err := middleware.GetProjectId(ctx)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}

	tx := db.GetTx(ctx)
	rules, err := transactional.NotificationRuleRepository.FindByProjectWithChannel(tx, projectId)
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
		middleware.RejectBindError(ctx, err, "Invalid request body")
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

	channel, err := transactional.NotificationChannelRepository.FindById(tx, req.ChannelId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to find notification channel: %w", err))
		return
	}
	if channel == nil || channel.ProjectId != projectId {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Channel not found in this project."})
		return
	}
	message, err := validateSLORuleConfig(tx, projectId, req.RuleType, req.Config)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to validate the SLO rule: %w", err))
		return
	}
	if message != "" {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": message})
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

	id, err := transactional.NotificationRuleRepository.Create(tx, rule)
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
		middleware.RejectBindError(ctx, err, "Invalid request body")
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

	existing, err := transactional.NotificationRuleRepository.FindById(tx, id)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to find notification rule: %w", err))
		return
	}
	if existing == nil || existing.ProjectId != projectId {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "rule not found"})
		return
	}

	channel, err := transactional.NotificationChannelRepository.FindById(tx, req.ChannelId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to find notification channel: %w", err))
		return
	}
	if channel == nil || channel.ProjectId != projectId {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Channel not found in this project."})
		return
	}
	message, err := validateSLORuleConfig(tx, projectId, req.RuleType, req.Config)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to validate the SLO rule: %w", err))
		return
	}
	if message != "" {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": message})
		return
	}

	cooldown := req.CooldownMinutes
	if cooldown <= 0 {
		cooldown = 15
	}

	conditionChanged := existing.RuleType != req.RuleType || existing.ChannelId != req.ChannelId || !sameRuleConfig(existing.Config, req.Config)
	if conditionChanged {
		if err := resolveSLORulePage(tx, existing); err != nil {
			ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to resolve the page of the edited rule: %w", err))
			return
		}
	}

	existing.ChannelId = req.ChannelId
	existing.Name = req.Name
	existing.RuleType = req.RuleType
	existing.Config = req.Config
	existing.CooldownMinutes = cooldown
	existing.Severity = req.Severity
	existing.UpdatedAt = time.Now().UTC()

	if err := transactional.NotificationRuleRepository.Update(tx, existing); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to update notification rule: %w", err))
		return
	}
	if conditionChanged {
		notifications.ClearRuleState(id)
		if err := transactional.NotificationRuleStateRepository.Delete(tx, id); err != nil {
			ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to reset notification rule state: %w", err))
			return
		}
	}

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
	existing, err := transactional.NotificationRuleRepository.FindById(tx, id)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to delete notification rule: %w", err))
		return
	}
	if existing == nil || existing.ProjectId != projectId {
		ctx.JSON(http.StatusOK, gin.H{"deleted": true})
		return
	}

	if err := resolveSLORulePage(tx, existing); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to resolve the page of the deleted rule: %w", err))
		return
	}
	if err := transactional.NotificationRuleRepository.Delete(tx, id); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to delete notification rule: %w", err))
		return
	}
	notifications.ClearRuleState(id)

	ctx.JSON(http.StatusOK, gin.H{"deleted": true})
}

type toggleRequest struct {
	Enabled *bool `json:"enabled"`
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
	existing, err := transactional.NotificationRuleRepository.FindById(tx, id)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to find notification rule: %w", err))
		return
	}
	if existing == nil || existing.ProjectId != projectId {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "rule not found"})
		return
	}

	// An explicit enabled makes the call idempotent (agents set a state);
	// the dashboard sends an empty body and flips the rule.
	var req toggleRequest
	if err := ctx.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		middleware.RejectBindError(ctx, err, "Invalid request body")
		return
	}
	newEnabled := !existing.Enabled
	if req.Enabled != nil {
		newEnabled = *req.Enabled
	}
	if !newEnabled {
		if err := resolveSLORulePage(tx, existing); err != nil {
			ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to resolve the page of the disabled rule: %w", err))
			return
		}
	}
	if err := transactional.NotificationRuleRepository.UpdateEnabled(tx, id, newEnabled); err != nil {
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
		middleware.RejectBindError(ctx, err, "Invalid request body")
		return
	}

	tx := db.GetTx(ctx)
	existing, err := transactional.NotificationRuleRepository.FindById(tx, id)
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

	if err := transactional.NotificationRuleRepository.UpdateSnoozedUntil(tx, id, snoozedUntil); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to snooze notification rule: %w", err))
		return
	}

	existing.SnoozedUntil = snoozedUntil
	ctx.JSON(http.StatusOK, existing)
}

var NotificationRuleController = notificationRuleController{}

func validateSLORuleConfig(tx *sql.Tx, projectId uuid.UUID, ruleType string, config json.RawMessage) (string, error) {
	switch ruleType {
	case "slo_burn_rate":
		var cfg struct {
			SloId              int     `json:"sloId"`
			LongWindowMinutes  int     `json:"longWindowMinutes"`
			ShortWindowMinutes int     `json:"shortWindowMinutes"`
			BurnRateThreshold  float64 `json:"burnRateThreshold"`
		}
		if err := json.Unmarshal(config, &cfg); err != nil {
			return "The rule configuration is not valid JSON.", nil
		}
		if cfg.LongWindowMinutes < 5 || cfg.LongWindowMinutes > 10080 {
			return "Long window must be between 5 minutes and 7 days.", nil
		}
		if cfg.ShortWindowMinutes < 1 || cfg.ShortWindowMinutes >= cfg.LongWindowMinutes {
			return "Short window must be at least 1 minute and shorter than the long window.", nil
		}
		if cfg.BurnRateThreshold <= 0 {
			return "Burn rate threshold must be above 0.", nil
		}
		return validateRuleSLO(tx, projectId, cfg.SloId)
	case "slo_budget_consumed":
		var cfg struct {
			SloId           int     `json:"sloId"`
			ConsumedPercent float64 `json:"consumedPercent"`
		}
		if err := json.Unmarshal(config, &cfg); err != nil {
			return "The rule configuration is not valid JSON.", nil
		}
		if cfg.ConsumedPercent <= 0 || cfg.ConsumedPercent > 100 {
			return "Budget consumed must be between 1 and 100 percent.", nil
		}
		return validateRuleSLO(tx, projectId, cfg.SloId)
	}
	return "", nil
}

func resolveSLORulePage(tx *sql.Tx, rule *models.NotificationRule) error {
	sloId := sloRuleTarget(rule.RuleType, rule.Config)
	if sloId == 0 {
		return nil
	}
	if _, err := oncall.AutoResolveByDedupKeyTx(tx, rule.Id, notifications.SLODedupToken(sloId), time.Now().UTC()); err != nil {
		return err
	}
	return transactional.NotificationRuleStateRepository.Delete(tx, rule.Id)
}

func sameRuleConfig(a, b json.RawMessage) bool {
	var left, right any
	if json.Unmarshal(a, &left) != nil || json.Unmarshal(b, &right) != nil {
		return false
	}
	return reflect.DeepEqual(left, right)
}

func validateRuleSLO(tx *sql.Tx, projectId uuid.UUID, sloId int) (string, error) {
	if sloId < 1 {
		return "Select an SLO.", nil
	}
	entity, err := transactional.SLORepository.FindByIdForProject(tx, sloId, projectId)
	if err != nil {
		return "", err
	}
	if entity == nil {
		return "SLO not found in this project.", nil
	}
	return "", nil
}
