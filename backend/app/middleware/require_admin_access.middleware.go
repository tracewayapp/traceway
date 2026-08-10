package middleware

import (
	"database/sql"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	traceway "go.tracewayapp.com"
)

const OrganizationIdContextKey = "organizationId"
const UserOrgRoleContextKey = "userOrgRole"

var RequireAdminAccess gin.HandlerFunc

func InitRequireAdminAccess() {
	RequireAdminAccess = requireOrgRole(func(role string) bool {
		return role == "owner" || role == "admin"
	}, "Admin or owner access required")
}

// requireOrgRole builds middleware that resolves the caller's role in the
// :organizationId organization and rejects with 403 when permitted returns
// false. Sets OrganizationIdContextKey and UserOrgRoleContextKey.
func requireOrgRole(permitted func(role string) bool, denialMessage string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userId := GetUserId(c)
		if userId == 0 {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		organizationId, err := strconv.Atoi(c.Param("organizationId"))
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "Invalid organization ID"})
			return
		}

		role, err := db.ExecuteTransaction(func(tx *sql.Tx) (string, error) {
			return transactional.OrganizationRepository.GetUserRole(tx, organizationId, userId)
		})

		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, traceway.NewStackTraceErrorf("Failed to check permissions: %w", err))
			return
		}

		if !permitted(role) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": denialMessage})
			return
		}

		c.Set(OrganizationIdContextKey, organizationId)
		c.Set(UserOrgRoleContextKey, role)
		c.Next()
	}
}

func GetOrganizationId(c *gin.Context) int {
	if id, exists := c.Get(OrganizationIdContextKey); exists {
		return id.(int)
	}
	return 0
}

func GetUserOrgRole(c *gin.Context) string {
	if role, exists := c.Get(UserOrgRoleContextKey); exists {
		return role.(string)
	}
	return ""
}
