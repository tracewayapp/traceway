package controllers

import (
	"backend/app/middleware"
	"backend/app/models"
	"backend/app/pgdb"
	"backend/app/repositories"
	"database/sql"
	"net/http"
	"time"

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

	data, err := pgdb.ExecuteTransaction(func(tx *sql.Tx) (*settingsData, error) {
		org, err := repositories.OrganizationRepository.FindById(tx, organizationId)
		if err != nil {
			return nil, err
		}

		members, err := repositories.OrganizationRepository.GetMembersWithDetails(tx, organizationId)
		if err != nil {
			return nil, err
		}

		invitations, err := repositories.InvitationRepository.FindByOrganization(tx, organizationId)
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

	members, err := pgdb.ExecuteTransaction(func(tx *sql.Tx) ([]*models.OrganizationMember, error) {
		return repositories.OrganizationRepository.GetMembersWithDetails(tx, organizationId)
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
	tx := middleware.GetTx(ctx)

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

	err = repositories.OrganizationRepository.UpdateTimezone(tx, organizationId, req.Timezone)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("Failed to update settings: %w", err))
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"timezone": req.Timezone})
}

var OrganizationController = organizationController{}
