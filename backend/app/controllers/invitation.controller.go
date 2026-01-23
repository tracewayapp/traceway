package controllers

import (
	"backend/app/middleware"
	"backend/app/models"
	"backend/app/pgdb"
	"backend/app/repositories"
	"backend/app/services"
	"database/sql"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	traceway "go.tracewayapp.com"
)

type invitationController struct{}

const maxMembersPerOrg = 10
const invitationExpiryDays = 7

func (c *invitationController) InviteUser(ctx *gin.Context) {
	tx := middleware.GetTx(ctx)
	organizationId := middleware.GetOrganizationId(ctx)
	userId := middleware.GetUserId(ctx)

	var request models.InviteUserRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	isMember, err := repositories.OrganizationRepository.IsUserMemberByEmail(tx, organizationId, request.Email)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("Failed to check membership: %w", err))
		return
	}
	if isMember {
		ctx.JSON(http.StatusConflict, gin.H{"error": "User is already a member of this organization"})
		return
	}

	hasPending, err := repositories.InvitationRepository.HasPendingInvitation(tx, request.Email, organizationId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("Failed to check pending invitations: %w", err))
		return
	}
	if hasPending {
		ctx.JSON(http.StatusConflict, gin.H{"error": "User already has a pending invitation"})
		return
	}

	memberCount, err := repositories.OrganizationRepository.CountMembers(tx, organizationId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("Failed to count members: %w", err))
		return
	}

	invitations, err := repositories.InvitationRepository.FindByOrganization(tx, organizationId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("Failed to count invitations: %w", err))
		return
	}

	totalCount := memberCount + len(invitations)
	if totalCount >= maxMembersPerOrg {
		ctx.JSON(http.StatusConflict, gin.H{"error": "Organization has reached the maximum number of members"})
		return
	}

	inviter, err := repositories.UserRepository.FindById(tx, userId)
	if err != nil || inviter == nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("Failed to get inviter info: %w", err))
		return
	}

	org, err := repositories.OrganizationRepository.FindById(tx, organizationId)
	if err != nil || org == nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("Failed to get organization info: %w", err))
		return
	}

	expiresAt := time.Now().AddDate(0, 0, invitationExpiryDays)
	invitation, err := repositories.InvitationRepository.Create(tx, organizationId, request.Email, request.Role, userId, expiresAt)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("Failed to create invitation: %w", err))
		return
	}

	go services.EmailService.SendInvitation(request.Email, inviter.Name, org.Name, invitation.Token)

	ctx.JSON(http.StatusCreated, gin.H{"message": "Invitation sent"})
}

func (c *invitationController) ListInvitations(ctx *gin.Context) {
	organizationId := middleware.GetOrganizationId(ctx)

	invitations, err := pgdb.ExecuteTransaction(func(tx *sql.Tx) ([]*models.InvitationWithInviter, error) {
		return repositories.InvitationRepository.FindByOrganization(tx, organizationId)
	})

	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("Failed to load invitations: %w", err))
		return
	}

	ctx.JSON(http.StatusOK, invitations)
}

func (c *invitationController) RevokeInvitation(ctx *gin.Context) {
	tx := middleware.GetTx(ctx)
	organizationId := middleware.GetOrganizationId(ctx)

	invitationId, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid invitation ID"})
		return
	}

	invitation, err := repositories.InvitationRepository.FindById(tx, invitationId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("Failed to find invitation: %w", err))
		return
	}
	if invitation == nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Invitation not found"})
		return
	}
	if invitation.OrganizationId != organizationId {
		ctx.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	err = repositories.InvitationRepository.Delete(tx, invitationId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("Failed to revoke invitation: %w", err))
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "Invitation revoked"})
}

func (c *invitationController) GetInvitationInfo(ctx *gin.Context) {
	token := ctx.Param("token")

	type invitationInfo struct {
		Invitation *models.Invitation
		Org        *models.Organization
		Inviter    *models.User
		UserExists bool
	}

	data, err := pgdb.ExecuteTransaction(func(tx *sql.Tx) (*invitationInfo, error) {
		invitation, err := repositories.InvitationRepository.FindByToken(tx, token)
		if err != nil {
			return nil, err
		}
		if invitation == nil {
			return nil, nil
		}

		org, err := repositories.OrganizationRepository.FindById(tx, invitation.OrganizationId)
		if err != nil {
			return nil, err
		}

		inviter, err := repositories.UserRepository.FindById(tx, invitation.InvitedBy)
		if err != nil {
			return nil, err
		}

		userExists, err := repositories.UserRepository.EmailExists(tx, invitation.Email)
		if err != nil {
			return nil, err
		}

		return &invitationInfo{
			Invitation: invitation,
			Org:        org,
			Inviter:    inviter,
			UserExists: userExists,
		}, nil
	})

	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("Failed to load invitation: %w", err))
		return
	}

	if data == nil || data.Invitation == nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Invitation not found"})
		return
	}

	if data.Invitation.Status != "pending" {
		ctx.JSON(http.StatusGone, gin.H{"error": "Invitation has already been used or expired"})
		return
	}

	if time.Now().After(data.Invitation.ExpiresAt) {
		ctx.JSON(http.StatusGone, gin.H{"error": "Invitation has expired"})
		return
	}

	ctx.JSON(http.StatusOK, &models.InvitationInfoResponse{
		Email:            data.Invitation.Email,
		OrganizationName: data.Org.Name,
		InviterName:      data.Inviter.Name,
		ExistsAsUser:     data.UserExists,
		Role:             data.Invitation.Role,
	})
}

