package controllers

import (
	"database/sql"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/middleware"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
	"github.com/tracewayapp/traceway/backend/app/secrets"
	traceway "go.tracewayapp.com"
)

type agentProfileController struct{}

// validAgents are the coding agents the harness can drive. Stage 1 ships
// claude-code; the others land with their adapters and are refused until
// then so a profile never names an agent nothing can run.
var validAgents = map[string]bool{"claude-code": true}

const (
	maxProfileTurns   = 500
	maxProfileTimeout = 24 * 60
)

// profileResponse is a profile as the settings page sees it: the credential
// never leaves the server, only whether one is set.
type profileResponse struct {
	*models.AgentProfile
	HasCredential bool `json:"hasCredential"`
}

func profileView(profile *models.AgentProfile) profileResponse {
	return profileResponse{AgentProfile: profile, HasCredential: profile.Credential != ""}
}

func profileViews(profiles []*models.AgentProfile) []profileResponse {
	out := make([]profileResponse, 0, len(profiles))
	for _, profile := range profiles {
		out = append(out, profileView(profile))
	}
	return out
}

// ListForProject serves the preflight's profile picker: the profiles of the
// project's organization, readable by every member.
func (ctrl *agentProfileController) ListForProject(ctx *gin.Context) {
	projectId, err := middleware.GetProjectId(ctx)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}
	tx := db.GetTx(ctx)
	project, err := transactional.ProjectRepository.FindById(tx, projectId)
	if err != nil || project == nil || project.OrganizationId == nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to load project: %w", err))
		return
	}
	profiles, err := transactional.AgentProfileRepository.FindByOrganization(tx, *project.OrganizationId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to list agent profiles: %w", err))
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"profiles": profileViews(profiles)})
}

func (ctrl *agentProfileController) List(ctx *gin.Context) {
	profiles, err := transactional.AgentProfileRepository.FindByOrganization(db.GetTx(ctx), middleware.GetOrganizationId(ctx))
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to list agent profiles: %w", err))
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"profiles": profileViews(profiles)})
}

type agentProfileRequest struct {
	Name           string   `json:"name"`
	Agent          string   `json:"agent"`
	Model          string   `json:"model"`
	Provider       string   `json:"provider"`
	BaseURL        string   `json:"baseUrl"`
	Credential     string   `json:"credential"`
	MaxTurns       int      `json:"maxTurns"`
	TimeoutMinutes int      `json:"timeoutMinutes"`
	BudgetUSD      float64  `json:"budgetUsd"`
	AllowedTools   []string `json:"allowedTools"`
	IsDefault      bool     `json:"isDefault"`
}

// validate trims and checks the request, returning the 422 message when it
// is not acceptable.
func (r *agentProfileRequest) validate() string {
	r.Name = strings.TrimSpace(r.Name)
	r.Agent = strings.TrimSpace(r.Agent)
	r.Model = strings.TrimSpace(r.Model)
	r.Provider = strings.TrimSpace(r.Provider)
	r.BaseURL = strings.TrimSpace(r.BaseURL)
	switch {
	case r.Name == "":
		return "Name is required."
	case len(r.Name) > 200:
		return "Name must be 200 characters or fewer."
	case !validAgents[r.Agent]:
		return "Agent must be one of: claude-code."
	case len(r.Model) > 200 || len(r.Provider) > 50 || len(r.BaseURL) > 500:
		return "Model, provider or base URL is too long."
	case r.BaseURL != "" && !strings.HasPrefix(r.BaseURL, "https://") && !strings.HasPrefix(r.BaseURL, "http://"):
		return "Base URL must start with http:// or https://."
	case r.MaxTurns < 0 || r.MaxTurns > maxProfileTurns:
		return "Max turns must be between 0 and 500."
	case r.TimeoutMinutes < 0 || r.TimeoutMinutes > maxProfileTimeout:
		return "Timeout must be between 0 and 1440 minutes."
	case r.BudgetUSD < 0:
		return "Budget cannot be negative."
	case len(r.AllowedTools) > 100:
		return "At most 100 allowed tools."
	}
	return ""
}

