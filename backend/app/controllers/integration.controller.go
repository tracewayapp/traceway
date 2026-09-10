package controllers

import (
	"encoding/json"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tracewayapp/traceway/backend/app/agent"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/middleware"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
	"github.com/tracewayapp/traceway/backend/app/secrets"
	traceway "go.tracewayapp.com"
)

type integrationController struct{}

// Providers lists every registered provider with the fields its settings
// form needs; the page renders itself from this.
func (ctrl *integrationController) Providers(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, gin.H{"providers": agent.Providers()})
}

// integrationResponse is an integration as the settings page sees it:
// credentials replaced by the sentinel, with the masked field names.
type integrationResponse struct {
	*models.Integration
	Config     json.RawMessage `json:"config"`
	HasSecrets []string        `json:"hasSecrets"`
}

func maskIntegration(integration *models.Integration) (integrationResponse, error) {
	fields := secretFieldsFor(integration.Provider)
	config, hasSecrets, err := secrets.MaskFields(json.RawMessage(integration.Config), fields)
	if err != nil {
		return integrationResponse{}, err
	}
	return integrationResponse{Integration: integration, Config: config, HasSecrets: hasSecrets}, nil
}

func secretFieldsFor(provider string) []string {
	p, ok := agent.ProviderFor(provider)
	if !ok {
		return nil
	}
	return agent.SecretFields(p)
}

func (ctrl *integrationController) List(ctx *gin.Context) {
	integrations, err := transactional.IntegrationRepository.FindByOrganization(db.GetTx(ctx), middleware.GetOrganizationId(ctx))
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to list integrations: %w", err))
		return
	}
	out := make([]integrationResponse, 0, len(integrations))
	for _, integration := range integrations {
		masked, err := maskIntegration(integration)
		if err != nil {
			ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to mask integration %d: %w", integration.Id, err))
			return
		}
		out = append(out, masked)
	}
	ctx.JSON(http.StatusOK, gin.H{"integrations": out})
}

type integrationRequest struct {
	Provider string            `json:"provider"`
	Name     string            `json:"name"`
	Config   map[string]string `json:"config"`
	Enabled  *bool             `json:"enabled"`
}

// prepareConfig merges the sentinel-carrying request config with the stored
// one, validates the plaintext through the provider, and returns it
// encrypted for storage.
func prepareConfig(provider agent.Provider, incoming map[string]string, stored json.RawMessage) (json.RawMessage, string, error) {
	if incoming == nil {
		incoming = map[string]string{}
	}
	raw, err := json.Marshal(incoming)
	if err != nil {
		return nil, "", err
	}
	fields := agent.SecretFields(provider)
	merged, err := secrets.KeepStoredFields(raw, stored, fields)
	if err != nil {
		return nil, "", err
	}
	plaintext, err := secrets.DecryptFields(merged, fields)
	if err != nil {
		return nil, "", err
	}
	var values map[string]string
	if err := json.Unmarshal(plaintext, &values); err != nil {
		return nil, "", err
	}
	if err := provider.Validate(values); err != nil {
		return nil, err.Error(), nil
	}
	encrypted, err := secrets.EncryptFields(plaintext, fields)
	return encrypted, "", err
}

func (ctrl *integrationController) Create(ctx *gin.Context) {
	var req integrationRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		middleware.RejectBindError(ctx, err, "Invalid request body")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || len(req.Name) > 200 {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Name is required and must be 200 characters or fewer."})
		return
	}
	provider, ok := agent.ProviderFor(req.Provider)
	if !ok || len(provider.Fields()) == 0 {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Unknown provider."})
		return
	}
	config, message, err := prepareConfig(provider, req.Config, nil)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to prepare integration config: %w", err))
		return
	}
	if message != "" {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": message})
		return
	}
	userId := middleware.GetUserId(ctx)
	now := time.Now().UTC()
	integration := &models.Integration{
		OrganizationId: middleware.GetOrganizationId(ctx),
		Provider:       provider.Provider(),
		Kinds:          models.StringSlice(provider.Kinds()),
		Name:           req.Name,
		Config:         models.JSONText(config),
		Enabled:        true,
		CreatedBy:      &userId,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	id, err := transactional.IntegrationRepository.Create(db.GetTx(ctx), integration)
	if err != nil {
		if isUniqueViolation(err) {
			ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "An integration of this provider with this name already exists."})
			return
		}
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to create integration: %w", err))
		return
	}
	integration.Id = id
	masked, err := maskIntegration(integration)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to mask integration: %w", err))
		return
	}
	middleware.OnCommit(ctx, agent.IntegrationsChanged)
	ctx.JSON(http.StatusCreated, gin.H{"integration": masked})
}