func (c *invitationController) AcceptInvitation(ctx *gin.Context) {
	tx := middleware.GetTx(ctx)
	token := ctx.Param("token")

	var request models.AcceptInvitationRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	invitation, err := repositories.InvitationRepository.FindByToken(tx, token)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("Failed to load invitation: %w", err))
		return
	}
	if invitation == nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Invitation not found"})
		return
	}

	if invitation.Status != "pending" {
		ctx.JSON(http.StatusGone, gin.H{"error": "Invitation has already been used or expired"})
		return
	}

	if time.Now().After(invitation.ExpiresAt) {
		ctx.JSON(http.StatusGone, gin.H{"error": "Invitation has expired"})
		return
	}

	existingUser, err := repositories.UserRepository.FindByEmail(tx, invitation.Email)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("Failed to check user: %w", err))
		return
	}
	if existingUser != nil {
		ctx.JSON(http.StatusConflict, gin.H{"error": "User already exists. Please log in and accept the invitation."})
		return
	}

	hashedPassword, err := services.HashPassword(request.Password)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("Failed to process password: %w", err))
		return
	}

	user, err := repositories.UserRepository.Create(tx, invitation.Email, request.Name, hashedPassword)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("Failed to create user: %w", err))
		return
	}

	_, err = repositories.OrganizationRepository.AddUser(tx, invitation.OrganizationId, user.Id, invitation.Role)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("Failed to add user to organization: %w", err))
		return
	}

	now := time.Now()
	invitation.AcceptedAt = &now
	invitation.Status = "accepted"

	err = repositories.InvitationRepository.Update(tx, invitation)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("Failed to update invitation: %w", err))
		return
	}

	jwtToken, err := services.GenerateToken(user.Id, user.Email)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("Failed to generate token: %w", err))
		return
	}

	projects, err := repositories.ProjectRepository.FindAllWithBackendUrlByUserId(tx, user.Id)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("Failed to load projects: %w", err))
		return
	}

	ctx.JSON(http.StatusCreated, &models.LoginResponse{
		Token:    jwtToken,
		User:     user.ToResponse(),
		Projects: projects,
	})
}

func (c *invitationController) AcceptExistingUser(ctx *gin.Context) {
	tx := middleware.GetTx(ctx)
	token := ctx.Param("token")
	userId := middleware.GetUserId(ctx)
	userEmail := middleware.GetUserEmail(ctx)

	invitation, err := repositories.InvitationRepository.FindByToken(tx, token)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("Failed to load invitation: %w", err))
		return
	}
	if invitation == nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Invitation not found"})
		return
	}

	if invitation.Status != "pending" {
		ctx.JSON(http.StatusGone, gin.H{"error": "Invitation has already been used or expired"})
		return
	}

	if time.Now().After(invitation.ExpiresAt) {
		ctx.JSON(http.StatusGone, gin.H{"error": "Invitation has expired"})
		return
	}

	if userEmail != invitation.Email {
		ctx.JSON(http.StatusForbidden, gin.H{"error": "This invitation was sent to a different email address. Please log in before trying to Accept the invitation."})
		return
	}

	isMember, err := repositories.OrganizationRepository.IsUserMember(tx, invitation.OrganizationId, userId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("Failed to check membership: %w", err))
		return
	}
	if isMember {
		ctx.JSON(http.StatusConflict, gin.H{"error": "You are already a member of this organization"})
		return
	}

	_, err = repositories.OrganizationRepository.AddUser(tx, invitation.OrganizationId, userId, invitation.Role)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("Failed to add you to organization: %w", err))
		return
	}

	now := time.Now()
	invitation.AcceptedAt = &now
	invitation.Status = "accepted"

	err = repositories.InvitationRepository.Update(tx, invitation)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("Failed to update invitation: %w", err))
		return
	}

	projects, err := repositories.ProjectRepository.FindAllWithBackendUrlByUserId(tx, userId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("Failed to load projects: %w", err))
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"message":  "You have joined the organization",
		"projects": projects,
	})
}

var InvitationController = invitationController{}
