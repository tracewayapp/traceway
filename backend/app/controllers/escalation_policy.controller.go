package controllers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/middleware"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/oncall"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"

	"github.com/gin-gonic/gin"
	traceway "go.tracewayapp.com"
)

type escalationPolicyController struct{}

var EscalationPolicyController = escalationPolicyController{}

type escalationPolicyRequest struct {
	Name       string          `json:"name"`
	Definition json.RawMessage `json:"definition"`
}

// ListForProject serves the channel-dialog picker and the on-call page: all
// policies of the project's organization, readable by any project member.
func (c *escalationPolicyController) ListForProject(ctx *gin.Context) {
	tx := db.GetTx(ctx)
	projectId, err := middleware.GetProjectId(ctx)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}
	project, err := transactional.ProjectRepository.FindById(tx, projectId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to load project: %w", err))
		return
	}
	if project == nil || project.OrganizationId == nil {
		ctx.JSON(http.StatusOK, gin.H{"policies": []*models.EscalationPolicy{}})
		return
	}
	policies, err := transactional.EscalationPolicyRepository.FindByOrganization(tx, *project.OrganizationId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to list escalation policies: %w", err))
		return
	}
	if policies == nil {
		policies = []*models.EscalationPolicy{}
	}
	ctx.JSON(http.StatusOK, gin.H{"policies": policies})
}

func (c *escalationPolicyController) ListForOrganization(ctx *gin.Context) {
	tx := db.GetTx(ctx)
	organizationId := middleware.GetOrganizationId(ctx)
	policies, err := transactional.EscalationPolicyRepository.FindByOrganization(tx, organizationId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to list escalation policies: %w", err))
		return
	}
	if policies == nil {
		policies = []*models.EscalationPolicy{}
	}
	ctx.JSON(http.StatusOK, gin.H{"policies": policies})
}

func (c *escalationPolicyController) Create(ctx *gin.Context) {
	tx := db.GetTx(ctx)
	organizationId := middleware.GetOrganizationId(ctx)
	userId := middleware.GetUserId(ctx)

	var request escalationPolicyRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	request.Name = strings.TrimSpace(request.Name)
	if message := validatePolicyName(request.Name); message != "" {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": message})
		return
	}
	existing, err := transactional.EscalationPolicyRepository.FindByOrganizationAndName(tx, organizationId, request.Name)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to check policy name: %w", err))
		return
	}
	if existing != nil {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "An escalation policy with this name already exists."})
		return
	}
	definition, err := oncall.ValidatePolicyDefinition(tx, organizationId, request.Definition)
	if err != nil {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	normalized, err := oncall.MarshalPolicyDefinition(definition)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to marshal policy definition: %w", err))
		return
	}

	now := time.Now().UTC()
	policy := &models.EscalationPolicy{
		OrganizationId: organizationId,
		Name:           request.Name,
		Definition:     models.JSONText(normalized),
		CreatedBy:      &userId,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	id, err := transactional.EscalationPolicyRepository.Create(tx, policy)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to create escalation policy: %w", err))
		return
	}
	policy.Id = id
	ctx.JSON(http.StatusCreated, policy)
}

func (c *escalationPolicyController) Update(ctx *gin.Context) {
	tx := db.GetTx(ctx)
	organizationId := middleware.GetOrganizationId(ctx)

	policy, ok := c.loadPolicy(ctx, organizationId)
	if !ok {
		return
	}
	var request escalationPolicyRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	request.Name = strings.TrimSpace(request.Name)
	if message := validatePolicyName(request.Name); message != "" {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": message})
		return
	}
	existing, err := transactional.EscalationPolicyRepository.FindByOrganizationAndName(tx, organizationId, request.Name)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to check policy name: %w", err))
		return
	}
	if existing != nil && existing.Id != policy.Id {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "An escalation policy with this name already exists."})
		return
	}
	definition, err := oncall.ValidatePolicyDefinition(tx, organizationId, request.Definition)
	if err != nil {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	normalized, err := oncall.MarshalPolicyDefinition(definition)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to marshal policy definition: %w", err))
		return
	}

	policy.Name = request.Name
	policy.Definition = models.JSONText(normalized)
	policy.UpdatedAt = time.Now().UTC()
	if err := transactional.EscalationPolicyRepository.Update(tx, policy); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to update escalation policy: %w", err))
		return
	}
	ctx.JSON(http.StatusOK, policy)
}

func (c *escalationPolicyController) Delete(ctx *gin.Context) {
	tx := db.GetTx(ctx)
	organizationId := middleware.GetOrganizationId(ctx)

	policy, ok := c.loadPolicy(ctx, organizationId)
	if !ok {
		return
	}

	// Block deletion while an escalation channel references the policy, so a
	// rule cannot silently start failing to page.
	referencing, err := c.findReferencingChannels(tx, organizationId, policy.Id)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to check referencing channels: %w", err))
		return
	}
	if len(referencing) > 0 {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "This policy is used by notification channel(s): " + strings.Join(referencing, ", ") + ". Remove those channels first."})
		return
	}

	if err := transactional.EscalationPolicyRepository.Delete(tx, policy.Id); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to delete escalation policy: %w", err))
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"message": "Escalation policy deleted"})
}

func (c *escalationPolicyController) findReferencingChannels(tx *sql.Tx, organizationId int, policyId int) ([]string, error) {
	channels, err := transactional.NotificationChannelRepository.FindEscalationByOrganization(tx, organizationId)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, channel := range channels {
		if oncall.EscalationChannelPolicyId(json.RawMessage(channel.Config)) == policyId {
			names = append(names, channel.Name)
		}
	}
	return names, nil
}

func (c *escalationPolicyController) loadPolicy(ctx *gin.Context, organizationId int) (*models.EscalationPolicy, bool) {
	tx := db.GetTx(ctx)
	policyId, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid policy ID"})
		return nil, false
	}
	policy, err := transactional.EscalationPolicyRepository.FindById(tx, policyId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to load escalation policy: %w", err))
		return nil, false
	}
	if policy == nil || policy.OrganizationId != organizationId {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Escalation policy not found"})
		return nil, false
	}
	return policy, true
}

func validatePolicyName(name string) string {
	if name == "" {
		return "A policy name is required."
	}
	if len(name) > 100 {
		return "The policy name can be at most 100 characters."
	}
	return ""
}
