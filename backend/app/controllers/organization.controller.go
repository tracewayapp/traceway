package controllers

import (
	"database/sql"
	"errors"
	"github.com/tracewayapp/traceway/backend/app/config"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/middleware"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/oncall"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	traceway "go.tracewayapp.com"
)

type organizationController struct{}

func (c *organizationController) GetSettings(ctx *gin.Context) {
	organizationId := middleware.GetOrganizationId(ctx)
	userRole := middleware.GetUserOrgRole(ctx)

	type settingsData struct {
		Organization *models.Organization
		Members      []*models.OrganizationMember
		Invitations  []*models.InvitationWithInviter
	}

	data, err := db.ExecuteTransaction(func(tx *sql.Tx) (*settingsData, error) {
		org, err := transactional.OrganizationRepository.FindById(tx, organizationId)
		if err != nil {
			return nil, err
		}

		members, err := transactional.OrganizationRepository.GetMembersWithDetails(tx, organizationId)
		if err != nil {
			return nil, err
		}

		invitations, err := transactional.InvitationRepository.FindByOrganization(tx, organizationId)
		if err != nil {
			return nil, err
		}

		return &settingsData{
			Organization: org,
			Members:      members,
			Invitations:  invitations,
		}, nil
	})

	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("Failed to load settings: %w", err))
		return
	}

	if data.Organization == nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Organization not found"})
		return
	}

	ctx.JSON(http.StatusOK, &models.OrganizationSettingsResponse{
		Organization: data.Organization,
		Members:      data.Members,
		Invitations:  data.Invitations,
		UserRole:     userRole,
	})
}

func (c *organizationController) GetMembers(ctx *gin.Context) {
	organizationId := middleware.GetOrganizationId(ctx)

	members, err := db.ExecuteTransaction(func(tx *sql.Tx) ([]*models.OrganizationMember, error) {
		return transactional.OrganizationRepository.GetMembersWithDetails(tx, organizationId)
	})

	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("Failed to load members: %w", err))
		return
	}

	ctx.JSON(http.StatusOK, members)
}

type UpdateSettingsRequest struct {
	Timezone string `json:"timezone" binding:"required"`
}

func (c *organizationController) UpdateSettings(ctx *gin.Context) {
	organizationId := middleware.GetOrganizationId(ctx)
	tx := db.GetTx(ctx)

	var req UpdateSettingsRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	_, err := time.LoadLocation(req.Timezone)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid timezone"})
		return
	}

	err = transactional.OrganizationRepository.UpdateTimezone(tx, organizationId, req.Timezone)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("Failed to update settings: %w", err))
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"timezone": req.Timezone})
}

var OrganizationController = organizationController{}

type CreateOrganizationRequest struct {
	Name     string `json:"name" binding:"required"`
	Timezone string `json:"timezone"`
}

func validateOrganizationName(name string) string {
	nameLen := utf8.RuneCountInString(name)
	if nameLen < 1 || nameLen > 100 {
		return "Organization name must be between 1 and 100 characters"
	}
	return ""
}

// Create lets an authenticated user start a new organization they own. Every
// other org-creating path (register, SSO finish-setup) is an account-creation
// path, so a user removed from their last organization otherwise has no way
// back into a usable account.
func (c *organizationController) Create(ctx *gin.Context) {
	var request CreateOrganizationRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Organization name is required"})
		return
	}

	name := strings.TrimSpace(request.Name)
	if message := validateOrganizationName(name); message != "" {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": message})
		return
	}

	// Unlike register, which takes the timezone from a required form field, this
	// endpoint is reached from a recovery screen with nothing to prefill from.
	timezone := strings.TrimSpace(request.Timezone)
	if timezone == "" {
		timezone = "UTC"
	}
	// On-call schedule resolution is tz-aware calendar math, so an unparseable
	// zone would surface much later as wrong shift boundaries.
	if _, err := oncall.LoadTimezone(timezone); err != nil {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	userId := middleware.GetUserId(ctx)
	tx := db.GetTx(ctx)

	// Self-hosted instances allow exactly one organization; Register enforces the
	// same rule (as a 409). This answers 422 instead because the message has to
	// reach the recovery form, and api.ts only extracts bodies from 401/403/422 --
	// a 409 would surface to the user as "API Error: Conflict".
	// Without this an authenticated user of any role -- readonly
	// included -- could mint an organization here and own it, since the route
	// carries no role guard and OrganizationLimitHook is nil outside cloud.
	if config.Config.CloudMode != "true" {
		hasOrganizations, err := transactional.OrganizationRepository.HasOrganizations(tx)
		if err != nil {
			ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to check for existing organizations: %w", err))
			return
		}
		if hasOrganizations {
			ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "This instance already has an organization. Ask an administrator to invite you to it."})
			return
		}
	}

	if OrganizationLimitHook != nil {
		if err := OrganizationLimitHook(tx, userId); err != nil {
			var limitErr *LimitExceededError
			if errors.As(err, &limitErr) {
				ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": limitErr.Message})
				return
			}
			ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("organization limit hook failed: %w", err))
			return
		}
	}

	user, err := transactional.UserRepository.FindById(tx, userId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to load creating user: %w", err))
		return
	}
	if user == nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("authenticated user %d not found", userId))
		return
	}

	org, err := transactional.OrganizationRepository.Create(tx, name, timezone)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to create organization: %w", err))
		return
	}

	if _, err := transactional.OrganizationRepository.AddUser(tx, org.Id, user.Id, "owner"); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to add creator to organization: %w", err))
		return
	}

	// Register and FinishSetup both run these for every new org+owner pair; it is
	// the cloud build's provisioning seam, so an organization created here must
	// not skip it.
	for _, hook := range PostRegistrationHooks {
		if err := hook(tx, org, user); err != nil {
			ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("post-registration hook failed: %w", err))
			return
		}
	}

	ctx.JSON(http.StatusCreated, models.UserOrganizationResponse{
		Id:       org.Id,
		Name:     org.Name,
		Role:     "owner",
		Timezone: org.Timezone,
	})
}
