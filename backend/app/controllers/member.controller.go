package controllers

import (
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/middleware"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/oncall"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	traceway "go.tracewayapp.com"
)

type memberController struct{}

func (c *memberController) UpdateRole(ctx *gin.Context) {
	tx := db.GetTx(ctx)
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

	targetRole, err := transactional.OrganizationRepository.GetUserRole(tx, organizationId, targetUserId)
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

	err = transactional.OrganizationRepository.UpdateUserRole(tx, organizationId, targetUserId, request.Role)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("Failed to update role: %w", err))
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "Role updated"})
}

func (c *memberController) RemoveMember(ctx *gin.Context) {
	tx := db.GetTx(ctx)
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

	targetRole, err := transactional.OrganizationRepository.GetUserRole(tx, organizationId, targetUserId)
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

	err = transactional.OrganizationRepository.RemoveUser(tx, organizationId, targetUserId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("Failed to remove member: %w", err))
		return
	}

	err = transactional.ProjectUserRoleRepository.DeleteByOrganizationAndUser(tx, organizationId, targetUserId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("Failed to remove member project roles: %w", err))
		return
	}

	err = transactional.TeamRepository.RemoveUserFromOrgTeams(tx, organizationId, targetUserId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("Failed to remove member from teams: %w", err))
		return
	}

	err = oncall.RemoveUserFromOrgSchedules(tx, organizationId, targetUserId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("Failed to remove member from on-call schedules: %w", err))
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "Member removed"})
}

func (c *memberController) GetProjectRoles(ctx *gin.Context) {
	tx := db.GetTx(ctx)
	organizationId := middleware.GetOrganizationId(ctx)

	targetUserId, err := strconv.Atoi(ctx.Param("userId"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	orgRole, err := transactional.OrganizationRepository.GetUserRole(tx, organizationId, targetUserId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("Failed to get user role: %w", err))
		return
	}
	if orgRole == "" {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "User is not a member of this organization"})
		return
	}

	projectRoles, err := transactional.ProjectUserRoleRepository.FindByOrganizationAndUser(tx, organizationId, targetUserId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("Failed to load member project roles: %w", err))
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"orgRole":      orgRole,
		"projectRoles": projectRoles,
	})
}

func (c *memberController) UpdateProjectRole(ctx *gin.Context) {
	tx := db.GetTx(ctx)
	organizationId := middleware.GetOrganizationId(ctx)

	targetUserId, err := strconv.Atoi(ctx.Param("userId"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	projectId, err := uuid.Parse(ctx.Param("projectId"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid project ID"})
		return
	}

	var request models.UpdateProjectRoleRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	project, err := transactional.ProjectRepository.FindById(tx, projectId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("Failed to load project: %w", err))
		return
	}
	if project == nil || project.OrganizationId == nil || *project.OrganizationId != organizationId {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Project not found in this organization"})
		return
	}

	targetRole, err := transactional.OrganizationRepository.GetUserRole(tx, organizationId, targetUserId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("Failed to get user role: %w", err))
		return
	}
	if targetRole == "" {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "User is not a member of this organization"})
		return
	}
	if request.Role == "default" {
		err = transactional.ProjectUserRoleRepository.Delete(tx, projectId, targetUserId)
	} else {
		if targetRole == "owner" || targetRole == "admin" {
			ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Owners and admins always have full access to every project"})
			return
		}
		err = transactional.ProjectUserRoleRepository.Upsert(tx, projectId, targetUserId, request.Role)
	}
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("Failed to update project role: %w", err))
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "Project role updated"})
}

var MemberController = memberController{}
