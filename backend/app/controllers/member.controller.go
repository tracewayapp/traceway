package controllers

import (
	"backend/app/middleware"
	"backend/app/models"
	"backend/app/repositories"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	traceway "go.tracewayapp.com"
)

type memberController struct{}

func (c *memberController) UpdateRole(ctx *gin.Context) {
	tx := middleware.GetTx(ctx)
	organizationId := middleware.GetOrganizationId(ctx)
	currentUserId := middleware.GetUserId(ctx)
	currentUserRole := middleware.GetUserOrgRole(ctx)

	targetUserId, err := strconv.Atoi(ctx.Param("userId"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	var request models.UpdateMemberRoleRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	targetRole, err := repositories.OrganizationRepository.GetUserRole(tx, organizationId, targetUserId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("Failed to get user role: %w", err))
		return
	}
	if targetRole == "" {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "User is not a member of this organization"})
		return
	}

	if targetRole == "owner" {
		ctx.JSON(http.StatusForbidden, gin.H{"error": "Cannot change the owner's role"})
		return
	}

	if request.Role == "admin" && currentUserRole != "owner" {
		ctx.JSON(http.StatusForbidden, gin.H{"error": "Only owners can promote members to admin"})
		return
	}

	if targetUserId == currentUserId {
		ctx.JSON(http.StatusForbidden, gin.H{"error": "Cannot change your own role"})
		return
	}

	err = repositories.OrganizationRepository.UpdateUserRole(tx, organizationId, targetUserId, request.Role)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("Failed to update role: %w", err))
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "Role updated"})
}

func (c *memberController) RemoveMember(ctx *gin.Context) {
	tx := middleware.GetTx(ctx)
	organizationId := middleware.GetOrganizationId(ctx)
	currentUserId := middleware.GetUserId(ctx)

	targetUserId, err := strconv.Atoi(ctx.Param("userId"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	if targetUserId == currentUserId {
		ctx.JSON(http.StatusForbidden, gin.H{"error": "Cannot remove yourself from the organization"})
		return
	}

	targetRole, err := repositories.OrganizationRepository.GetUserRole(tx, organizationId, targetUserId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("Failed to get user role: %w", err))
		return
	}
	if targetRole == "" {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "User is not a member of this organization"})
		return
	}

	if targetRole == "owner" {
		ctx.JSON(http.StatusForbidden, gin.H{"error": "Cannot remove the owner from the organization"})
		return
	}

	err = repositories.OrganizationRepository.RemoveUser(tx, organizationId, targetUserId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("Failed to remove member: %w", err))
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "Member removed"})
}

var MemberController = memberController{}
