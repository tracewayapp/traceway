package controllers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/middleware"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/notifications"
	"github.com/tracewayapp/traceway/backend/app/oncall"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"

	"github.com/google/uuid"
	traceway "go.tracewayapp.com"
)

type notificationChannelController struct{}

func (ctrl *notificationChannelController) List(ctx *gin.Context) {
	projectId, err := middleware.GetProjectId(ctx)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}

	tx := db.GetTx(ctx)
	channels, err := transactional.NotificationChannelRepository.FindByProject(tx, projectId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to list notification channels: %w", err))
		return
	}

	masked := make([]channelResponse, 0, len(channels))
	for _, channel := range channels {
		response, err := maskChannel(channel)
		if err != nil {
			ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to mask notification channel %d: %w", channel.Id, err))
			return
		}
		masked = append(masked, response)
	}

	ctx.JSON(http.StatusOK, gin.H{"channels": masked})
}

// channelResponse is a channel as the dashboard sees it: credentials replaced
// by notifications.SecretSentinel, with HasSecrets naming the fields that
// carry one so the form can offer to keep them.
type channelResponse struct {
	*models.NotificationChannel
	Config     json.RawMessage `json:"config"`
	HasSecrets []string        `json:"hasSecrets"`
}

func maskChannel(channel *models.NotificationChannel) (channelResponse, error) {
	config, hasSecrets, err := notifications.MaskSecretFields(channel.ChannelType, channel.Config)
	if err != nil {
		return channelResponse{}, err
	}
	return channelResponse{NotificationChannel: channel, Config: config, HasSecrets: hasSecrets}, nil
}

type createChannelRequest struct {
	Name        string          `json:"name"`
	ChannelType string          `json:"channelType"`
	Config      json.RawMessage `json:"config"`
}

var builtinChannelTypes = []string{"email", "webhook", "slack", "github", "pushover", "telegram", "escalation", notifications.AgentChannelType}

func validChannelType(channelType string) bool {
	return slices.Contains(builtinChannelTypes, channelType) || slices.Contains(notifications.RegisteredAdapterTypes(), channelType)
}

func channelTypeError() string {
	return "Channel type must be one of: " + strings.Join(append(append([]string{}, builtinChannelTypes...), notifications.RegisteredAdapterTypes()...), ", ") + "."
}

// validateEscalationChannelConfig checks the {policyId} config against the
// project's organization. Escalation channels have no adapter, so this
// replaces the NewAdapter validation path. Returns a 422 message.
func validateEscalationChannelConfig(tx *sql.Tx, projectId uuid.UUID, config json.RawMessage) (string, error) {
	policyId := oncall.EscalationChannelPolicyId(config)
	if policyId == 0 {
		return "An escalation policy is required.", nil
	}
	policy, err := transactional.EscalationPolicyRepository.FindById(tx, policyId)
	if err != nil {
		return "", err
	}
	project, err := transactional.ProjectRepository.FindById(tx, projectId)
	if err != nil {
		return "", err
	}
	if policy == nil || project == nil || project.OrganizationId == nil || *project.OrganizationId != policy.OrganizationId {
		return "Escalation policy not found in this project's organization.", nil
	}
	return "", nil
}

// validateChannelConfig routes to the right validation for the channel type.
// Returns a 422 message, or an empty string when the config is valid.
func validateChannelConfig(tx *sql.Tx, projectId uuid.UUID, channelType string, config json.RawMessage) (string, error) {
	if channelType == "escalation" {
		return validateEscalationChannelConfig(tx, projectId, config)
	}
	if channelType == notifications.AgentChannelType {
		return validateAgentChannelConfig(tx, projectId, config)
	}
	adapter, err := notifications.NewAdapter(channelType, config)
	if err != nil {
		return err.Error(), nil
	}
	if err := adapter.Validate(); err != nil {
		return err.Error(), nil
	}
	if bound, ok := adapter.(notifications.IntegrationBound); ok && bound.IntegrationId() != 0 {
		return validateChannelIntegration(tx, projectId, bound.IntegrationId())
	}
	return "", nil
}

// validateAgentChannelConfig checks the approval mode and that the profile,
// when one is named, belongs to the project's organization.
func validateAgentChannelConfig(tx *sql.Tx, projectId uuid.UUID, config json.RawMessage) (string, error) {
	cfg, problem := notifications.ParseAgentChannelConfig(config)
	if problem != "" {
		return problem, nil
	}
	if cfg.ProfileId == nil {
		return "", nil
	}
	project, err := transactional.ProjectRepository.FindById(tx, projectId)
	if err != nil {
		return "", err
	}
	if project == nil || project.OrganizationId == nil {
		return "The project belongs to no organization.", nil
	}
	profile, err := transactional.AgentProfileRepository.FindById(tx, *cfg.ProfileId)
	if err != nil {
		return "", err
	}
	if profile == nil || profile.OrganizationId != *project.OrganizationId {
		return "Pick an agent profile of this organization.", nil
	}
	return "", nil
}