func (ctrl *agentProfileController) Create(ctx *gin.Context) {
	var req agentProfileRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		middleware.RejectBindError(ctx, err, "Invalid request body")
		return
	}
	if message := req.validate(); message != "" {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": message})
		return
	}
	credential, err := encryptCredential(req.Credential, "")
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to encrypt credential: %w", err))
		return
	}
	tx := db.GetTx(ctx)
	organizationId := middleware.GetOrganizationId(ctx)
	now := time.Now().UTC()
	profile := &models.AgentProfile{
		OrganizationId: organizationId,
		Name:           req.Name,
		Agent:          req.Agent,
		Model:          req.Model,
		Provider:       req.Provider,
		BaseURL:        req.BaseURL,
		Credential:     credential,
		MaxTurns:       req.MaxTurns,
		TimeoutMinutes: req.TimeoutMinutes,
		BudgetUSD:      req.BudgetUSD,
		AllowedTools:   models.StringSlice(req.AllowedTools),
		NetworkPolicy:  models.JSONText(`{}`),
		IsDefault:      req.IsDefault,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if profile.AllowedTools == nil {
		profile.AllowedTools = models.StringSlice{}
	}
	id, err := transactional.AgentProfileRepository.Create(tx, profile)
	if err != nil {
		if isUniqueViolation(err) {
			ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "A profile with this name already exists."})
			return
		}
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to create agent profile: %w", err))
		return
	}
	profile.Id = id
	if err := ctrl.applyDefault(tx, profile, req.IsDefault, now); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to set default profile: %w", err))
		return
	}
	ctx.JSON(http.StatusCreated, gin.H{"profile": profileView(profile)})
}

// applyDefault makes a profile the default when asked, or makes it the
// default anyway when the organization has none yet, so the first profile
// is usable from the preflight without a second step.
func (ctrl *agentProfileController) applyDefault(tx *sql.Tx, profile *models.AgentProfile, wantDefault bool, now time.Time) error {
	if !wantDefault {
		current, err := transactional.AgentProfileRepository.FindDefault(tx, profile.OrganizationId)
		if err != nil {
			return err
		}
		if current != nil {
			return nil
		}
	}
	profile.IsDefault = true
	return transactional.AgentProfileRepository.SetDefault(tx, profile.OrganizationId, profile.Id, now)
}

func (ctrl *agentProfileController) loadOrgProfile(ctx *gin.Context) (*models.AgentProfile, bool) {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid profile id"})
		return nil, false
	}
	profile, err := transactional.AgentProfileRepository.FindById(db.GetTx(ctx), id)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to load agent profile: %w", err))
		return nil, false
	}
	if profile == nil || profile.OrganizationId != middleware.GetOrganizationId(ctx) {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "profile not found"})
		return nil, false
	}
	return profile, true
}

func (ctrl *agentProfileController) Update(ctx *gin.Context) {
	profile, ok := ctrl.loadOrgProfile(ctx)
	if !ok {
		return
	}
	var req agentProfileRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		middleware.RejectBindError(ctx, err, "Invalid request body")
		return
	}
	if message := req.validate(); message != "" {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": message})
		return
	}
	credential, err := encryptCredential(req.Credential, profile.Credential)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to encrypt credential: %w", err))
		return
	}
	now := time.Now().UTC()
	profile.Name = req.Name
	profile.Agent = req.Agent
	profile.Model = req.Model
	profile.Provider = req.Provider
	profile.BaseURL = req.BaseURL
	profile.Credential = credential
	profile.MaxTurns = req.MaxTurns
	profile.TimeoutMinutes = req.TimeoutMinutes
	profile.BudgetUSD = req.BudgetUSD
	profile.AllowedTools = models.StringSlice(req.AllowedTools)
	if profile.AllowedTools == nil {
		profile.AllowedTools = models.StringSlice{}
	}
	profile.UpdatedAt = now
	tx := db.GetTx(ctx)
	if err := transactional.AgentProfileRepository.Update(tx, profile); err != nil {
		if isUniqueViolation(err) {
			ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "A profile with this name already exists."})
			return
		}
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to update agent profile: %w", err))
		return
	}
	if req.IsDefault && !profile.IsDefault {
		if err := ctrl.applyDefault(tx, profile, true, now); err != nil {
			ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to set default profile: %w", err))
			return
		}
	}
	ctx.JSON(http.StatusOK, gin.H{"profile": profileView(profile)})
}

func (ctrl *agentProfileController) SetDefault(ctx *gin.Context) {
	profile, ok := ctrl.loadOrgProfile(ctx)
	if !ok {
		return
	}
	if err := transactional.AgentProfileRepository.SetDefault(db.GetTx(ctx), profile.OrganizationId, profile.Id, time.Now().UTC()); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to set default profile: %w", err))
		return
	}
	profile.IsDefault = true
	ctx.JSON(http.StatusOK, gin.H{"profile": profileView(profile)})
}

func (ctrl *agentProfileController) Delete(ctx *gin.Context) {
	profile, ok := ctrl.loadOrgProfile(ctx)
	if !ok {
		return
	}
	if err := transactional.AgentProfileRepository.Delete(db.GetTx(ctx), profile.Id); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to delete agent profile: %w", err))
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"deleted": true})
}

// encryptCredential stores a typed credential encrypted; the sentinel or an
// empty value keeps what is stored, so an edit never has to retype the key.
func encryptCredential(typed string, stored string) (string, error) {
	typed = strings.TrimSpace(typed)
	if typed == "" || typed == secrets.Sentinel {
		return stored, nil
	}
	return secrets.Encrypt([]byte(typed))
}

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "unique") || strings.Contains(message, "duplicate key")
}

var AgentProfileController = agentProfileController{}