func (ctrl *integrationController) loadOrgIntegration(ctx *gin.Context) (*models.Integration, bool) {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid integration id"})
		return nil, false
	}
	integration, err := transactional.IntegrationRepository.FindById(db.GetTx(ctx), id)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to load integration: %w", err))
		return nil, false
	}
	if integration == nil || integration.OrganizationId != middleware.GetOrganizationId(ctx) {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "integration not found"})
		return nil, false
	}
	return integration, true
}

func (ctrl *integrationController) Update(ctx *gin.Context) {
	integration, ok := ctrl.loadOrgIntegration(ctx)
	if !ok {
		return
	}
	var req integrationRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		middleware.RejectBindError(ctx, err, "Invalid request body")
		return
	}
	provider, ok := agent.ProviderFor(integration.Provider)
	if !ok {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "The provider of this integration is no longer registered."})
		return
	}
	if name := strings.TrimSpace(req.Name); name != "" {
		if len(name) > 200 {
			ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Name must be 200 characters or fewer."})
			return
		}
		integration.Name = name
	}
	if req.Config != nil {
		config, message, err := prepareConfig(provider, req.Config, json.RawMessage(integration.Config))
		if err != nil {
			ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to prepare integration config: %w", err))
			return
		}
		if message != "" {
			ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": message})
			return
		}
		integration.Config = models.JSONText(config)
	}
	if req.Enabled != nil {
		integration.Enabled = *req.Enabled
	}
	integration.UpdatedAt = time.Now().UTC()
	if err := transactional.IntegrationRepository.Update(db.GetTx(ctx), integration); err != nil {
		if isUniqueViolation(err) {
			ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "An integration of this provider with this name already exists."})
			return
		}
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to update integration: %w", err))
		return
	}
	masked, err := maskIntegration(integration)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to mask integration: %w", err))
		return
	}
	middleware.OnCommit(ctx, agent.IntegrationsChanged)
	ctx.JSON(http.StatusOK, gin.H{"integration": masked})
}

func (ctrl *integrationController) Delete(ctx *gin.Context) {
	integration, ok := ctrl.loadOrgIntegration(ctx)
	if !ok {
		return
	}
	if err := transactional.IntegrationRepository.Delete(db.GetTx(ctx), integration.Id); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to delete integration: %w", err))
		return
	}
	middleware.OnCommit(ctx, agent.IntegrationsChanged)
	ctx.JSON(http.StatusOK, gin.H{"deleted": true})
}

// projectIntegration is what a project member may see of an organization
// integration: enough to pick it in a channel dialog, no config.
type projectIntegration struct {
	Id       int      `json:"id"`
	Name     string   `json:"name"`
	Provider string   `json:"provider"`
	Kinds    []string `json:"kinds"`
}

// ListForProject lists the enabled integrations of the project's
// organization, optionally of one kind, for members who are not admins.
func (ctrl *integrationController) ListForProject(ctx *gin.Context) {
	projectId, err := middleware.GetProjectId(ctx)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}
	tx := db.GetTx(ctx)
	project, err := transactional.ProjectRepository.FindById(tx, projectId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to load project: %w", err))
		return
	}
	if project == nil || project.OrganizationId == nil {
		ctx.JSON(http.StatusOK, gin.H{"integrations": []projectIntegration{}})
		return
	}
	rows, err := transactional.IntegrationRepository.FindByOrganization(tx, *project.OrganizationId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to list integrations: %w", err))
		return
	}
	kind := ctx.Query("kind")
	out := []projectIntegration{}
	for _, row := range rows {
		if !row.Enabled || (kind != "" && !slices.Contains(row.Kinds, kind)) {
			continue
		}
		out = append(out, projectIntegration{Id: row.Id, Name: row.Name, Provider: row.Provider, Kinds: row.Kinds})
	}
	ctx.JSON(http.StatusOK, gin.H{"integrations": out})
}

// Inbound is the one route every provider's webhooks and callbacks arrive
// on. The provider verifies the signature with the integration's own secret
// inside its Inbound; an unknown or disabled integration is a 404 before
// any body is read. Not under Transactional: the dispatcher opens its own
// short transactions per event.
func (ctrl *integrationController) Inbound(ctx *gin.Context) {
	provider := ctx.Param("provider")
	id, err := strconv.Atoi(ctx.Param("integrationId"))
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	integration, err := agent.LoadIntegration(provider, id)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to load integration: %w", err))
		return
	}
	if integration == nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	result, err := agent.HandleInbound(ctx.Request.Context(), provider, integration, ctx.Request)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "rejected"})
		traceway.CaptureException(traceway.NewStackTraceErrorf("agent inbound %s/%d rejected: %w", provider, id, err))
		return
	}
	if result.Response != nil {
		ctx.Data(http.StatusOK, "application/json", result.Response)
		return
	}
	ctx.JSON(http.StatusOK, result)
}

var IntegrationController = integrationController{}