// validateChannelIntegration checks that an integration-bound channel
// points at an enabled integration of the project's own organization.
func validateChannelIntegration(tx *sql.Tx, projectId uuid.UUID, integrationId int) (string, error) {
	project, err := transactional.ProjectRepository.FindById(tx, projectId)
	if err != nil {
		return "", err
	}
	if project == nil || project.OrganizationId == nil {
		return "The project belongs to no organization.", nil
	}
	integration, err := transactional.IntegrationRepository.FindById(tx, integrationId)
	if err != nil {
		return "", err
	}
	if integration == nil || integration.OrganizationId != *project.OrganizationId {
		return "Pick an integration of this organization.", nil
	}
	if !integration.Enabled {
		return "That integration is disabled.", nil
	}
	return "", nil
}

func (ctrl *notificationChannelController) Create(ctx *gin.Context) {
	projectId, err := middleware.GetProjectId(ctx)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}

	var req createChannelRequest
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
	if !validChannelType(req.ChannelType) {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": channelTypeError()})
		return
	}

	if message, err := validateChannelConfig(db.GetTx(ctx), projectId, req.ChannelType, req.Config); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to validate channel config: %w", err))
		return
	} else if message != "" {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": message})
		return
	}

	userId := middleware.GetUserId(ctx)
	var createdBy *int
	if userId > 0 {
		createdBy = &userId
	}

	config, err := notifications.EncryptSecretFields(req.ChannelType, req.Config)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to encrypt channel config: %w", err))
		return
	}

	tx := db.GetTx(ctx)
	now := time.Now().UTC()
	channel := &models.NotificationChannel{
		ProjectId:   projectId,
		Name:        req.Name,
		ChannelType: req.ChannelType,
		Config:      config,
		Enabled:     true,
		CreatedBy:   createdBy,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	id, err := transactional.NotificationChannelRepository.Create(tx, channel)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to create notification channel: %w", err))
		return
	}
	channel.Id = id

	response, err := maskChannel(channel)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to mask notification channel: %w", err))
		return
	}
	ctx.JSON(http.StatusCreated, response)
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
	if !validChannelType(req.ChannelType) {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": channelTypeError()})
		return
	}

	tx := db.GetTx(ctx)
	existing, err := transactional.NotificationChannelRepository.FindById(tx, id)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to find notification channel: %w", err))
		return
	}
	if existing == nil || existing.ProjectId != projectId {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "channel not found"})
		return
	}

	// The form sends the sentinel for credentials it never saw; only the same
	// channel type's stored values can stand in for them.
	var stored json.RawMessage
	if existing.ChannelType == req.ChannelType {
		stored = existing.Config
	}
	config, err := notifications.KeepStoredSecrets(req.ChannelType, req.Config, stored)
	if err != nil {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Invalid channel configuration."})
		return
	}

	if message, err := validateChannelConfig(tx, projectId, req.ChannelType, config); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to validate channel config: %w", err))
		return
	} else if message != "" {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": message})
		return
	}

	config, err = notifications.EncryptSecretFields(req.ChannelType, config)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to encrypt channel config: %w", err))
		return
	}

	existing.Name = req.Name
	existing.ChannelType = req.ChannelType
	existing.Config = config
	existing.UpdatedAt = time.Now().UTC()

	if err := transactional.NotificationChannelRepository.Update(tx, existing); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to update notification channel: %w", err))
		return
	}

	response, err := maskChannel(existing)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to mask notification channel: %w", err))
		return
	}
	ctx.JSON(http.StatusOK, response)
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

	tx := db.GetTx(ctx)
	existing, err := transactional.NotificationChannelRepository.FindById(tx, id)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to delete notification channel: %w", err))
		return
	}
	if existing == nil || existing.ProjectId != projectId {
		ctx.JSON(http.StatusOK, gin.H{"deleted": true})
		return
	}

	if err := transactional.NotificationChannelRepository.Delete(tx, id); err != nil {
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
		return transactional.NotificationChannelRepository.FindById(tx, id)
	})
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to find notification channel: %w", err))
		return
	}
	if channel == nil || channel.ProjectId != projectId {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "channel not found"})
		return
	}

	// Testing an escalation channel opens a real page so the whole loop is
	// exercised; the dialog warns that it pages the on-call responder.
	if channel.ChannelType == notifications.AgentChannelType {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Fix agent channels start attempts from New Issue and Error Regression rules; there is nothing to send. Use Fix it on an issue to try the agent."})
		return
	}
	if channel.ChannelType == "escalation" {
		policyId := oncall.EscalationChannelPolicyId(json.RawMessage(channel.Config))
		if policyId == 0 {
			ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "An escalation policy is required."})
			return
		}
		opened, err := oncall.OpenTestPage(policyId, projectId, channel.Id, channel.Name)
		if err != nil {
			ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("test page failed: %w", err))
			return
		}
		if !opened {
			ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "A test page for this channel is still open. Resolve it before testing again."})
			return
		}
		ctx.JSON(http.StatusOK, gin.H{"success": true})
		return
	}

	adapter, err := notifications.NewAdapter(channel.ChannelType, channel.Config)
	if err != nil {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	testMsg := notifications.TestChannelMessage()

	if err := adapter.Send(ctx.Request.Context(), testMsg); err != nil {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Test notification failed: " + err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"success": true})
}

var NotificationChannelController = notificationChannelController{}
