package controllers

import (
	"database/sql"
	"errors"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/middleware"
	"github.com/tracewayapp/traceway/backend/app/models"
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
	// zone here would surface much later as wrong shift boundaries.
	if _, err := time.LoadLocation(timezone); err != nil {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Unknown timezone"})
		return
	}

	userId := middleware.GetUserId(ctx)
	tx := db.GetTx(ctx)

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

	org, err := transactional.OrganizationRepository.Create(tx, name, timezone)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to create organization: %w", err))
		return
	}

	if _, err := transactional.OrganizationRepository.AddUser(tx, org.Id, userId, "owner"); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to add creator to organization: %w", err))
		return
	}

	ctx.JSON(http.StatusCreated, models.UserOrganizationResponse{
		Id:       org.Id,
		Name:     org.Name,
		Role:     "owner",
		Timezone: org.Timezone,
	})
}
